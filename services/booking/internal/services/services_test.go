package services

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	staffv1 "github.com/RomanKovalev007/barber_crm/api/proto/staff/v1"
	"github.com/RomanKovalev007/barber_crm/services/booking/internal/apperr"
	"github.com/RomanKovalev007/barber_crm/services/booking/internal/model"
	"github.com/RomanKovalev007/barber_crm/services/booking/internal/repo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// ---------- mocks ----------

type MockRepo struct {
	mock.Mock
}

func (m *MockRepo) CreateBookingTx(ctx context.Context, b *model.Booking) error {
	return m.Called(ctx, b).Error(0)
}
func (m *MockRepo) GetBooking(ctx context.Context, id string) (*model.Booking, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(*model.Booking), args.Error(1)
}
func (m *MockRepo) UpdateBookingDetailsTx(ctx context.Context, id, serviceID, serviceName string, price int32, timeStart, timeEnd time.Time) error {
	return m.Called(ctx, id, serviceID, serviceName, price, timeStart, timeEnd).Error(0)
}
func (m *MockRepo) UpdateBookingStatus(ctx context.Context, id, status string) error {
	return m.Called(ctx, id, status).Error(0)
}
func (m *MockRepo) DeleteBooking(ctx context.Context, id string) error {
	return m.Called(ctx, id).Error(0)
}
func (m *MockRepo) GetBookingsByBarberAndDate(ctx context.Context, barberID string, date time.Time) ([]model.Booking, error) {
	args := m.Called(ctx, barberID, date)
	return args.Get(0).([]model.Booking), args.Error(1)
}
func (m *MockRepo) GetClientBookings(ctx context.Context, barberID, clientPhone string, limit, offset int) ([]model.Booking, int, error) {
	args := m.Called(ctx, barberID, clientPhone, limit, offset)
	return args.Get(0).([]model.Booking), args.Int(1), args.Error(2)
}
func (m *MockRepo) GetCompactSlotsEnabled(ctx context.Context, barberID string) (bool, error) {
	args := m.Called(ctx, barberID)
	return args.Bool(0), args.Error(1)
}
func (m *MockRepo) SetCompactSlotsEnabled(ctx context.Context, barberID string, enabled bool) error {
	return m.Called(ctx, barberID, enabled).Error(0)
}

type MockStaffClient struct {
	mock.Mock
}

func (m *MockStaffClient) GetBarber(ctx context.Context, barberID string) (*staffv1.BarberResponse, error) {
	args := m.Called(ctx, barberID)
	return args.Get(0).(*staffv1.BarberResponse), args.Error(1)
}
func (m *MockStaffClient) ListServices(ctx context.Context, barberID string, includeInactive bool) (*staffv1.ListServicesResponse, error) {
	args := m.Called(ctx, barberID, includeInactive)
	return args.Get(0).(*staffv1.ListServicesResponse), args.Error(1)
}
func (m *MockStaffClient) GetSchedule(ctx context.Context, barberID, week string) (*staffv1.GetScheduleResponse, error) {
	args := m.Called(ctx, barberID, week)
	return args.Get(0).(*staffv1.GetScheduleResponse), args.Error(1)
}

type MockProducer struct {
	mock.Mock
}

func (m *MockProducer) Publish(ctx context.Context, key string, msg proto.Message) error {
	return m.Called(ctx, key, msg).Error(0)
}

// ---------- helpers ----------

func newTestService(r bookingRepo, sc staffClientIntr) *bookingService {
	p := new(MockProducer)
	p.On("Publish", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	return &bookingService{
		log:         slog.New(slog.NewTextHandler(io.Discard, nil)),
		repo:        r,
		staffClient: sc,
		producer:    p,
	}
}

func isoWeek(date time.Time) string {
	year, week := date.ISOWeek()
	return fmt.Sprintf("%d-W%02d", year, week)
}

var testDate = time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC) // Monday, 2026-W12

// ---------- CreateBooking ----------

