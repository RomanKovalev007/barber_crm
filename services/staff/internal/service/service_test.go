package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/protobuf/proto"

	"github.com/RomanKovalev007/barber_crm/pkg/auth"
	"github.com/RomanKovalev007/barber_crm/services/staff/internal/apperr"
	"github.com/RomanKovalev007/barber_crm/services/staff/internal/model"
	"github.com/RomanKovalev007/barber_crm/services/staff/internal/repository"
)

// ---------- mocks ----------

type MockRepo struct {
	mock.Mock
}

func (m *MockRepo) GetBarber(ctx context.Context, id string) (*model.Barber, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(*model.Barber), args.Error(1)
}
func (m *MockRepo) GetBarberByLogin(ctx context.Context, login string) (*model.Barber, error) {
	args := m.Called(ctx, login)
	return args.Get(0).(*model.Barber), args.Error(1)
}
func (m *MockRepo) ListBarbers(ctx context.Context) ([]model.Barber, error) {
	args := m.Called(ctx)
	return args.Get(0).([]model.Barber), args.Error(1)
}
func (m *MockRepo) UpsertSchedule(ctx context.Context, barberID string, day *model.ScheduleDay) (*model.ScheduleDay, error) {
	args := m.Called(ctx, barberID, day)
	return args.Get(0).(*model.ScheduleDay), args.Error(1)
}
func (m *MockRepo) UpsertWeekSchedule(ctx context.Context, barberID string, days []*model.ScheduleDay) ([]*model.ScheduleDay, error) {
	args := m.Called(ctx, barberID, days)
	return args.Get(0).([]*model.ScheduleDay), args.Error(1)
}
func (m *MockRepo) DeleteSchedule(ctx context.Context, barberID, date string) (string, error) {
	args := m.Called(ctx, barberID, date)
	return args.String(0), args.Error(1)
}
func (m *MockRepo) GetSchedule(ctx context.Context, barberID, week string) ([]model.ScheduleDay, error) {
	args := m.Called(ctx, barberID, week)
	return args.Get(0).([]model.ScheduleDay), args.Error(1)
}
func (m *MockRepo) CreateService(ctx context.Context, s *model.Service) error {
	return m.Called(ctx, s).Error(0)
}
func (m *MockRepo) UpdateService(ctx context.Context, s *model.Service) error {
	return m.Called(ctx, s).Error(0)
}
func (m *MockRepo) DeleteService(ctx context.Context, id, barberID string) error {
	return m.Called(ctx, id, barberID).Error(0)
}
func (m *MockRepo) ListServices(ctx context.Context, barberID string, includeInactive bool) ([]model.Service, error) {
	args := m.Called(ctx, barberID, includeInactive)
	return args.Get(0).([]model.Service), args.Error(1)
}

type MockSessionStore struct {
	mock.Mock
}

func (m *MockSessionStore) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	return m.Called(ctx, key, value, ttl).Error(0)
}
func (m *MockSessionStore) Get(ctx context.Context, key string) (string, error) {
	args := m.Called(ctx, key)
	return args.String(0), args.Error(1)
}
func (m *MockSessionStore) Del(ctx context.Context, key string) error {
	return m.Called(ctx, key).Error(0)
}

type MockProducer struct {
	mock.Mock
}

func (m *MockProducer) Publish(ctx context.Context, topic, key string, msg proto.Message) error {
	return m.Called(ctx, topic, key, msg).Error(0)
}

// ---------- helpers ----------

const testSecret = "test-secret-key"
const testTTL = 60

func newTestService(repo staffRepo, sessions redisStore, producer eventProducer) *Service {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return New(repo, sessions, producer, testTTL, testSecret, logger)
}

func hashPassword(t *testing.T, password string) string {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	require.NoError(t, err)
	return string(hash)
}

// ---------- Login ----------

