package grpc

import (
	"context"
	"testing"

	pb "github.com/RomanKovalev007/barber_crm/api/proto/staff/v1"
	"github.com/RomanKovalev007/barber_crm/services/staff/internal/apperr"
	"github.com/RomanKovalev007/barber_crm/services/staff/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ─── Mock ─────────────────────────────────────────────────────────────────────

type mockStaffSvc struct{ mock.Mock }

func (m *mockStaffSvc) Login(ctx context.Context, login, password string) (*model.Barber, string, string, error) {
	args := m.Called(ctx, login, password)
	return args.Get(0).(*model.Barber), args.String(1), args.String(2), args.Error(3)
}
func (m *mockStaffSvc) Logout(ctx context.Context, refreshToken string) error {
	return m.Called(ctx, refreshToken).Error(0)
}
func (m *mockStaffSvc) RefreshToken(ctx context.Context, refreshTokenStr string) (string, string, error) {
	args := m.Called(ctx, refreshTokenStr)
	return args.String(0), args.String(1), args.Error(2)
}
func (m *mockStaffSvc) GetBarber(ctx context.Context, id string) (*model.Barber, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(*model.Barber), args.Error(1)
}
func (m *mockStaffSvc) ListBarbers(ctx context.Context) ([]model.Barber, error) {
	args := m.Called(ctx)
	return args.Get(0).([]model.Barber), args.Error(1)
}
func (m *mockStaffSvc) UpsertSchedule(ctx context.Context, barberID string, day *model.ScheduleDay) (*model.ScheduleDay, error) {
	args := m.Called(ctx, barberID, day)
	return args.Get(0).(*model.ScheduleDay), args.Error(1)
}
func (m *mockStaffSvc) UpsertWeekSchedule(ctx context.Context, barberID string, days []*model.ScheduleDay) ([]*model.ScheduleDay, error) {
	args := m.Called(ctx, barberID, days)
	return args.Get(0).([]*model.ScheduleDay), args.Error(1)
}
func (m *mockStaffSvc) DeleteSchedule(ctx context.Context, barberID, date string) error {
	return m.Called(ctx, barberID, date).Error(0)
}
func (m *mockStaffSvc) GetSchedule(ctx context.Context, barberID, week string) ([]model.ScheduleDay, error) {
	args := m.Called(ctx, barberID, week)
	return args.Get(0).([]model.ScheduleDay), args.Error(1)
}
func (m *mockStaffSvc) ListServices(ctx context.Context, barberID string, includeInactive bool) ([]model.Service, error) {
	args := m.Called(ctx, barberID, includeInactive)
	return args.Get(0).([]model.Service), args.Error(1)
}
func (m *mockStaffSvc) CreateService(ctx context.Context, svc *model.Service) error {
	return m.Called(ctx, svc).Error(0)
}
func (m *mockStaffSvc) DeleteService(ctx context.Context, id, barberID string) error {
	return m.Called(ctx, id, barberID).Error(0)
}
func (m *mockStaffSvc) UpdateService(ctx context.Context, svc *model.Service) error {
	return m.Called(ctx, svc).Error(0)
}

func newStaffServer() (*Server, *mockStaffSvc) {
	svc := &mockStaffSvc{}
	return NewServer(svc), svc
}

func grpcCode(err error) codes.Code {
	st, _ := status.FromError(err)
	return st.Code()
}

func sampleBarber() *model.Barber {
	return &model.Barber{ID: "b-1", Name: "Ivan", Login: "ivan"}
}

func sampleService() *model.Service {
	return &model.Service{ID: "svc-1", BarberID: "b-1", Name: "Haircut", Price: 500, DurationMinutes: 30, IsActive: true}
}

func sampleScheduleDay() *model.ScheduleDay {
	return &model.ScheduleDay{ID: "sched-1", BarberID: "b-1", Date: "2026-03-16", StartTime: "09:00", EndTime: "18:00", PartOfDay: model.PartOfDayAM}
}

// ─── Login ────────────────────────────────────────────────────────────────────

func TestLogin_Success(t *testing.T) {
	srv, svc := newStaffServer()
	svc.On("Login", mock.Anything, "ivan", "pass").Return(sampleBarber(), "access", "refresh", nil)

	resp, err := srv.Login(context.Background(), &pb.LoginRequest{Login: "ivan", Password: "pass"})
	require.NoError(t, err)
	assert.Equal(t, "access", resp.AccessToken)
	assert.Equal(t, "refresh", resp.RefreshToken)
	assert.Equal(t, int32(3600), resp.ExpiresIn)
	assert.Equal(t, "b-1", resp.Barber.BarberId)
}

func TestLogin_Unauthenticated(t *testing.T) {
	srv, svc := newStaffServer()
	svc.On("Login", mock.Anything, "ivan", "wrong").Return(&model.Barber{}, "", "", apperr.Unauthenticated("invalid credentials"))

	_, err := srv.Login(context.Background(), &pb.LoginRequest{Login: "ivan", Password: "wrong"})
	assert.Equal(t, codes.Unauthenticated, grpcCode(err))
}

// ─── Logout ───────────────────────────────────────────────────────────────────

func TestLogout_Success(t *testing.T) {
	srv, svc := newStaffServer()
	svc.On("Logout", mock.Anything, "refresh-token").Return(nil)

	_, err := srv.Logout(context.Background(), &pb.LogoutRequest{RefreshToken: "refresh-token"})
	require.NoError(t, err)
}

func TestLogout_Error(t *testing.T) {
	srv, svc := newStaffServer()
	svc.On("Logout", mock.Anything, "bad-token").Return(apperr.NotFound("session not found"))

	_, err := srv.Logout(context.Background(), &pb.LogoutRequest{RefreshToken: "bad-token"})
	assert.Equal(t, codes.NotFound, grpcCode(err))
}

// ─── RefreshToken ─────────────────────────────────────────────────────────────

func TestRefreshToken_Success(t *testing.T) {
	srv, svc := newStaffServer()
	svc.On("RefreshToken", mock.Anything, "old-refresh").Return("new-access", "new-refresh", nil)

	resp, err := srv.RefreshToken(context.Background(), &pb.RefreshTokenRequest{RefreshToken: "old-refresh"})
	require.NoError(t, err)
	assert.Equal(t, "new-access", resp.AccessToken)
	assert.Equal(t, "new-refresh", resp.RefreshToken)
}

func TestRefreshToken_Invalid(t *testing.T) {
	srv, svc := newStaffServer()
	svc.On("RefreshToken", mock.Anything, "expired").Return("", "", apperr.Unauthenticated("expired"))

	_, err := srv.RefreshToken(context.Background(), &pb.RefreshTokenRequest{RefreshToken: "expired"})
	assert.Equal(t, codes.Unauthenticated, grpcCode(err))
}

// ─── GetBarber ────────────────────────────────────────────────────────────────

func TestGetBarber_Success(t *testing.T) {
	srv, svc := newStaffServer()
	b := sampleBarber()
	b.Services = []model.Service{*sampleService()}
	svc.On("GetBarber", mock.Anything, "b-1").Return(b, nil)

	resp, err := srv.GetBarber(context.Background(), &pb.GetBarberRequest{BarberId: "b-1"})
	require.NoError(t, err)
	assert.Equal(t, "b-1", resp.BarberId)
	assert.Len(t, resp.Services, 1)
}

func TestGetBarber_NotFound(t *testing.T) {
	srv, svc := newStaffServer()
	svc.On("GetBarber", mock.Anything, "x").Return(&model.Barber{}, apperr.NotFound("not found"))

	_, err := srv.GetBarber(context.Background(), &pb.GetBarberRequest{BarberId: "x"})
	assert.Equal(t, codes.NotFound, grpcCode(err))
}

// ─── ListBarbers ──────────────────────────────────────────────────────────────

func TestListBarbers_Success(t *testing.T) {
	srv, svc := newStaffServer()
	svc.On("ListBarbers", mock.Anything).Return([]model.Barber{*sampleBarber()}, nil)

	resp, err := srv.ListBarbers(context.Background(), &pb.ListBarbersRequest{})
	require.NoError(t, err)
	assert.Len(t, resp.Barbers, 1)
}

func TestListBarbers_Empty(t *testing.T) {
	srv, svc := newStaffServer()
	svc.On("ListBarbers", mock.Anything).Return([]model.Barber{}, nil)

	resp, err := srv.ListBarbers(context.Background(), &pb.ListBarbersRequest{})
	require.NoError(t, err)
	assert.Empty(t, resp.Barbers)
}

// ─── GetSchedule ──────────────────────────────────────────────────────────────

func TestGetSchedule_Success(t *testing.T) {
	srv, svc := newStaffServer()
	svc.On("GetSchedule", mock.Anything, "b-1", "2026-W11").Return([]model.ScheduleDay{*sampleScheduleDay()}, nil)

	resp, err := srv.GetSchedule(context.Background(), &pb.GetScheduleRequest{BarberId: "b-1", Week: "2026-W11"})
	require.NoError(t, err)
	assert.Equal(t, "2026-W11", resp.Week)
	assert.Len(t, resp.Days, 1)
}

func TestGetSchedule_Error(t *testing.T) {
	srv, svc := newStaffServer()
	svc.On("GetSchedule", mock.Anything, "b-1", "bad").Return([]model.ScheduleDay{}, apperr.Internal("db error"))

	_, err := srv.GetSchedule(context.Background(), &pb.GetScheduleRequest{BarberId: "b-1", Week: "bad"})
	assert.Equal(t, codes.Internal, grpcCode(err))
}

// ─── UpsertSchedule ───────────────────────────────────────────────────────────

func TestUpsertSchedule_Success(t *testing.T) {
	srv, svc := newStaffServer()
	day := sampleScheduleDay()
	svc.On("UpsertSchedule", mock.Anything, "b-1", mock.Anything).Return(day, nil)

	resp, err := srv.UpsertSchedule(context.Background(), &pb.UpsertScheduleRequest{
		BarberId:  "b-1",
		Date:      "2026-03-16",
		StartTime: "09:00",
		EndTime:   "18:00",
		PartOfDay: pb.PartOfDay_PART_OF_DAY_AM,
	})
	require.NoError(t, err)
	assert.Equal(t, "2026-03-16", resp.Date)
}

func TestUpsertSchedule_PartOfDayPM(t *testing.T) {
	srv, svc := newStaffServer()
	day := sampleScheduleDay()
	day.PartOfDay = model.PartOfDayPM
	svc.On("UpsertSchedule", mock.Anything, "b-1", mock.MatchedBy(func(d *model.ScheduleDay) bool {
		return d.PartOfDay == model.PartOfDayPM
	})).Return(day, nil)

	resp, err := srv.UpsertSchedule(context.Background(), &pb.UpsertScheduleRequest{
		BarberId:  "b-1",
		Date:      "2026-03-16",
		StartTime: "13:00",
		EndTime:   "20:00",
		PartOfDay: pb.PartOfDay_PART_OF_DAY_PM,
	})
	require.NoError(t, err)
	assert.Equal(t, pb.PartOfDay_PART_OF_DAY_PM, resp.PartOfDay)
}

// ─── DeleteSchedule ───────────────────────────────────────────────────────────

func TestDeleteSchedule_Success(t *testing.T) {
	srv, svc := newStaffServer()
	svc.On("DeleteSchedule", mock.Anything, "b-1", "2026-03-16").Return(nil)

	_, err := srv.DeleteSchedule(context.Background(), &pb.DeleteScheduleRequest{BarberId: "b-1", Date: "2026-03-16"})
	require.NoError(t, err)
}

func TestDeleteSchedule_NotFound(t *testing.T) {
	srv, svc := newStaffServer()
	svc.On("DeleteSchedule", mock.Anything, "b-1", "2026-01-01").Return(apperr.NotFound("not found"))

	_, err := srv.DeleteSchedule(context.Background(), &pb.DeleteScheduleRequest{BarberId: "b-1", Date: "2026-01-01"})
	assert.Equal(t, codes.NotFound, grpcCode(err))
}

// ─── ListServices ─────────────────────────────────────────────────────────────

func TestListServices_Success(t *testing.T) {
	srv, svc := newStaffServer()
	svc.On("ListServices", mock.Anything, "b-1", true).Return([]model.Service{*sampleService()}, nil)

	resp, err := srv.ListServices(context.Background(), &pb.ListServicesRequest{BarberId: "b-1", IncludeInactive: true})
	require.NoError(t, err)
	assert.Len(t, resp.Services, 1)
	assert.Equal(t, "svc-1", resp.Services[0].ServiceId)
}

func TestListServices_Empty(t *testing.T) {
	srv, svc := newStaffServer()
	svc.On("ListServices", mock.Anything, "b-1", false).Return([]model.Service{}, nil)

	resp, err := srv.ListServices(context.Background(), &pb.ListServicesRequest{BarberId: "b-1"})
	require.NoError(t, err)
	assert.Empty(t, resp.Services)
}

// ─── CreateService ────────────────────────────────────────────────────────────

func TestCreateService_Success(t *testing.T) {
	srv, svc := newStaffServer()
	svc.On("CreateService", mock.Anything, mock.Anything).Return(nil)

	resp, err := srv.CreateService(context.Background(), &pb.CreateServiceRequest{
		BarberId:        "b-1",
		Name:            "Haircut",
		Price:           500,
		DurationMinutes: 30,
	})
	require.NoError(t, err)
	assert.Equal(t, "Haircut", resp.Name)
	assert.Equal(t, int32(500), resp.Price)
	assert.True(t, resp.IsActive)
}

func TestCreateService_Error(t *testing.T) {
	srv, svc := newStaffServer()
	svc.On("CreateService", mock.Anything, mock.Anything).Return(apperr.Internal("db error"))

	_, err := srv.CreateService(context.Background(), &pb.CreateServiceRequest{
		BarberId: "b-1",
		Name:     "Haircut",
	})
	assert.Equal(t, codes.Internal, grpcCode(err))
}

// ─── UpdateService ────────────────────────────────────────────────────────────

func TestUpdateService_Success(t *testing.T) {
	srv, svc := newStaffServer()
	svc.On("UpdateService", mock.Anything, mock.Anything).Return(nil)

	resp, err := srv.UpdateService(context.Background(), &pb.UpdateServiceRequest{
		ServiceId:       "svc-1",
		BarberId:        "b-1",
		Name:            "Beard trim",
		Price:           300,
		DurationMinutes: 15,
		IsActive:        true,
	})
	require.NoError(t, err)
	assert.Equal(t, "Beard trim", resp.Name)
}

func TestUpdateService_NotFound(t *testing.T) {
	srv, svc := newStaffServer()
	svc.On("UpdateService", mock.Anything, mock.Anything).Return(apperr.NotFound("not found"))

	_, err := srv.UpdateService(context.Background(), &pb.UpdateServiceRequest{ServiceId: "x", BarberId: "b-1"})
	assert.Equal(t, codes.NotFound, grpcCode(err))
}

// ─── DeleteService ────────────────────────────────────────────────────────────

func TestDeleteService_Success(t *testing.T) {
	srv, svc := newStaffServer()
	svc.On("DeleteService", mock.Anything, "svc-1", "b-1").Return(nil)

	_, err := srv.DeleteService(context.Background(), &pb.DeleteServiceRequest{ServiceId: "svc-1", BarberId: "b-1"})
	require.NoError(t, err)
}

func TestDeleteService_NotFound(t *testing.T) {
	srv, svc := newStaffServer()
	svc.On("DeleteService", mock.Anything, "x", "b-1").Return(apperr.NotFound("not found"))

	_, err := srv.DeleteService(context.Background(), &pb.DeleteServiceRequest{ServiceId: "x", BarberId: "b-1"})
	assert.Equal(t, codes.NotFound, grpcCode(err))
}

// ─── toGRPCError ─────────────────────────────────────────────────────────────

func TestToGRPCError_AppErrors(t *testing.T) {
	cases := []struct {
		err      error
		expected codes.Code
	}{
		{apperr.NotFound("x"), codes.NotFound},
		{apperr.InvalidArgument("x"), codes.InvalidArgument},
		{apperr.Unauthenticated("x"), codes.Unauthenticated},
		{apperr.Internal("x"), codes.Internal},
	}
	for _, tc := range cases {
		err := toGRPCError(tc.err)
		assert.Equal(t, tc.expected, grpcCode(err))
	}
}

func TestToGRPCError_UnknownError(t *testing.T) {
	err := toGRPCError(assert.AnError)
	assert.Equal(t, codes.Internal, grpcCode(err))
}
