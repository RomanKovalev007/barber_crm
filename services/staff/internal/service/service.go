package service

import (
	"context"
	"errors"
	"log/slog"
	"regexp"
	"time"

	"golang.org/x/crypto/bcrypt"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	staffpb "github.com/RomanKovalev007/barber_crm/api/proto/staff/v1"
	"github.com/RomanKovalev007/barber_crm/pkg/auth"
	"github.com/RomanKovalev007/barber_crm/services/staff/internal/apperr"
	"github.com/RomanKovalev007/barber_crm/services/staff/internal/kafka"
	"github.com/RomanKovalev007/barber_crm/services/staff/internal/model"
	"github.com/RomanKovalev007/barber_crm/services/staff/internal/repository"
)

var weekPattern = regexp.MustCompile(`^\d{4}-W(0[1-9]|[1-4]\d|5[0-3])$`)
var datePattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
var timePattern = regexp.MustCompile(`^\d{2}:\d{2}$`)

type redisStore interface {
	Set(ctx context.Context, key string, value any, ttl time.Duration) error
	Get(ctx context.Context, key string) (string, error)
	Del(ctx context.Context, key string) error
}

type staffRepo interface {
	GetBarber(ctx context.Context, id string) (*model.Barber, error)
	GetBarberByLogin(ctx context.Context, login string) (*model.Barber, error)
	ListBarbers(ctx context.Context) ([]model.Barber, error)

	UpsertSchedule(ctx context.Context, barberID string, day *model.ScheduleDay) (*model.ScheduleDay, error)
	UpsertWeekSchedule(ctx context.Context, barberID string, days []*model.ScheduleDay) ([]*model.ScheduleDay, error)
	DeleteSchedule(ctx context.Context, barberID, date string) (string, error)
	GetSchedule(ctx context.Context, barberID string, week string) ([]model.ScheduleDay, error)

	CreateService(ctx context.Context, s *model.Service) error
	UpdateService(ctx context.Context, s *model.Service) error
	DeleteService(ctx context.Context, id string, barberID string) error
	ListServices(ctx context.Context, barberID string, includeInactive bool) ([]model.Service, error)
}

type eventProducer interface {
	Publish(ctx context.Context, topic, key string, msg proto.Message) error
}

type Service struct {
	repo      staffRepo
	redis     redisStore
	producer  eventProducer
	ttl       int
	jwtSecret string
	logger    *slog.Logger
}

func New(repo staffRepo, redis redisStore, producer eventProducer, ttl int, jwtSecret string, logger *slog.Logger) *Service {
	return &Service{
		repo:      repo,
		redis:     redis,
		producer:  producer,
		ttl:       ttl,
		jwtSecret: jwtSecret,
		logger:    logger,
	}
}

// barbers

func (s *Service) Login(ctx context.Context, login, password string) (*model.Barber, string, string, error) {
	if len(password) > 72 {
		return nil, "", "", apperr.InvalidArgument("password too long")
	}

	barber, err := s.repo.GetBarberByLogin(ctx, login)
	if err != nil {
		s.logger.Warn("login failed: barber not found", "login", login)
		return nil, "", "", apperr.Unauthenticated("invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(barber.PasswordHash), []byte(password)); err != nil {
		s.logger.Warn("login failed: wrong password", "login", login)
		return nil, "", "", apperr.Unauthenticated("invalid credentials")
	}

	accessToken, err := auth.GenerateAccessToken(barber.ID, s.jwtSecret)
	if err != nil {
		s.logger.Error("failed to generate access token", "barber_id", barber.ID, "error", err)
		return nil, "", "", apperr.Internal("failed to generate access token")
	}

	refreshToken, err := auth.GenerateRefreshToken(barber.ID, s.jwtSecret)
	if err != nil {
		s.logger.Error("failed to generate refresh token", "barber_id", barber.ID, "error", err)
		return nil, "", "", apperr.Internal("failed to generate refresh token")
	}

	if err := s.redis.Set(ctx, "session:"+barber.ID, refreshToken, time.Duration(s.ttl)*time.Minute); err != nil {
		s.logger.Error("failed to save session", "barber_id", barber.ID, "error", err)
		return nil, "", "", apperr.Internal("failed to save session")
	}

	s.logger.Info("barber logged in", "barber_id", barber.ID)
	return barber, accessToken, refreshToken, nil
}

func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	claims, err := auth.ValidateToken(refreshToken, s.jwtSecret)
	if err != nil {
		s.logger.Warn("logout failed: invalid refresh token")
		return apperr.Unauthenticated("invalid refresh token")
	}
	if err := s.redis.Del(ctx, "session:"+claims.BarberID); err != nil {
		s.logger.Error("failed to delete session", "barber_id", claims.BarberID, "error", err)
		return apperr.Internal("failed to delete session")
	}
	s.logger.Info("barber logged out", "barber_id", claims.BarberID)
	return nil
}