func TestCreateBooking_Success(t *testing.T) {
	ctx := context.Background()
	timeStart := time.Date(2026, 3, 16, 10, 0, 0, 0, time.UTC)

	r := new(MockRepo)
	sc := new(MockStaffClient)

	sc.On("GetBarber", ctx, "b1").Return(&staffv1.BarberResponse{}, nil)
	sc.On("ListServices", ctx, "b1", false).Return(&staffv1.ListServicesResponse{
		Services: []*staffv1.ServiceResponse{{ServiceId: "svc-1", Name: "Haircut"}},
	}, nil)
	r.On("CreateBookingTx", ctx, mock.AnythingOfType("*model.Booking")).Return(nil)

	svc := newTestService(r, sc)

	b := &model.Booking{
		ClientName:  "Ivan",
		ClientPhone: "+79001234567",
		BarberID:    "b1",
		ServiceID:   "svc-1",
		TimeStart:   timeStart,
	}
	created, err := svc.CreateBooking(ctx, b)

	require.NoError(t, err)
	assert.Equal(t, model.StatusPending, created.Status)
	assert.Equal(t, "Haircut", created.ServiceName)
	assert.Equal(t, timeStart.Add(slotDuration), created.TimeEnd)
	r.AssertExpectations(t)
	sc.AssertExpectations(t)
}

func TestCreateBooking_BarberNotFound(t *testing.T) {
	ctx := context.Background()
	sc := new(MockStaffClient)
	sc.On("GetBarber", ctx, "b1").Return((*staffv1.BarberResponse)(nil), errors.New("not found"))

	svc := newTestService(new(MockRepo), sc)

	_, err := svc.CreateBooking(ctx, &model.Booking{BarberID: "b1", ClientPhone: "+7"})

	var appErr *apperr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeNotFound, appErr.Code)
	sc.AssertExpectations(t)
}

func TestCreateBooking_ListServicesError(t *testing.T) {
	ctx := context.Background()
	sc := new(MockStaffClient)
	sc.On("GetBarber", ctx, "b1").Return(&staffv1.BarberResponse{}, nil)
	sc.On("ListServices", ctx, "b1", false).Return((*staffv1.ListServicesResponse)(nil), errors.New("db error"))

	svc := newTestService(new(MockRepo), sc)

	_, err := svc.CreateBooking(ctx, &model.Booking{BarberID: "b1", ClientPhone: "+7"})

	var appErr *apperr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeInternal, appErr.Code)
}

func TestCreateBooking_ServiceNotFound(t *testing.T) {
	ctx := context.Background()
	sc := new(MockStaffClient)
	sc.On("GetBarber", ctx, "b1").Return(&staffv1.BarberResponse{}, nil)
	sc.On("ListServices", ctx, "b1", false).Return(&staffv1.ListServicesResponse{
		Services: []*staffv1.ServiceResponse{{ServiceId: "svc-other", Name: "Other"}},
	}, nil)

	svc := newTestService(new(MockRepo), sc)

	_, err := svc.CreateBooking(ctx, &model.Booking{BarberID: "b1", ServiceID: "svc-1", ClientPhone: "+7"})

	var appErr *apperr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeNotFound, appErr.Code)
}

func TestCreateBooking_ClientAlreadyHasActiveBooking(t *testing.T) {
	ctx := context.Background()
	sc := new(MockStaffClient)
	sc.On("GetBarber", ctx, "b1").Return(&staffv1.BarberResponse{}, nil)
	sc.On("ListServices", ctx, "b1", false).Return(&staffv1.ListServicesResponse{
		Services: []*staffv1.ServiceResponse{{ServiceId: "svc-1", Name: "Haircut", Price: 500}},
	}, nil)

	r := new(MockRepo)
	r.On("CreateBookingTx", ctx, mock.AnythingOfType("*model.Booking")).Return(repo.ErrActiveBookingExists)

	svc := newTestService(r, sc)

	_, err := svc.CreateBooking(ctx, &model.Booking{BarberID: "b1", ServiceID: "svc-1", ClientPhone: "+7"})

	var appErr *apperr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeAlreadyExists, appErr.Code)
}