func TestLogin_Success(t *testing.T) {
	ctx := context.Background()
	password := "secret123"
	barber := &model.Barber{ID: "barber-1", Login: "ivan", PasswordHash: hashPassword(t, password)}

	repo := new(MockRepo)
	repo.On("GetBarberByLogin", ctx, "ivan").Return(barber, nil)

	sessions := new(MockSessionStore)
	sessions.On("Set", ctx, "session:barber-1", mock.AnythingOfType("string"), time.Duration(testTTL)*time.Minute).Return(nil)

	svc := newTestService(repo, sessions, new(MockProducer))

	b, accessToken, refreshToken, err := svc.Login(ctx, "ivan", password)

	require.NoError(t, err)
	assert.Equal(t, barber.ID, b.ID)
	assert.NotEmpty(t, accessToken)
	assert.NotEmpty(t, refreshToken)
	repo.AssertExpectations(t)
	sessions.AssertExpectations(t)
}

func TestLogin_BarberNotFound(t *testing.T) {
	ctx := context.Background()

	repo := new(MockRepo)
	repo.On("GetBarberByLogin", ctx, "unknown").Return((*model.Barber)(nil), errors.New("not found"))

	svc := newTestService(repo, new(MockSessionStore), new(MockProducer))

	_, _, _, err := svc.Login(ctx, "unknown", "pass")

	var appErr *apperr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeUnauthenticated, appErr.Code)
	repo.AssertExpectations(t)
}

func TestLogin_WrongPassword(t *testing.T) {
	ctx := context.Background()
	barber := &model.Barber{ID: "barber-1", PasswordHash: hashPassword(t, "correct")}

	repo := new(MockRepo)
	repo.On("GetBarberByLogin", ctx, "ivan").Return(barber, nil)

	svc := newTestService(repo, new(MockSessionStore), new(MockProducer))

	_, _, _, err := svc.Login(ctx, "ivan", "wrong")

	var appErr *apperr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeUnauthenticated, appErr.Code)
}

func TestLogin_PasswordTooLong(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(new(MockRepo), new(MockSessionStore), new(MockProducer))

	_, _, _, err := svc.Login(ctx, "ivan", string(make([]byte, 73)))

	var appErr *apperr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeInvalidArgument, appErr.Code)
}

func TestLogin_SessionStoreError(t *testing.T) {
	ctx := context.Background()
	password := "secret"
	barber := &model.Barber{ID: "barber-1", PasswordHash: hashPassword(t, password)}

	repo := new(MockRepo)
	repo.On("GetBarberByLogin", ctx, "ivan").Return(barber, nil)

	sessions := new(MockSessionStore)
	sessions.On("Set", ctx, "session:barber-1", mock.AnythingOfType("string"), time.Duration(testTTL)*time.Minute).Return(errors.New("redis unavailable"))

	svc := newTestService(repo, sessions, new(MockProducer))

	_, _, _, err := svc.Login(ctx, "ivan", password)

	var appErr *apperr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeInternal, appErr.Code)
}

// ---------- Logout ----------

func TestLogout_Success(t *testing.T) {
	ctx := context.Background()
	barberID := "barber-1"

	refreshToken, err := auth.GenerateRefreshToken(barberID, testSecret)
	require.NoError(t, err)

	sessions := new(MockSessionStore)
	sessions.On("Del", ctx, "session:"+barberID).Return(nil)

	svc := newTestService(new(MockRepo), sessions, new(MockProducer))

	err = svc.Logout(ctx, refreshToken)
	require.NoError(t, err)
	sessions.AssertExpectations(t)
}

func TestLogout_InvalidToken(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(new(MockRepo), new(MockSessionStore), new(MockProducer))

	err := svc.Logout(ctx, "not-a-jwt")

	var appErr *apperr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeUnauthenticated, appErr.Code)
}

// ---------- RefreshToken ----------

func TestRefreshToken_Success(t *testing.T) {
	ctx := context.Background()
	barberID := "barber-1"

	refreshToken, err := auth.GenerateRefreshToken(barberID, testSecret)
	require.NoError(t, err)

	sessions := new(MockSessionStore)
	sessions.On("Get", ctx, "session:"+barberID).Return(refreshToken, nil)
	sessions.On("Set", ctx, "session:"+barberID, mock.AnythingOfType("string"), time.Duration(testTTL)*time.Minute).Return(nil)

	svc := newTestService(new(MockRepo), sessions, new(MockProducer))

	newAccess, newRefresh, err := svc.RefreshToken(ctx, refreshToken)

	require.NoError(t, err)
	assert.NotEmpty(t, newAccess)
	assert.NotEmpty(t, newRefresh)
	claims, err := auth.ValidateToken(newRefresh, testSecret)
	require.NoError(t, err)
	assert.Equal(t, barberID, claims.BarberID)
	sessions.AssertExpectations(t)
}

