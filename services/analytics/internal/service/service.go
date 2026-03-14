package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	pb "github.com/RomanKovalev007/barber_crm/api/proto/analytics/v1"
	"github.com/RomanKovalev007/barber_crm/services/analytics/internal/model"
)

type analyticsRepo interface {
	GetBookingStats(ctx context.Context, barberID, from, to string) (*model.BookingStats, error)
	GetScheduleMinutes(ctx context.Context, barberID, from, to string) (float64, error)
	GetTopServices(ctx context.Context, barberID, from, to string) ([]model.TopService, error)
	GetDailyBreakdown(ctx context.Context, barberID, from, to string) ([]model.DayStat, error)
}

type Service struct {
	repo   analyticsRepo
	logger *slog.Logger
}

func New(repo analyticsRepo, logger *slog.Logger) *Service {
	return &Service{repo: repo, logger: logger}
}

func (s *Service) GetBarberStats(ctx context.Context, req *pb.GetBarberStatsRequest) (*pb.BarberStatsResponse, error) {
	if req.BarberId == "" {
		return nil, fmt.Errorf("barber_id is empty")
	}

	from, to := resolvePeriod(req.Period)

	stats, err := s.repo.GetBookingStats(ctx, req.BarberId, from, to)
	if err != nil {
		return nil, fmt.Errorf("booking stats: %w", err)
	}

	scheduleMinutes, err := s.repo.GetScheduleMinutes(ctx, req.BarberId, from, to)
	if err != nil {
		return nil, fmt.Errorf("schedule minutes: %w", err)
	}

	topServices, err := s.repo.GetTopServices(ctx, req.BarberId, from, to)
	if err != nil {
		return nil, fmt.Errorf("top services: %w", err)
	}

	daily, err := s.repo.GetDailyBreakdown(ctx, req.BarberId, from, to)
	if err != nil {
		return nil, fmt.Errorf("daily breakdown: %w", err)
	}

	hoursWorked := scheduleMinutes / 60.0

	var averageCheck float64
	if stats.ClientsServed > 0 {
		averageCheck = float64(stats.TotalRevenue) / float64(stats.ClientsServed)
	}

	var occupancyRate float64
	if scheduleMinutes > 0 {
		occupancyRate = stats.BookedMinutes / scheduleMinutes
	}

	resp := &pb.BarberStatsResponse{
		BarberId: req.BarberId,
		DateFrom: from,
		DateTo:   to,
		ClientsServed:    stats.ClientsServed,
		TotalRevenue:     stats.TotalRevenue,
		HoursWorked:      hoursWorked,
		AverageCheck:     averageCheck,
		BookingsTotal:    stats.BookingsTotal,
		BookingsCompleted: stats.BookingsCompleted,
		BookingsCancelled: stats.BookingsCancelled,
		BookingsNoShow:   stats.BookingsNoShow,
		OccupancyRate:    occupancyRate,
		TopServices:      toProtoTopServices(topServices),
		DailyBreakdown:   toProtoDailyBreakdown(daily),
	}

	return resp, nil
}

// ─── period resolution ───────────────────────────────────────────────────────

// resolvePeriod конвертирует Period из запроса в даты from/to (YYYY-MM-DD).
// Для PERIOD_ALL возвращает ("", "") — репозиторий игнорирует фильтр по дате.
func resolvePeriod(p *pb.Period) (from, to string) {
	if p == nil {
		return "", ""
	}

	now := time.Now()

	switch k := p.Kind.(type) {
	case *pb.Period_Preset:
		switch k.Preset {
		case pb.PredefinedPeriod_PERIOD_DAY:
			d := now.Format("2006-01-02")
			return d, d

		case pb.PredefinedPeriod_PERIOD_WEEK:
			// ISO: неделя Пн–Вс
			weekday := int(now.Weekday())
			if weekday == 0 {
				weekday = 7 // Воскресенье → 7
			}
			monday := now.AddDate(0, 0, -(weekday - 1))
			sunday := monday.AddDate(0, 0, 6)
			return monday.Format("2006-01-02"), sunday.Format("2006-01-02")

		case pb.PredefinedPeriod_PERIOD_MONTH:
			first := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
			last := first.AddDate(0, 1, -1)
			return first.Format("2006-01-02"), last.Format("2006-01-02")

		case pb.PredefinedPeriod_PERIOD_ALL:
			return "", ""
		}

	case *pb.Period_Custom:
		return k.Custom.DateFrom, k.Custom.DateTo
	}

	return "", ""
}

// ─── proto converters ────────────────────────────────────────────────────────

func toProtoTopServices(services []model.TopService) []*pb.TopService {
	result := make([]*pb.TopService, 0, len(services))
	for _, s := range services {
		result = append(result, &pb.TopService{
			ServiceId:   s.ServiceID,
			ServiceName: s.ServiceName,
			Count:       s.Count,
			Revenue:     s.Revenue,
		})
	}
	return result
}

func toProtoDailyBreakdown(days []model.DayStat) []*pb.DayStat {
	result := make([]*pb.DayStat, 0, len(days))
	for _, d := range days {
		result = append(result, &pb.DayStat{
			Date:        d.Date,
			Clients:     d.Clients,
			Revenue:     d.Revenue,
			HoursWorked: d.HoursWorked,
		})
	}
	return result
}