func TestCreateBooking_SlotConflict(t *testing.T) {
	ctx := context.Background()
	sc := new(MockStaffClient)
	sc.On("GetBarber", ctx, "b1").Return(&staffv1.BarberResponse{}, nil)
	sc.On("ListServices", ctx, "b1", false).Return(&staffv1.ListServicesResponse{
		Services: []*staffv1.ServiceResponse{{ServiceId: "svc-1", Name: "Haircut", Price: 500}},
	}, nil)

	r := new(MockRepo)
	r.On("CreateBookingTx", ctx, mock.AnythingOfType("*model.Booking")).Return(repo.ErrSlotConflict)

	svc := newTestService(r, sc)

	_, err := svc.CreateBooking(ctx, &model.Booking{BarberID: "b1", ServiceID: "svc-1", ClientPhone: "+7"})

	var appErr *apperr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeAlreadyExists, appErr.Code)
}

func TestCreateBooking_RepoCreateError(t *testing.T) {
	ctx := context.Background()
	sc := new(MockStaffClient)
	sc.On("GetBarber", ctx, "b1").Return(&staffv1.BarberResponse{}, nil)
	sc.On("ListServices", ctx, "b1", false).Return(&staffv1.ListServicesResponse{
		Services: []*staffv1.ServiceResponse{{ServiceId: "svc-1", Name: "Haircut", Price: 500}},
	}, nil)

	r := new(MockRepo)
	r.On("CreateBookingTx", ctx, mock.AnythingOfType("*model.Booking")).Return(errors.New("db error"))

	svc := newTestService(r, sc)

	_, err := svc.CreateBooking(ctx, &model.Booking{BarberID: "b1", ServiceID: "svc-1", ClientPhone: "+7"})

	var appErr *apperr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeInternal, appErr.Code)
}

// ---------- GetBooking ----------

func TestGetBooking_Success(t *testing.T) {
	ctx := context.Background()
	expected := &model.Booking{ID: "bk-1", BarberID: "b1", ClientName: "Ivan"}

	r := new(MockRepo)
	r.On("GetBooking", ctx, "bk-1").Return(expected, nil)

	svc := newTestService(r, new(MockStaffClient))

	b, err := svc.GetBooking(ctx, "bk-1", "b1")

	require.NoError(t, err)
	assert.Equal(t, expected, b)
	r.AssertExpectations(t)
}

func TestGetBooking_OwnershipMismatch(t *testing.T) {
	ctx := context.Background()

	r := new(MockRepo)
	r.On("GetBooking", ctx, "bk-1").Return(&model.Booking{ID: "bk-1", BarberID: "b1"}, nil)

	svc := newTestService(r, new(MockStaffClient))

	_, err := svc.GetBooking(ctx, "bk-1", "b2")

	var appErr *apperr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeNotFound, appErr.Code)
}

func TestGetBooking_NotFound(t *testing.T) {
	ctx := context.Background()

	r := new(MockRepo)
	r.On("GetBooking", ctx, "bk-x").Return((*model.Booking)(nil), repo.ErrNotFound)

	svc := newTestService(r, new(MockStaffClient))

	_, err := svc.GetBooking(ctx, "bk-x", "b1")

	var appErr *apperr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeNotFound, appErr.Code)
}

func TestGetBooking_RepoError(t *testing.T) {
	ctx := context.Background()

	r := new(MockRepo)
	r.On("GetBooking", ctx, "bk-1").Return((*model.Booking)(nil), errors.New("db error"))

	svc := newTestService(r, new(MockStaffClient))

	_, err := svc.GetBooking(ctx, "bk-1", "b1")

	var appErr *apperr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeInternal, appErr.Code)
}

// ---------- UpdateBookingDetails ----------

func TestUpdateBookingDetails_Success(t *testing.T) {
	ctx := context.Background()
	timeStart := time.Date(2026, 3, 16, 14, 0, 0, 0, time.UTC)
	timeEnd := timeStart.Add(slotDuration)

	existing := &model.Booking{ID: "bk-1", BarberID: "b1", ServiceName: "OldCut"}
	updated := &model.Booking{ID: "bk-1", BarberID: "b1", ServiceID: "svc-2", ServiceName: "NewCut", TimeStart: timeStart, TimeEnd: timeEnd}

	sc := new(MockStaffClient)
	sc.On("ListServices", ctx, "b1", false).Return(&staffv1.ListServicesResponse{
		Services: []*staffv1.ServiceResponse{{ServiceId: "svc-2", Name: "NewCut"}},
	}, nil)

	r := new(MockRepo)
	r.On("GetBooking", ctx, "bk-1").Return(existing, nil).Once()
	r.On("UpdateBookingDetailsTx", ctx, "bk-1", "svc-2", "NewCut", int32(0), timeStart, timeEnd).Return(nil)
	r.On("GetBooking", ctx, "bk-1").Return(updated, nil).Once()

	svc := newTestService(r, sc)

	b, err := svc.UpdateBookingDetails(ctx, "bk-1", "b1", "svc-2", timeStart)

	require.NoError(t, err)
	assert.Equal(t, updated, b)
	r.AssertExpectations(t)
	sc.AssertExpectations(t)
}