func TestRefreshToken_InvalidToken(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(new(MockRepo), new(MockSessionStore), new(MockProducer))

	_, _, err := svc.RefreshToken(ctx, "bad-token")

	var appErr *apperr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeUnauthenticated, appErr.Code)
}

func TestRefreshToken_SessionMismatch(t *testing.T) {
	ctx := context.Background()
	barberID := "barber-1"

	refreshToken, err := auth.GenerateRefreshToken(barberID, testSecret)
	require.NoError(t, err)

	sessions := new(MockSessionStore)
	sessions.On("Get", ctx, "session:"+barberID).Return("other-token", nil)

	svc := newTestService(new(MockRepo), sessions, new(MockProducer))

	_, _, err = svc.RefreshToken(ctx, refreshToken)

	var appErr *apperr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeUnauthenticated, appErr.Code)
}

func TestRefreshToken_SessionNotFound(t *testing.T) {
	ctx := context.Background()
	barberID := "barber-1"

	refreshToken, err := auth.GenerateRefreshToken(barberID, testSecret)
	require.NoError(t, err)

	sessions := new(MockSessionStore)
	sessions.On("Get", ctx, "session:"+barberID).Return("", errors.New("key not found"))

	svc := newTestService(new(MockRepo), sessions, new(MockProducer))

	_, _, err = svc.RefreshToken(ctx, refreshToken)

	var appErr *apperr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeUnauthenticated, appErr.Code)
}

// ---------- GetBarber ----------

func TestGetBarber_Success(t *testing.T) {
	ctx := context.Background()
	expected := &model.Barber{ID: "b1", Name: "Ivan"}

	repo := new(MockRepo)
	repo.On("GetBarber", ctx, "b1").Return(expected, nil)

	svc := newTestService(repo, new(MockSessionStore), new(MockProducer))

	b, err := svc.GetBarber(ctx, "b1")

	require.NoError(t, err)
	assert.Equal(t, expected, b)
	repo.AssertExpectations(t)
}

func TestGetBarber_EmptyID(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(new(MockRepo), new(MockSessionStore), new(MockProducer))

	_, err := svc.GetBarber(ctx, "")

	var appErr *apperr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeInvalidArgument, appErr.Code)
}

func TestGetBarber_NotFound(t *testing.T) {
	ctx := context.Background()

	repo := new(MockRepo)
	repo.On("GetBarber", ctx, "b1").Return((*model.Barber)(nil), errors.New("not found"))

	svc := newTestService(repo, new(MockSessionStore), new(MockProducer))

	_, err := svc.GetBarber(ctx, "b1")

	var appErr *apperr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeNotFound, appErr.Code)
	repo.AssertExpectations(t)
}

// ---------- ListBarbers ----------

func TestListBarbers_Success(t *testing.T) {
	ctx := context.Background()
	expected := []model.Barber{{ID: "b1"}, {ID: "b2"}}

	repo := new(MockRepo)
	repo.On("ListBarbers", ctx).Return(expected, nil)

	svc := newTestService(repo, new(MockSessionStore), new(MockProducer))

	barbers, err := svc.ListBarbers(ctx)

	require.NoError(t, err)
	assert.Equal(t, expected, barbers)
	repo.AssertExpectations(t)
}

func TestListBarbers_RepoError(t *testing.T) {
	ctx := context.Background()

	repo := new(MockRepo)
	repo.On("ListBarbers", ctx).Return([]model.Barber(nil), errors.New("db error"))

	svc := newTestService(repo, new(MockSessionStore), new(MockProducer))

	_, err := svc.ListBarbers(ctx)

	var appErr *apperr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeInternal, appErr.Code)
}

// ---------- CreateService ----------

func TestCreateService_Success(t *testing.T) {
	ctx := context.Background()
	s := &model.Service{BarberID: "b1", Name: "Haircut", Price: 500, DurationMinutes: 60}

	repo := new(MockRepo)
	repo.On("CreateService", ctx, s).Return(nil)

	svc := newTestService(repo, new(MockSessionStore), new(MockProducer))

	err := svc.CreateService(ctx, s)

	require.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestCreateService_EmptyBarberID(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(new(MockRepo), new(MockSessionStore), new(MockProducer))

	err := svc.CreateService(ctx, &model.Service{Name: "Haircut", Price: 100, DurationMinutes: 60})

	var appErr *apperr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeInvalidArgument, appErr.Code)
}

