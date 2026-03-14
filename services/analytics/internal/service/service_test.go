package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	pb "github.com/RomanKovalev007/barber_crm/api/proto/analytics/v1"
	"github.com/RomanKovalev007/barber_crm/services/analytics/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// ─── mock ────────────────────────────────────────────────────────────────────

type MockRepo struct {
	mock.Mock
}

func (m *MockRepo) GetBookingStats(ctx context.Context, barberID, from, to string) (*model.BookingStats, error) {
	args := m.Called(ctx, barberID, from, to)
	return args.Get(0).(*model.BookingStats), args.Error(1)
}

func (m *MockRepo) GetScheduleMinutes(ctx context.Context, barberID, from, to string) (float64, error) {
	args := m.Called(ctx, barberID, from, to)
	return args.Get(0).(float64), args.Error(1)
}

func (m *MockRepo) GetTopServices(ctx context.Context, barberID, from, to string) ([]model.TopService, error) {
	args := m.Called(ctx, barberID, from, to)
	return args.Get(0).([]model.TopService), args.Error(1)
}

func (m *MockRepo) GetDailyBreakdown(ctx context.Context, barberID, from, to string) ([]model.DayStat, error) {
	args := m.Called(ctx, barberID, from, to)
	return args.Get(0).([]model.DayStat), args.Error(1)
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func newTestService(r analyticsRepo) *Service {
	return New(r, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func defaultStats() *model.BookingStats {
	return &model.BookingStats{
		ClientsServed:     5,
		TotalRevenue:      10000,
		BookingsTotal:     6,
		BookingsCompleted: 5,
		BookingsCancelled: 1,
		BookingsNoShow:    0,
		BookedMinutes:     300,
	}
}

func customPeriod(from, to string) *pb.Period {
	return &pb.Period{
		Kind: &pb.Period_Custom{
			Custom: &pb.DateRange{DateFrom: from, DateTo: to},
		},
	}
}

// ─── GetBarberStats ───────────────────────────────────────────────────────────

func TestGetBarberStats_EmptyBarberID(t *testing.T) {
	svc := newTestService(new(MockRepo))

	_, err := svc.GetBarberStats(context.Background(), &pb.GetBarberStatsRequest{BarberId: ""})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "barber_id is empty")
}

func TestGetBarberStats_Success(t *testing.T) {
	ctx := context.Background()

	r := new(MockRepo)
	r.On("GetBookingStats", ctx, "b1", "2026-01-01", "2026-01-31").Return(defaultStats(), nil)
	r.On("GetScheduleMinutes", ctx, "b1", "2026-01-01", "2026-01-31").Return(600.0, nil)
	r.On("GetTopServices", ctx, "b1", "2026-01-01", "2026-01-31").Return([]model.TopService{
		{ServiceID: "s1", ServiceName: "Haircut", Count: 3, Revenue: 6000},
	}, nil)
	r.On("GetDailyBreakdown", ctx, "b1", "2026-01-01", "2026-01-31").Return([]model.DayStat{
		{Date: "2026-01-15", Clients: 5, Revenue: 10000, HoursWorked: 5},
	}, nil)

	svc := newTestService(r)

	resp, err := svc.GetBarberStats(ctx, &pb.GetBarberStatsRequest{
		BarberId: "b1",
		Period:   customPeriod("2026-01-01", "2026-01-31"),
	})

	require.NoError(t, err)
	assert.Equal(t, "b1", resp.BarberId)
	assert.Equal(t, "2026-01-01", resp.DateFrom)
	assert.Equal(t, "2026-01-31", resp.DateTo)
	assert.Equal(t, int64(5), resp.ClientsServed)
	assert.Equal(t, int64(10000), resp.TotalRevenue)
	assert.InDelta(t, 10.0, resp.HoursWorked, 0.001)     // 600 / 60
	assert.InDelta(t, 2000.0, resp.AverageCheck, 0.001)  // 10000 / 5
	assert.InDelta(t, 0.5, resp.OccupancyRate, 0.001)    // 300 / 600
	assert.Equal(t, int64(6), resp.BookingsTotal)
	assert.Equal(t, int64(5), resp.BookingsCompleted)
	assert.Equal(t, int64(1), resp.BookingsCancelled)
	assert.Equal(t, int64(0), resp.BookingsNoShow)
	require.Len(t, resp.TopServices, 1)
	assert.Equal(t, "Haircut", resp.TopServices[0].ServiceName)
	require.Len(t, resp.DailyBreakdown, 1)
	assert.Equal(t, "2026-01-15", resp.DailyBreakdown[0].Date)
	r.AssertExpectations(t)
}

func TestGetBarberStats_ZeroClients_AverageCheckIsZero(t *testing.T) {
	ctx := context.Background()

	stats := &model.BookingStats{ClientsServed: 0, TotalRevenue: 0}

	r := new(MockRepo)
	r.On("GetBookingStats", ctx, "b1", "2026-01-01", "2026-01-31").Return(stats, nil)
	r.On("GetScheduleMinutes", ctx, "b1", "2026-01-01", "2026-01-31").Return(0.0, nil)
	r.On("GetTopServices", ctx, "b1", "2026-01-01", "2026-01-31").Return([]model.TopService{}, nil)
	r.On("GetDailyBreakdown", ctx, "b1", "2026-01-01", "2026-01-31").Return([]model.DayStat{}, nil)

	svc := newTestService(r)

	resp, err := svc.GetBarberStats(ctx, &pb.GetBarberStatsRequest{
		BarberId: "b1",
		Period:   customPeriod("2026-01-01", "2026-01-31"),
	})

	require.NoError(t, err)
	assert.Equal(t, 0.0, resp.AverageCheck)
	assert.Equal(t, 0.0, resp.OccupancyRate)
}

func TestGetBarberStats_GetBookingStatsError(t *testing.T) {
	ctx := context.Background()

	r := new(MockRepo)
	r.On("GetBookingStats", ctx, "b1", mock.Anything, mock.Anything).
		Return((*model.BookingStats)(nil), errors.New("db error"))

	svc := newTestService(r)

	_, err := svc.GetBarberStats(ctx, &pb.GetBarberStatsRequest{
		BarberId: "b1",
		Period:   customPeriod("2026-01-01", "2026-01-31"),
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "booking stats")
}

func TestGetBarberStats_GetScheduleMinutesError(t *testing.T) {
	ctx := context.Background()

	r := new(MockRepo)
	r.On("GetBookingStats", ctx, "b1", mock.Anything, mock.Anything).Return(defaultStats(), nil)
	r.On("GetScheduleMinutes", ctx, "b1", mock.Anything, mock.Anything).
		Return(0.0, errors.New("db error"))

	svc := newTestService(r)

	_, err := svc.GetBarberStats(ctx, &pb.GetBarberStatsRequest{
		BarberId: "b1",
		Period:   customPeriod("2026-01-01", "2026-01-31"),
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "schedule minutes")
}

func TestGetBarberStats_GetTopServicesError(t *testing.T) {
	ctx := context.Background()

	r := new(MockRepo)
	r.On("GetBookingStats", ctx, "b1", mock.Anything, mock.Anything).Return(defaultStats(), nil)
	r.On("GetScheduleMinutes", ctx, "b1", mock.Anything, mock.Anything).Return(600.0, nil)
	r.On("GetTopServices", ctx, "b1", mock.Anything, mock.Anything).
		Return([]model.TopService(nil), errors.New("db error"))

	svc := newTestService(r)

	_, err := svc.GetBarberStats(ctx, &pb.GetBarberStatsRequest{
		BarberId: "b1",
		Period:   customPeriod("2026-01-01", "2026-01-31"),
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "top services")
}

func TestGetBarberStats_GetDailyBreakdownError(t *testing.T) {
	ctx := context.Background()

	r := new(MockRepo)
	r.On("GetBookingStats", ctx, "b1", mock.Anything, mock.Anything).Return(defaultStats(), nil)
	r.On("GetScheduleMinutes", ctx, "b1", mock.Anything, mock.Anything).Return(600.0, nil)
	r.On("GetTopServices", ctx, "b1", mock.Anything, mock.Anything).Return([]model.TopService{}, nil)
	r.On("GetDailyBreakdown", ctx, "b1", mock.Anything, mock.Anything).
		Return([]model.DayStat(nil), errors.New("db error"))

	svc := newTestService(r)

	_, err := svc.GetBarberStats(ctx, &pb.GetBarberStatsRequest{
		BarberId: "b1",
		Period:   customPeriod("2026-01-01", "2026-01-31"),
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "daily breakdown")
}

// ─── resolvePeriod ────────────────────────────────────────────────────────────

func TestResolvePeriod_Nil(t *testing.T) {
	from, to := resolvePeriod(nil)
	assert.Empty(t, from)
	assert.Empty(t, to)
}

func TestResolvePeriod_All(t *testing.T) {
	from, to := resolvePeriod(&pb.Period{
		Kind: &pb.Period_Preset{Preset: pb.PredefinedPeriod_PERIOD_ALL},
	})
	assert.Empty(t, from)
	assert.Empty(t, to)
}

func TestResolvePeriod_Custom(t *testing.T) {
	from, to := resolvePeriod(customPeriod("2025-06-01", "2025-06-30"))
	assert.Equal(t, "2025-06-01", from)
	assert.Equal(t, "2025-06-30", to)
}

func TestResolvePeriod_Day(t *testing.T) {
	from, to := resolvePeriod(&pb.Period{
		Kind: &pb.Period_Preset{Preset: pb.PredefinedPeriod_PERIOD_DAY},
	})
	assert.NotEmpty(t, from)
	assert.Equal(t, from, to) // today == today
}

func TestResolvePeriod_Week(t *testing.T) {
	from, to := resolvePeriod(&pb.Period{
		Kind: &pb.Period_Preset{Preset: pb.PredefinedPeriod_PERIOD_WEEK},
	})
	assert.NotEmpty(t, from)
	assert.NotEmpty(t, to)
	assert.LessOrEqual(t, from, to)
	// Понедельник — Воскресенье: разница 6 дней
	assert.NotEqual(t, from, to)
}

func TestResolvePeriod_Month(t *testing.T) {
	from, to := resolvePeriod(&pb.Period{
		Kind: &pb.Period_Preset{Preset: pb.PredefinedPeriod_PERIOD_MONTH},
	})
	assert.NotEmpty(t, from)
	assert.NotEmpty(t, to)
	assert.LessOrEqual(t, from, to)
	// Первый день месяца заканчивается на "-01"
	assert.Equal(t, "01", from[len(from)-2:])
}

func TestResolvePeriod_Unspecified(t *testing.T) {
	from, to := resolvePeriod(&pb.Period{
		Kind: &pb.Period_Preset{Preset: pb.PredefinedPeriod_PERIOD_UNSPECIFIED},
	})
	assert.Empty(t, from)
	assert.Empty(t, to)
}

// ─── proto converters ────────────────────────────────────────────────────────

func TestToProtoTopServices_Empty(t *testing.T) {
	result := toProtoTopServices(nil)
	assert.NotNil(t, result)
	assert.Empty(t, result)
}

func TestToProtoTopServices(t *testing.T) {
	services := []model.TopService{
		{ServiceID: "s1", ServiceName: "Haircut", Count: 10, Revenue: 20000},
		{ServiceID: "s2", ServiceName: "Beard", Count: 5, Revenue: 7500},
	}

	result := toProtoTopServices(services)

	require.Len(t, result, 2)
	assert.Equal(t, "s1", result[0].ServiceId)
	assert.Equal(t, "Haircut", result[0].ServiceName)
	assert.Equal(t, int64(10), result[0].Count)
	assert.Equal(t, int64(20000), result[0].Revenue)
	assert.Equal(t, "s2", result[1].ServiceId)
}

func TestToProtoDailyBreakdown_Empty(t *testing.T) {
	result := toProtoDailyBreakdown(nil)
	assert.NotNil(t, result)
	assert.Empty(t, result)
}

func TestToProtoDailyBreakdown(t *testing.T) {
	days := []model.DayStat{
		{Date: "2026-01-01", Clients: 3, Revenue: 6000, HoursWorked: 4.5},
		{Date: "2026-01-02", Clients: 2, Revenue: 4000, HoursWorked: 3.0},
	}

	result := toProtoDailyBreakdown(days)

	require.Len(t, result, 2)
	assert.Equal(t, "2026-01-01", result[0].Date)
	assert.Equal(t, int64(3), result[0].Clients)
	assert.Equal(t, int64(6000), result[0].Revenue)
	assert.InDelta(t, 4.5, result[0].HoursWorked, 0.001)
	assert.Equal(t, "2026-01-02", result[1].Date)
}