func TestUpdateBookingDetails_NotFound(t *testing.T) {
	ctx := context.Background()

	r := new(MockRepo)
	r.On("GetBooking", ctx, "bk-x").Return((*model.Booking)(nil), repo.ErrNotFound)

	svc := newTestService(r, new(MockStaffClient))

	_, err := svc.UpdateBookingDetails(ctx, "bk-x", "b1", "svc-1", time.Now())

	var appErr *apperr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeNotFound, appErr.Code)
}

func TestUpdateBookingDetails_OwnershipMismatch(t *testing.T) {
	ctx := context.Background()

	r := new(MockRepo)
	r.On("GetBooking", ctx, "bk-1").Return(&model.Booking{ID: "bk-1", BarberID: "b1"}, nil)

	svc := newTestService(r, new(MockStaffClient))

	_, err := svc.UpdateBookingDetails(ctx, "bk-1", "b2", "svc-1", time.Now())

	var appErr *apperr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeNotFound, appErr.Code)
}

func TestUpdateBookingDetails_ServiceNotFound(t *testing.T) {
	ctx := context.Background()

	sc := new(MockStaffClient)
	sc.On("ListServices", ctx, "b1", false).Return(&staffv1.ListServicesResponse{
		Services: []*staffv1.ServiceResponse{{ServiceId: "svc-other", Name: "Other"}},
	}, nil)

	r := new(MockRepo)
	r.On("GetBooking", ctx, "bk-1").Return(&model.Booking{ID: "bk-1", BarberID: "b1"}, nil)

	svc := newTestService(r, sc)

	_, err := svc.UpdateBookingDetails(ctx, "bk-1", "b1", "svc-1", time.Now())

	var appErr *apperr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeNotFound, appErr.Code)
}

func TestUpdateBookingDetails_SlotConflict(t *testing.T) {
	ctx := context.Background()
	timeStart := time.Date(2026, 3, 16, 10, 0, 0, 0, time.UTC)
	timeEnd := timeStart.Add(slotDuration)

	sc := new(MockStaffClient)
	sc.On("ListServices", ctx, "b1", false).Return(&staffv1.ListServicesResponse{
		Services: []*staffv1.ServiceResponse{{ServiceId: "svc-1", Name: "Haircut", Price: 500}},
	}, nil)

	r := new(MockRepo)
	r.On("GetBooking", ctx, "bk-1").Return(&model.Booking{ID: "bk-1", BarberID: "b1"}, nil)
	r.On("UpdateBookingDetailsTx", ctx, "bk-1", "svc-1", "Haircut", int32(500), timeStart, timeEnd).Return(repo.ErrSlotConflict)

	svc := newTestService(r, sc)

	_, err := svc.UpdateBookingDetails(ctx, "bk-1", "b1", "svc-1", timeStart)

	var appErr *apperr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeAlreadyExists, appErr.Code)
}

func TestUpdateBookingDetails_NoConflictWithSelf(t *testing.T) {
	ctx := context.Background()
	timeStart := time.Date(2026, 3, 16, 10, 0, 0, 0, time.UTC)
	timeEnd := timeStart.Add(slotDuration)

	updated := &model.Booking{ID: "bk-1", BarberID: "b1"}

	sc := new(MockStaffClient)
	sc.On("ListServices", ctx, "b1", false).Return(&staffv1.ListServicesResponse{
		Services: []*staffv1.ServiceResponse{{ServiceId: "svc-1", Name: "Haircut", Price: 500}},
	}, nil)

	r := new(MockRepo)
	r.On("GetBooking", ctx, "bk-1").Return(&model.Booking{ID: "bk-1", BarberID: "b1"}, nil).Once()
	r.On("UpdateBookingDetailsTx", ctx, "bk-1", "svc-1", "Haircut", int32(500), timeStart, timeEnd).Return(nil)
	r.On("GetBooking", ctx, "bk-1").Return(updated, nil).Once()

	svc := newTestService(r, sc)

	_, err := svc.UpdateBookingDetails(ctx, "bk-1", "b1", "svc-1", timeStart)
	require.NoError(t, err)
}