func TestCreateService_ShortName(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(new(MockRepo), new(MockSessionStore), new(MockProducer))

	err := svc.CreateService(ctx, &model.Service{BarberID: "b1", Name: "H", Price: 100, DurationMinutes: 60})

	var appErr *apperr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeInvalidArgument, appErr.Code)
}

func TestCreateService_NegativePrice(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(new(MockRepo), new(MockSessionStore), new(MockProducer))

	err := svc.CreateService(ctx, &model.Service{BarberID: "b1", Name: "Haircut", Price: -1, DurationMinutes: 60})

	var appErr *apperr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeInvalidArgument, appErr.Code)
}

func TestCreateService_InvalidDuration(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(new(MockRepo), new(MockSessionStore), new(MockProducer))

	for _, d := range []int{0, -15, 10, 25, 50} {
		err := svc.CreateService(ctx, &model.Service{BarberID: "b1", Name: "Haircut", Price: 100, DurationMinutes: d})
		var appErr *apperr.AppError
		require.ErrorAs(t, err, &appErr, "duration: %d", d)
		assert.Equal(t, apperr.CodeInvalidArgument, appErr.Code, "duration: %d", d)
	}
}

func TestCreateService_RepoError(t *testing.T) {
	ctx := context.Background()
	s := &model.Service{BarberID: "b1", Name: "Haircut", Price: 500, DurationMinutes: 60}

	repo := new(MockRepo)
	repo.On("CreateService", ctx, s).Return(errors.New("db error"))

	svc := newTestService(repo, new(MockSessionStore), new(MockProducer))

	err := svc.CreateService(ctx, s)

	var appErr *apperr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeInternal, appErr.Code)
	repo.AssertExpectations(t)
}


// ---------- UpdateService ----------

func TestUpdateService_Success(t *testing.T) {
	ctx := context.Background()
	s := &model.Service{ID: "svc-1", BarberID: "b1", Name: "Haircut", Price: 600, DurationMinutes: 45}

	repo := new(MockRepo)
	repo.On("UpdateService", ctx, s).Return(nil)

	svc := newTestService(repo, new(MockSessionStore), new(MockProducer))

	err := svc.UpdateService(ctx, s)

	require.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestUpdateService_EmptyServiceID(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(new(MockRepo), new(MockSessionStore), new(MockProducer))

	err := svc.UpdateService(ctx, &model.Service{BarberID: "b1", Name: "Haircut", Price: 100, DurationMinutes: 60})

	var appErr *apperr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeInvalidArgument, appErr.Code)
}

func TestUpdateService_EmptyBarberID(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(new(MockRepo), new(MockSessionStore), new(MockProducer))

	err := svc.UpdateService(ctx, &model.Service{ID: "svc-1", Name: "Haircut", Price: 100, DurationMinutes: 60})

	var appErr *apperr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeInvalidArgument, appErr.Code)
}

func TestUpdateService_ShortName(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(new(MockRepo), new(MockSessionStore), new(MockProducer))

	err := svc.UpdateService(ctx, &model.Service{ID: "svc-1", BarberID: "b1", Name: "X", Price: 100, DurationMinutes: 60})

	var appErr *apperr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeInvalidArgument, appErr.Code)
}

func TestUpdateService_NegativePrice(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(new(MockRepo), new(MockSessionStore), new(MockProducer))

	err := svc.UpdateService(ctx, &model.Service{ID: "svc-1", BarberID: "b1", Name: "Haircut", Price: -5, DurationMinutes: 60})

	var appErr *apperr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeInvalidArgument, appErr.Code)
}

func TestUpdateService_InvalidDuration(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(new(MockRepo), new(MockSessionStore), new(MockProducer))

	for _, d := range []int{0, -15, 10, 25} {
		err := svc.UpdateService(ctx, &model.Service{ID: "svc-1", BarberID: "b1", Name: "Haircut", Price: 100, DurationMinutes: d})
		var appErr *apperr.AppError
		require.ErrorAs(t, err, &appErr, "duration: %d", d)
		assert.Equal(t, apperr.CodeInvalidArgument, appErr.Code, "duration: %d", d)
	}
}