func (s *Service) RefreshToken(ctx context.Context, refreshTokenStr string) (string, string, error) {
	claims, err := auth.ValidateToken(refreshTokenStr, s.jwtSecret)
	if err != nil {
		s.logger.Warn("refresh token failed: invalid token")
		return "", "", apperr.Unauthenticated("invalid refresh token")
	}

	stored, err := s.redis.Get(ctx, "session:"+claims.BarberID)
	if err != nil || stored != refreshTokenStr {
		s.logger.Warn("refresh token failed: session mismatch or not found", "barber_id", claims.BarberID)
		return "", "", apperr.Unauthenticated("invalid refresh token")
	}

	accessToken, err := auth.GenerateAccessToken(claims.BarberID, s.jwtSecret)
	if err != nil {
		s.logger.Error("failed to generate access token", "barber_id", claims.BarberID, "error", err)
		return "", "", apperr.Internal("failed to generate access token")
	}

	newRefresh, err := auth.GenerateRefreshToken(claims.BarberID, s.jwtSecret)
	if err != nil {
		s.logger.Error("failed to generate refresh token", "barber_id", claims.BarberID, "error", err)
		return "", "", apperr.Internal("failed to generate refresh token")
	}

	if err := s.redis.Set(ctx, "session:"+claims.BarberID, newRefresh, time.Duration(s.ttl)*time.Minute); err != nil {
		s.logger.Error("failed to save session", "barber_id", claims.BarberID, "error", err)
		return "", "", apperr.Internal("failed to save session")
	}

	s.logger.Info("token refreshed", "barber_id", claims.BarberID)
	return accessToken, newRefresh, nil
}

func (s *Service) GetBarber(ctx context.Context, id string) (*model.Barber, error) {
	if id == "" {
		return nil, apperr.InvalidArgument("barber_id is empty")
	}
	barber, err := s.repo.GetBarber(ctx, id)
	if err != nil {
		return nil, apperr.NotFound("barber not found")
	}
	return barber, nil
}

func (s *Service) ListBarbers(ctx context.Context) ([]model.Barber, error) {
	barbers, err := s.repo.ListBarbers(ctx)
	if err != nil {
		return nil, apperr.Internal("failed to list barbers")
	}
	return barbers, nil
}

// services

func (s *Service) CreateService(ctx context.Context, svc *model.Service) error {
	if svc.BarberID == "" {
		return apperr.InvalidArgument("barber_id is empty")
	}
	if len(svc.Name) < 2 {
		return apperr.InvalidArgument("name must be at least 2 characters")
	}
	if svc.Price < 0 {
		return apperr.InvalidArgument("price can not be negative")
	}
	if svc.DurationMinutes <= 0 || svc.DurationMinutes%15 != 0 {
		return apperr.InvalidArgument("duration_minutes must be a positive multiple of 15")
	}
	if err := s.repo.CreateService(ctx, svc); err != nil {
		s.logger.Error("failed to create service", "barber_id", svc.BarberID, "error", err)
		return apperr.Internal("failed to create service")
	}
	s.logger.Info("service created", "service_id", svc.ID, "barber_id", svc.BarberID)
	return nil
}