// ---------- UpdateBookingStatus ----------

func TestUpdateBookingStatus_Success(t *testing.T) {
	ctx := context.Background()

	for _, newStatus := range []string{model.StatusCancelled, model.StatusCompleted, model.StatusNoShow} {
		r := new(MockRepo)
		r.On("GetBooking", ctx, "bk-1").Return(&model.Booking{ID: "bk-1", BarberID: "b1", Status: model.StatusPending}, nil).Once()
		r.On("UpdateBookingStatus", ctx, "bk-1", newStatus).Return(nil)
		r.On("GetBooking", ctx, "bk-1").Return(&model.Booking{ID: "bk-1", Status: newStatus}, nil).Once()

		svc := newTestService(r, new(MockStaffClient))

		b, err := svc.UpdateBookingStatus(ctx, "bk-1", "b1", newStatus)

		require.NoError(t, err, "status: %s", newStatus)
		assert.Equal(t, newStatus, b.Status, "status: %s", newStatus)
		r.AssertExpectations(t)
	}
}

func TestUpdateBookingStatus_NotFound(t *testing.T) {
	ctx := context.Background()

	r := new(MockRepo)
	r.On("GetBooking", ctx, "bk-x").Return((*model.Booking)(nil), repo.ErrNotFound)

	svc := newTestService(r, new(MockStaffClient))

	_, err := svc.UpdateBookingStatus(ctx, "bk-x", "b1", model.StatusCancelled)

	var appErr *apperr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeNotFound, appErr.Code)
}

func TestUpdateBookingStatus_OwnershipMismatch(t *testing.T) {
	ctx := context.Background()

	r := new(MockRepo)
	r.On("GetBooking", ctx, "bk-1").Return(&model.Booking{ID: "bk-1", BarberID: "b1", Status: model.StatusPending}, nil)

	svc := newTestService(r, new(MockStaffClient))

	_, err := svc.UpdateBookingStatus(ctx, "bk-1", "b2", model.StatusCancelled)

	var appErr *apperr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeNotFound, appErr.Code)
}

func TestUpdateBookingStatus_FinalToFinal(t *testing.T) {
	ctx := context.Background()

	// переход между финальными статусами разрешён (барбер мог ошибиться)
	transitions := [][2]string{
		{model.StatusCompleted, model.StatusCancelled},
		{model.StatusCancelled, model.StatusNoShow},
		{model.StatusNoShow, model.StatusCompleted},
	}
	for _, tr := range transitions {
		from, to := tr[0], tr[1]
		r := new(MockRepo)
		r.On("GetBooking", ctx, "bk-1").Return(&model.Booking{ID: "bk-1", BarberID: "b1", Status: from}, nil)
		r.On("UpdateBookingStatus", ctx, "bk-1", to).Return(nil)
		r.On("GetBooking", ctx, "bk-1").Return(&model.Booking{ID: "bk-1", BarberID: "b1", Status: to}, nil).Maybe()

		svc := newTestService(r, new(MockStaffClient))
		_, err := svc.UpdateBookingStatus(ctx, "bk-1", "b1", to)
		assert.NoError(t, err, "transition %s → %s should be allowed", from, to)
	}
}

func TestUpdateBookingStatus_RepoUpdateError(t *testing.T) {
	ctx := context.Background()

	r := new(MockRepo)
	r.On("GetBooking", ctx, "bk-1").Return(&model.Booking{ID: "bk-1", BarberID: "b1", Status: model.StatusPending}, nil)
	r.On("UpdateBookingStatus", ctx, "bk-1", model.StatusCancelled).Return(errors.New("db error"))

	svc := newTestService(r, new(MockStaffClient))

	_, err := svc.UpdateBookingStatus(ctx, "bk-1", "b1", model.StatusCancelled)

	var appErr *apperr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeInternal, appErr.Code)
}

// ---------- DeleteBooking ----------