func TestUpdateService_RepoError(t *testing.T) {
	ctx := context.Background()
	s := &model.Service{ID: "svc-1", BarberID: "b1", Name: "Haircut", Price: 100, DurationMinutes: 60}

	repo := new(MockRepo)
	repo.On("UpdateService", ctx, s).Return(errors.New("db error"))

	svc := newTestService(repo, new(MockSessionStore), new(MockProducer))

	err := svc.UpdateService(ctx, s)

	var appErr *apperr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeInternal, appErr.Code)
	repo.AssertExpectations(t)
}

func TestUpdateService_NotFound(t *testing.T) {
	ctx := context.Background()
	s := &model.Service{ID: "svc-1", BarberID: "b1", Name: "Haircut", Price: 100, DurationMinutes: 60}

	repo := new(MockRepo)
	repo.On("UpdateService", ctx, s).Return(errors.New("service not found"))

	svc := newTestService(repo, new(MockSessionStore), new(MockProducer))

	err := svc.UpdateService(ctx, s)

	var appErr *apperr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeInternal, appErr.Code)
	repo.AssertExpectations(t)
}

// ---------- DeleteService ----------

func TestDeleteService_Success(t *testing.T) {
	ctx := context.Background()

	repo := new(MockRepo)
	repo.On("DeleteService", ctx, "svc-1", "b1").Return(nil)

	svc := newTestService(repo, new(MockSessionStore), new(MockProducer))

	err := svc.DeleteService(ctx, "svc-1", "b1")

	require.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestDeleteService_EmptyServiceID(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(new(MockRepo), new(MockSessionStore), new(MockProducer))

	err := svc.DeleteService(ctx, "", "b1")

	var appErr *apperr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeInvalidArgument, appErr.Code)
}

func TestDeleteService_EmptyBarberID(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(new(MockRepo), new(MockSessionStore), new(MockProducer))

	err := svc.DeleteService(ctx, "svc-1", "")

	var appErr *apperr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeInvalidArgument, appErr.Code)
}

func TestDeleteService_NotFound(t *testing.T) {
	ctx := context.Background()

	repo := new(MockRepo)
	repo.On("DeleteService", ctx, "svc-1", "b1").Return(errors.New("service not found"))

	svc := newTestService(repo, new(MockSessionStore), new(MockProducer))

	err := svc.DeleteService(ctx, "svc-1", "b1")

	var appErr *apperr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeNotFound, appErr.Code)
	repo.AssertExpectations(t)
}

// ---------- ListServices ----------

func TestListServices_Success(t *testing.T) {
	ctx := context.Background()
	expected := []model.Service{{ID: "s1"}, {ID: "s2"}}

	repo := new(MockRepo)
	repo.On("ListServices", ctx, "b1", false).Return(expected, nil)

	svc := newTestService(repo, new(MockSessionStore), new(MockProducer))

	services, err := svc.ListServices(ctx, "b1", false)

	require.NoError(t, err)
	assert.Equal(t, expected, services)
	repo.AssertExpectations(t)
}

func TestListServices_EmptyBarberID(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(new(MockRepo), new(MockSessionStore), new(MockProducer))

	_, err := svc.ListServices(ctx, "", false)

	var appErr *apperr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeInvalidArgument, appErr.Code)
}

func TestListServices_RepoError(t *testing.T) {
	ctx := context.Background()

	repo := new(MockRepo)
	repo.On("ListServices", ctx, "b1", false).Return([]model.Service(nil), errors.New("db error"))

	svc := newTestService(repo, new(MockSessionStore), new(MockProducer))

	_, err := svc.ListServices(ctx, "b1", false)

	var appErr *apperr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeInternal, appErr.Code)
}

// ---------- GetSchedule ----------

func TestGetSchedule_Success(t *testing.T) {
	ctx := context.Background()
	expected := []model.ScheduleDay{{ID: "d1", Date: "2026-03-03"}}

	repo := new(MockRepo)
	repo.On("GetSchedule", ctx, "b1", "2026-W10").Return(expected, nil)

	svc := newTestService(repo, new(MockSessionStore), new(MockProducer))

	days, err := svc.GetSchedule(ctx, "b1", "2026-W10")

	require.NoError(t, err)
	assert.Equal(t, expected, days)
	repo.AssertExpectations(t)
}