func (s *Service) UpdateService(ctx context.Context, svc *model.Service) error {
	if svc.ID == "" {
		return apperr.InvalidArgument("service_id is empty")
	}
	if svc.BarberID == "" {
		return apperr.InvalidArgument("barber_id is empty")
	}
	if len(svc.Name) < 2 {
		return apperr.InvalidArgument("name must be at least 2 characters")
	}
	if svc.Price < 0 {
		return apperr.InvalidArgument("price can not be negative")
	}
	if svc.DurationMinutes <= 0 || svc.DurationMinutes%15 != 0 {
		return apperr.InvalidArgument("duration_minutes must be a positive multiple of 15")
	}
	if err := s.repo.UpdateService(ctx, svc); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return apperr.NotFound("service not found")
		}
		s.logger.Error("failed to update service", "service_id", svc.ID, "error", err)
		return apperr.Internal("failed to update service")
	}
	s.logger.Info("service updated", "service_id", svc.ID, "barber_id", svc.BarberID)
	return nil
}

func (s *Service) DeleteService(ctx context.Context, id, barberID string) error {
	if id == "" {
		return apperr.InvalidArgument("service_id is empty")
	}
	if barberID == "" {
		return apperr.InvalidArgument("barber_id is empty")
	}
	if err := s.repo.DeleteService(ctx, id, barberID); err != nil {
		s.logger.Error("failed to delete service", "service_id", id, "error", err)
		return apperr.NotFound("service not found")
	}
	s.logger.Info("service deleted", "service_id", id, "barber_id", barberID)
	return nil
}

func (s *Service) ListServices(ctx context.Context, barberID string, includeInactive bool) ([]model.Service, error) {
	if barberID == "" {
		return nil, apperr.InvalidArgument("barber_id is empty")
	}
	services, err := s.repo.ListServices(ctx, barberID, includeInactive)
	if err != nil {
		return nil, apperr.Internal("failed to list services")
	}
	return services, nil
}

// schedule

func (s *Service) GetSchedule(ctx context.Context, barberID, week string) ([]model.ScheduleDay, error) {
	if barberID == "" {
		return nil, apperr.InvalidArgument("barber_id is empty")
	}
	if !weekPattern.MatchString(week) {
		return nil, apperr.InvalidArgument("week must be in format YYYY-Www (e.g. 2026-W10)")
	}
	days, err := s.repo.GetSchedule(ctx, barberID, week)
	if err != nil {
		return nil, apperr.Internal("failed to get schedule")
	}
	return days, nil
}

func (s *Service) UpsertSchedule(ctx context.Context, barberID string, day *model.ScheduleDay) (*model.ScheduleDay, error) {
	if barberID == "" {
		return nil, apperr.InvalidArgument("barber_id is empty")
	}
	if !datePattern.MatchString(day.Date) {
		return nil, apperr.InvalidArgument("date must be in format YYYY-MM-DD")
	}
	if !timePattern.MatchString(day.StartTime) {
		return nil, apperr.InvalidArgument("start_time must be in format HH:MM")
	}
	if !timePattern.MatchString(day.EndTime) {
		return nil, apperr.InvalidArgument("end_time must be in format HH:MM")
	}
	if day.StartTime >= day.EndTime {
		return nil, apperr.InvalidArgument("start_time must be before end_time")
	}
	if day.PartOfDay != model.PartOfDayAM && day.PartOfDay != model.PartOfDayPM {
		return nil, apperr.InvalidArgument("part_of_day must be 'am' or 'pm'")
	}
	result, err := s.repo.UpsertSchedule(ctx, barberID, day)
	if err != nil {
		s.logger.Error("failed to upsert schedule", "barber_id", barberID, "date", day.Date, "error", err)
		return nil, apperr.Internal("failed to upsert schedule")
	}
	s.logger.Info("schedule upserted", "barber_id", barberID, "date", day.Date)
	event := &staffpb.ScheduleEvent{
		ScheduleId: result.ID,
		BarberId:   barberID,
		Date:       result.Date,
		StartTime:  result.StartTime,
		EndTime:    result.EndTime,
		PartOfDay:  partOfDayToProto(result.PartOfDay),
		EventType:  staffpb.ScheduleEventType_SCHEDULE_EVENT_ADDED,
		OccurredAt: timestamppb.Now(),
	}
	if err := s.producer.Publish(ctx, kafka.TopicScheduleEvents, barberID, event); err != nil {
		s.logger.Warn("failed to publish schedule.added event", "error", err)
	}
	return result, nil
}