func TestDeleteBooking_Success(t *testing.T) {
	ctx := context.Background()

	r := new(MockRepo)
	r.On("GetBooking", ctx, "bk-1").Return(&model.Booking{ID: "bk-1", BarberID: "b1"}, nil)
	r.On("DeleteBooking", ctx, "bk-1").Return(nil)

	svc := newTestService(r, new(MockStaffClient))

	err := svc.DeleteBooking(ctx, "bk-1", "b1")

	require.NoError(t, err)
	r.AssertExpectations(t)
}

func TestDeleteBooking_OwnershipMismatch(t *testing.T) {
	ctx := context.Background()

	r := new(MockRepo)
	r.On("GetBooking", ctx, "bk-1").Return(&model.Booking{ID: "bk-1", BarberID: "b1"}, nil)

	svc := newTestService(r, new(MockStaffClient))

	err := svc.DeleteBooking(ctx, "bk-1", "b2")

	var appErr *apperr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeNotFound, appErr.Code)
}

func TestDeleteBooking_NotFound(t *testing.T) {
	ctx := context.Background()

	r := new(MockRepo)
	r.On("GetBooking", ctx, "bk-x").Return((*model.Booking)(nil), repo.ErrNotFound)

	svc := newTestService(r, new(MockStaffClient))

	err := svc.DeleteBooking(ctx, "bk-x", "b1")

	var appErr *apperr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeNotFound, appErr.Code)
}

func TestDeleteBooking_RepoError(t *testing.T) {
	ctx := context.Background()

	r := new(MockRepo)
	r.On("GetBooking", ctx, "bk-1").Return(&model.Booking{ID: "bk-1", BarberID: "b1"}, nil)
	r.On("DeleteBooking", ctx, "bk-1").Return(errors.New("db error"))

	svc := newTestService(r, new(MockStaffClient))

	err := svc.DeleteBooking(ctx, "bk-1", "b1")

	var appErr *apperr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeInternal, appErr.Code)
}

// ---------- GetSlots ----------

func TestGetSlots_NoWorkingDay(t *testing.T) {
	ctx := context.Background()

	sc := new(MockStaffClient)
	sc.On("GetSchedule", ctx, "b1", isoWeek(testDate)).Return(&staffv1.GetScheduleResponse{
		Days: []*staffv1.ScheduleDay{},
	}, nil)

	svc := newTestService(new(MockRepo), sc)

	result, err := svc.GetSlots(ctx, "b1", testDate)

	require.NoError(t, err)
	assert.Empty(t, result.Slots)
	assert.Equal(t, "2026-03-16", result.Date)
}

func TestGetSlots_Success(t *testing.T) {
	ctx := context.Background()
	date := testDate
	dateTrunc := date.UTC().Truncate(24 * time.Hour)

	sc := new(MockStaffClient)
	sc.On("GetSchedule", ctx, "b1", isoWeek(date)).Return(&staffv1.GetScheduleResponse{
		Days: []*staffv1.ScheduleDay{{
			Date:      "2026-03-16",
			StartTime: "09:00",
			EndTime:   "11:00",
		}},
	}, nil)

	r := new(MockRepo)
	r.On("GetBookingsByBarberAndDate", ctx, "b1", dateTrunc).Return([]model.Booking{}, nil)

	svc := newTestService(r, sc)

	result, err := svc.GetSlots(ctx, "b1", date)

	require.NoError(t, err)
	assert.Len(t, result.Slots, 8) // 09:00..11:00 по 15 мин = 8 слотов
	assert.Equal(t, model.SlotFree, result.Slots[0].Status)
	assert.Nil(t, result.Slots[0].Booking)
	assert.Equal(t, time.Date(2026, 3, 16, 9, 0, 0, 0, time.UTC), result.Slots[0].TimeStart)
	assert.Equal(t, time.Date(2026, 3, 16, 9, 15, 0, 0, time.UTC), result.Slots[0].TimeEnd)
}