func TestGetSchedule_EmptyBarberID(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(new(MockRepo), new(MockSessionStore), new(MockProducer))

	_, err := svc.GetSchedule(ctx, "", "2026-W10")

	var appErr *apperr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeInvalidArgument, appErr.Code)
}

func TestGetSchedule_InvalidWeekFormat(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(new(MockRepo), new(MockSessionStore), new(MockProducer))

	for _, bad := range []string{"", "2026", "W10", "2026-10", "2026-W00", "2026-W54", "2026-w10"} {
		_, err := svc.GetSchedule(ctx, "b1", bad)
		var appErr *apperr.AppError
		require.ErrorAs(t, err, &appErr, "input: %q", bad)
		assert.Equal(t, apperr.CodeInvalidArgument, appErr.Code, "input: %q", bad)
	}
}

func TestGetSchedule_RepoError(t *testing.T) {
	ctx := context.Background()

	repo := new(MockRepo)
	repo.On("GetSchedule", ctx, "b1", "2026-W10").Return([]model.ScheduleDay(nil), errors.New("db error"))

	svc := newTestService(repo, new(MockSessionStore), new(MockProducer))

	_, err := svc.GetSchedule(ctx, "b1", "2026-W10")

	var appErr *apperr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeInternal, appErr.Code)
}

// ---------- UpsertSchedule ----------

func TestUpsertSchedule_Success(t *testing.T) {
	ctx := context.Background()
	day := &model.ScheduleDay{Date: "2026-03-03", StartTime: "09:00", EndTime: "18:00", PartOfDay: model.PartOfDayAM}
	result := &model.ScheduleDay{ID: "day-1", BarberID: "b1", Date: "2026-03-03", StartTime: "09:00", EndTime: "18:00", PartOfDay: model.PartOfDayAM}

	repo := new(MockRepo)
	repo.On("UpsertSchedule", ctx, "b1", day).Return(result, nil)

	producer := new(MockProducer)
	producer.On("Publish", ctx, "schedule.events", "b1", mock.Anything).Return(nil)

	svc := newTestService(repo, new(MockSessionStore), producer)

	got, err := svc.UpsertSchedule(ctx, "b1", day)

	require.NoError(t, err)
	assert.Equal(t, result, got)
	repo.AssertExpectations(t)
	producer.AssertExpectations(t)
}

func TestUpsertSchedule_EmptyBarberID(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(new(MockRepo), new(MockSessionStore), new(MockProducer))

	_, err := svc.UpsertSchedule(ctx, "", &model.ScheduleDay{Date: "2026-03-03", StartTime: "09:00", EndTime: "18:00"})

	var appErr *apperr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeInvalidArgument, appErr.Code)
}

func TestUpsertSchedule_InvalidDate(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(new(MockRepo), new(MockSessionStore), new(MockProducer))

	for _, bad := range []string{"", "03-03-2026", "2026/03/03", "2026-3-3"} {
		_, err := svc.UpsertSchedule(ctx, "b1", &model.ScheduleDay{Date: bad, StartTime: "09:00", EndTime: "18:00"})
		var appErr *apperr.AppError
		require.ErrorAs(t, err, &appErr, "input: %q", bad)
		assert.Equal(t, apperr.CodeInvalidArgument, appErr.Code, "input: %q", bad)
	}
}

func TestUpsertSchedule_InvalidStartTime(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(new(MockRepo), new(MockSessionStore), new(MockProducer))

	for _, bad := range []string{"", "9:00", "09:00:00", "0900"} {
		_, err := svc.UpsertSchedule(ctx, "b1", &model.ScheduleDay{Date: "2026-03-03", StartTime: bad, EndTime: "18:00"})
		var appErr *apperr.AppError
		require.ErrorAs(t, err, &appErr, "input: %q", bad)
		assert.Equal(t, apperr.CodeInvalidArgument, appErr.Code, "input: %q", bad)
	}
}