func (s *Service) UpsertWeekSchedule(ctx context.Context, barberID string, days []*model.ScheduleDay) ([]*model.ScheduleDay, error) {
	if barberID == "" {
		return nil, apperr.InvalidArgument("barber_id is empty")
	}
	if len(days) == 0 || len(days) > 7 {
		return nil, apperr.InvalidArgument("days must contain 1 to 7 entries")
	}
	for _, day := range days {
		if !datePattern.MatchString(day.Date) {
			return nil, apperr.InvalidArgument("date must be in format YYYY-MM-DD")
		}
		if !timePattern.MatchString(day.StartTime) {
			return nil, apperr.InvalidArgument("start_time must be in format HH:MM")
		}
		if !timePattern.MatchString(day.EndTime) {
			return nil, apperr.InvalidArgument("end_time must be in format HH:MM")
		}
		if day.StartTime >= day.EndTime {
			return nil, apperr.InvalidArgument("start_time must be before end_time")
		}
		if day.PartOfDay != model.PartOfDayAM && day.PartOfDay != model.PartOfDayPM {
			return nil, apperr.InvalidArgument("part_of_day must be 'am' or 'pm'")
		}
	}

	result, err := s.repo.UpsertWeekSchedule(ctx, barberID, days)
	if err != nil {
		s.logger.Error("failed to upsert week schedule", "barber_id", barberID, "error", err)
		return nil, apperr.Internal("failed to upsert week schedule")
	}
	s.logger.Info("week schedule upserted", "barber_id", barberID, "days", len(result))

	for _, d := range result {
		event := &staffpb.ScheduleEvent{
			ScheduleId: d.ID,
			BarberId:   barberID,
			Date:       d.Date,
			StartTime:  d.StartTime,
			EndTime:    d.EndTime,
			PartOfDay:  partOfDayToProto(d.PartOfDay),
			EventType:  staffpb.ScheduleEventType_SCHEDULE_EVENT_ADDED,
			OccurredAt: timestamppb.Now(),
		}
		if err := s.producer.Publish(ctx, kafka.TopicScheduleEvents, barberID, event); err != nil {
			s.logger.Warn("failed to publish schedule.added event", "date", d.Date, "error", err)
		}
	}
	return result, nil
}

func partOfDayToProto(p model.PartOfDay) staffpb.PartOfDay {
	switch p {
	case model.PartOfDayAM:
		return staffpb.PartOfDay_PART_OF_DAY_AM
	case model.PartOfDayPM:
		return staffpb.PartOfDay_PART_OF_DAY_PM
	default:
		return staffpb.PartOfDay_PART_OF_DAY_UNSPECIFIED
	}
}

func (s *Service) DeleteSchedule(ctx context.Context, barberID, date string) error {
	if barberID == "" {
		return apperr.InvalidArgument("barber_id is empty")
	}
	if !datePattern.MatchString(date) {
		return apperr.InvalidArgument("date must be in format YYYY-MM-DD")
	}
	scheduleID, err := s.repo.DeleteSchedule(ctx, barberID, date)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return apperr.NotFound("schedule day not found")
		}
		s.logger.Error("failed to delete schedule", "barber_id", barberID, "date", date, "error", err)
		return apperr.Internal("failed to delete schedule")
	}
	s.logger.Info("schedule deleted", "barber_id", barberID, "date", date)
	event := &staffpb.ScheduleEvent{
		ScheduleId: scheduleID,
		BarberId:   barberID,
		Date:       date,
		EventType:  staffpb.ScheduleEventType_SCHEDULE_EVENT_DELETED,
		OccurredAt: timestamppb.Now(),
	}
	if err := s.producer.Publish(ctx, kafka.TopicScheduleEvents, barberID, event); err != nil {
		s.logger.Warn("failed to publish schedule.deleted event", "error", err)
	}
	return nil
}