func TestGetSlots_WithBookedSlot(t *testing.T) {
	ctx := context.Background()
	date := testDate
	dateTrunc := date.UTC().Truncate(24 * time.Hour)

	slotStart := time.Date(2026, 3, 16, 9, 0, 0, 0, time.UTC)
	slotEnd := slotStart.Add(slotDuration)

	booking := model.Booking{
		ID:          "bk-1",
		ClientName:  "Ivan",
		ClientPhone: "+7",
		ServiceName: "Haircut",
		TimeStart:   slotStart,
		TimeEnd:     slotEnd,
	}

	sc := new(MockStaffClient)
	sc.On("GetSchedule", ctx, "b1", isoWeek(date)).Return(&staffv1.GetScheduleResponse{
		Days: []*staffv1.ScheduleDay{{
			Date:      "2026-03-16",
			StartTime: "09:00",
			EndTime:   "11:00",
		}},
	}, nil)

	r := new(MockRepo)
	r.On("GetBookingsByBarberAndDate", ctx, "b1", dateTrunc).Return([]model.Booking{booking}, nil)

	svc := newTestService(r, sc)

	result, err := svc.GetSlots(ctx, "b1", date)

	// 09:00..11:00 по 15 мин = 8 слотов; бронь 09:00-10:00 занимает первые 4
	require.NoError(t, err)
	require.Len(t, result.Slots, 8)
	assert.Equal(t, model.SlotBooked, result.Slots[0].Status)
	require.NotNil(t, result.Slots[0].Booking)
	assert.Equal(t, "bk-1", result.Slots[0].Booking.BookingID)
	assert.Equal(t, model.SlotBooked, result.Slots[3].Status)
	assert.Equal(t, model.SlotFree, result.Slots[4].Status)
	assert.Nil(t, result.Slots[4].Booking)
}

func TestGetSlots_GetScheduleError(t *testing.T) {
	ctx := context.Background()

	sc := new(MockStaffClient)
	sc.On("GetSchedule", ctx, "b1", isoWeek(testDate)).Return((*staffv1.GetScheduleResponse)(nil), errors.New("rpc error"))

	svc := newTestService(new(MockRepo), sc)

	_, err := svc.GetSlots(ctx, "b1", testDate)

	var appErr *apperr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeInternal, appErr.Code)
}

func TestGetSlots_GetBookingsError(t *testing.T) {
	ctx := context.Background()
	dateTrunc := testDate.UTC().Truncate(24 * time.Hour)

	sc := new(MockStaffClient)
	sc.On("GetSchedule", ctx, "b1", isoWeek(testDate)).Return(&staffv1.GetScheduleResponse{
		Days: []*staffv1.ScheduleDay{{Date: "2026-03-16", StartTime: "09:00", EndTime: "10:00"}},
	}, nil)

	r := new(MockRepo)
	r.On("GetBookingsByBarberAndDate", ctx, "b1", dateTrunc).Return([]model.Booking(nil), errors.New("db error"))

	svc := newTestService(r, sc)

	_, err := svc.GetSlots(ctx, "b1", testDate)

	var appErr *apperr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeInternal, appErr.Code)
}

// ---------- GetFreeSlots ----------

func TestGetFreeSlots_ReturnsOnlyFreeSlots(t *testing.T) {
	ctx := context.Background()
	date := testDate
	dateTrunc := date.UTC().Truncate(24 * time.Hour)

	slotStart := time.Date(2026, 3, 16, 9, 0, 0, 0, time.UTC)
	booking := model.Booking{
		ID:        "bk-1",
		TimeStart: slotStart,
		TimeEnd:   slotStart.Add(slotDuration),
	}

	sc := new(MockStaffClient)
	sc.On("GetSchedule", ctx, "b1", isoWeek(date)).Return(&staffv1.GetScheduleResponse{
		Days: []*staffv1.ScheduleDay{{Date: "2026-03-16", StartTime: "09:00", EndTime: "11:00"}},
	}, nil)

	r := new(MockRepo)
	r.On("GetCompactSlotsEnabled", ctx, "b1").Return(false, nil)
	r.On("GetBookingsByBarberAndDate", ctx, "b1", dateTrunc).Return([]model.Booking{booking}, nil)

	svc := newTestService(r, sc)

	result, err := svc.GetFreeSlots(ctx, "b1", date)

	require.NoError(t, err)
	require.Len(t, result.Slots, 1) // only the 10:00-11:00 slot
	assert.Equal(t, model.SlotFree, result.Slots[0].Status)
	assert.Equal(t, time.Date(2026, 3, 16, 10, 0, 0, 0, time.UTC), result.Slots[0].TimeStart)
}