func TestUpsertSchedule_InvalidEndTime(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(new(MockRepo), new(MockSessionStore), new(MockProducer))

	for _, bad := range []string{"", "18", "6:00pm", "1800"} {
		_, err := svc.UpsertSchedule(ctx, "b1", &model.ScheduleDay{Date: "2026-03-03", StartTime: "09:00", EndTime: bad})
		var appErr *apperr.AppError
		require.ErrorAs(t, err, &appErr, "input: %q", bad)
		assert.Equal(t, apperr.CodeInvalidArgument, appErr.Code, "input: %q", bad)
	}
}

func TestUpsertSchedule_StartTimeNotBeforeEndTime(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(new(MockRepo), new(MockSessionStore), new(MockProducer))

	for _, tc := range []struct{ start, end string }{
		{"18:00", "09:00"},
		{"09:00", "09:00"},
	} {
		_, err := svc.UpsertSchedule(ctx, "b1", &model.ScheduleDay{Date: "2026-03-03", StartTime: tc.start, EndTime: tc.end})
		var appErr *apperr.AppError
		require.ErrorAs(t, err, &appErr, "start=%q end=%q", tc.start, tc.end)
		assert.Equal(t, apperr.CodeInvalidArgument, appErr.Code, "start=%q end=%q", tc.start, tc.end)
	}
}

func TestUpsertSchedule_InvalidPartOfDay(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(new(MockRepo), new(MockSessionStore), new(MockProducer))

	_, err := svc.UpsertSchedule(ctx, "b1", &model.ScheduleDay{Date: "2026-03-03", StartTime: "09:00", EndTime: "18:00", PartOfDay: "noon"})

	var appErr *apperr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeInvalidArgument, appErr.Code)
}

func TestUpsertSchedule_RepoError(t *testing.T) {
	ctx := context.Background()
	day := &model.ScheduleDay{Date: "2026-03-03", StartTime: "09:00", EndTime: "18:00", PartOfDay: model.PartOfDayAM}

	repo := new(MockRepo)
	repo.On("UpsertSchedule", ctx, "b1", day).Return((*model.ScheduleDay)(nil), errors.New("db error"))

	svc := newTestService(repo, new(MockSessionStore), new(MockProducer))

	_, err := svc.UpsertSchedule(ctx, "b1", day)

	var appErr *apperr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeInternal, appErr.Code)
	repo.AssertExpectations(t)
}

// ---------- DeleteSchedule ----------

func TestDeleteSchedule_Success(t *testing.T) {
	ctx := context.Background()

	repo := new(MockRepo)
	repo.On("DeleteSchedule", ctx, "b1", "2026-03-03").Return("sched-1", nil)

	producer := new(MockProducer)
	producer.On("Publish", ctx, "schedule.events", "b1", mock.Anything).Return(nil)

	svc := newTestService(repo, new(MockSessionStore), producer)

	err := svc.DeleteSchedule(ctx, "b1", "2026-03-03")

	require.NoError(t, err)
	repo.AssertExpectations(t)
	producer.AssertExpectations(t)
}

func TestDeleteSchedule_EmptyBarberID(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(new(MockRepo), new(MockSessionStore), new(MockProducer))

	err := svc.DeleteSchedule(ctx, "", "2026-03-03")

	var appErr *apperr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeInvalidArgument, appErr.Code)
}

func TestDeleteSchedule_InvalidDate(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(new(MockRepo), new(MockSessionStore), new(MockProducer))

	for _, bad := range []string{"", "03-03-2026", "2026/03/03"} {
		err := svc.DeleteSchedule(ctx, "b1", bad)
		var appErr *apperr.AppError
		require.ErrorAs(t, err, &appErr, "input: %q", bad)
		assert.Equal(t, apperr.CodeInvalidArgument, appErr.Code, "input: %q", bad)
	}
}

func TestDeleteSchedule_NotFound(t *testing.T) {
	ctx := context.Background()

	repo := new(MockRepo)
	repo.On("DeleteSchedule", ctx, "b1", "2026-03-03").Return("", repository.ErrNotFound)

	svc := newTestService(repo, new(MockSessionStore), new(MockProducer))

	err := svc.DeleteSchedule(ctx, "b1", "2026-03-03")

	var appErr *apperr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeNotFound, appErr.Code)
	repo.AssertExpectations(t)
}
