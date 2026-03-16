package bookingrpc

import (
	"context"
	"testing"
	"time"

	pb "github.com/RomanKovalev007/barber_crm/api/proto/booking/v1"
	"github.com/RomanKovalev007/barber_crm/services/booking/internal/apperr"
	"github.com/RomanKovalev007/barber_crm/services/booking/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ─── Mock ─────────────────────────────────────────────────────────────────────

type mockSvc struct{ mock.Mock }

func (m *mockSvc) CreateBooking(ctx context.Context, b *model.Booking) (*model.Booking, error) {
	args := m.Called(ctx, b)
	return args.Get(0).(*model.Booking), args.Error(1)
}
func (m *mockSvc) GetBooking(ctx context.Context, id, barberID string) (*model.Booking, error) {
	args := m.Called(ctx, id, barberID)
	return args.Get(0).(*model.Booking), args.Error(1)
}
func (m *mockSvc) UpdateBookingDetails(ctx context.Context, bookingID, barberID, serviceID string, timeStart time.Time) (*model.Booking, error) {
	args := m.Called(ctx, bookingID, barberID, serviceID, timeStart)
	return args.Get(0).(*model.Booking), args.Error(1)
}
func (m *mockSvc) UpdateBookingStatus(ctx context.Context, bookingID, barberID, newStatus string) (*model.Booking, error) {
	args := m.Called(ctx, bookingID, barberID, newStatus)
	return args.Get(0).(*model.Booking), args.Error(1)
}
func (m *mockSvc) DeleteBooking(ctx context.Context, id, barberID string) error {
	return m.Called(ctx, id, barberID).Error(0)
}
func (m *mockSvc) GetSlots(ctx context.Context, barberID string, date time.Time) (*model.SlotsResult, error) {
	args := m.Called(ctx, barberID, date)
	return args.Get(0).(*model.SlotsResult), args.Error(1)
}
func (m *mockSvc) GetFreeSlots(ctx context.Context, barberID string, date time.Time) (*model.SlotsResult, error) {
	args := m.Called(ctx, barberID, date)
	return args.Get(0).(*model.SlotsResult), args.Error(1)
}

func newServer() (*bookingServer, *mockSvc) {
	svc := &mockSvc{}
	return NewServer(svc), svc
}

func grpcCode(err error) codes.Code {
	st, _ := status.FromError(err)
	return st.Code()
}

func futureTime() time.Time {
	return time.Now().Add(24 * time.Hour)
}

func sampleBooking() *model.Booking {
	return &model.Booking{
		ID:          "bk-1",
		ClientName:  "Ivan",
		ClientPhone: "+79991234567",
		BarberID:    "barber-1",
		ServiceID:   "svc-1",
		TimeStart:   futureTime(),
		TimeEnd:     futureTime().Add(time.Hour),
		Status:      model.StatusPending,
	}
}

// ─── CreateBooking ────────────────────────────────────────────────────────────

func TestCreateBooking_MissingFields(t *testing.T) {
	srv, _ := newServer()
	ctx := context.Background()
	ts := timestamppb.New(futureTime())

	cases := []struct {
		name string
		req  *pb.CreateBookingRequest
	}{
		{"no client_name", &pb.CreateBookingRequest{BarberId: "b", ServiceId: "s", ClientPhone: "+79991234567", TimeStart: ts}},
		{"no barber_id", &pb.CreateBookingRequest{ClientName: "Ivan", ServiceId: "s", ClientPhone: "+79991234567", TimeStart: ts}},
		{"no service_id", &pb.CreateBookingRequest{ClientName: "Ivan", BarberId: "b", ClientPhone: "+79991234567", TimeStart: ts}},
		{"no client_phone", &pb.CreateBookingRequest{ClientName: "Ivan", BarberId: "b", ServiceId: "s", TimeStart: ts}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := srv.CreateBooking(ctx, tc.req)
			assert.Equal(t, codes.InvalidArgument, grpcCode(err), tc.name)
		})
	}
}

func TestCreateBooking_InvalidPhone(t *testing.T) {
	srv, _ := newServer()
	_, err := srv.CreateBooking(context.Background(), &pb.CreateBookingRequest{
		ClientName:  "Ivan",
		BarberId:    "b",
		ServiceId:   "s",
		ClientPhone: "abc",
		TimeStart:   timestamppb.New(futureTime()),
	})
	assert.Equal(t, codes.InvalidArgument, grpcCode(err))
}

func TestCreateBooking_NoTimeStart(t *testing.T) {
	srv, _ := newServer()
	_, err := srv.CreateBooking(context.Background(), &pb.CreateBookingRequest{
		ClientName:  "Ivan",
		BarberId:    "b",
		ServiceId:   "s",
		ClientPhone: "+79991234567",
	})
	assert.Equal(t, codes.InvalidArgument, grpcCode(err))
}

func TestCreateBooking_PastTime(t *testing.T) {
	srv, _ := newServer()
	_, err := srv.CreateBooking(context.Background(), &pb.CreateBookingRequest{
		ClientName:  "Ivan",
		BarberId:    "b",
		ServiceId:   "s",
		ClientPhone: "+79991234567",
		TimeStart:   timestamppb.New(time.Now().Add(-time.Hour)),
	})
	assert.Equal(t, codes.InvalidArgument, grpcCode(err))
}

func TestCreateBooking_Success(t *testing.T) {
	srv, svc := newServer()
	b := sampleBooking()
	svc.On("CreateBooking", mock.Anything, mock.Anything).Return(b, nil)

	resp, err := srv.CreateBooking(context.Background(), &pb.CreateBookingRequest{
		ClientName:  "Ivan",
		BarberId:    "barber-1",
		ServiceId:   "svc-1",
		ClientPhone: "+79991234567",
		TimeStart:   timestamppb.New(futureTime()),
	})
	require.NoError(t, err)
	assert.Equal(t, "bk-1", resp.Booking.BookingId)
}

func TestCreateBooking_ServiceError(t *testing.T) {
	srv, svc := newServer()
	svc.On("CreateBooking", mock.Anything, mock.Anything).Return(&model.Booking{}, apperr.AlreadyExists("slot taken"))

	_, err := srv.CreateBooking(context.Background(), &pb.CreateBookingRequest{
		ClientName:  "Ivan",
		BarberId:    "b",
		ServiceId:   "s",
		ClientPhone: "+79991234567",
		TimeStart:   timestamppb.New(futureTime()),
	})
	assert.Equal(t, codes.AlreadyExists, grpcCode(err))
}

// ─── GetBooking ───────────────────────────────────────────────────────────────

func TestGetBooking_MissingFields(t *testing.T) {
	srv, _ := newServer()
	ctx := context.Background()

	_, err := srv.GetBooking(ctx, &pb.BookingIdRequest{BarberId: "b"})
	assert.Equal(t, codes.InvalidArgument, grpcCode(err))

	_, err = srv.GetBooking(ctx, &pb.BookingIdRequest{BookingId: "bk-1"})
	assert.Equal(t, codes.InvalidArgument, grpcCode(err))
}

func TestGetBooking_Success(t *testing.T) {
	srv, svc := newServer()
	svc.On("GetBooking", mock.Anything, "bk-1", "barber-1").Return(sampleBooking(), nil)

	resp, err := srv.GetBooking(context.Background(), &pb.BookingIdRequest{BookingId: "bk-1", BarberId: "barber-1"})
	require.NoError(t, err)
	assert.Equal(t, "bk-1", resp.Booking.BookingId)
}

func TestGetBooking_NotFound(t *testing.T) {
	srv, svc := newServer()
	svc.On("GetBooking", mock.Anything, "bk-x", "barber-1").Return(&model.Booking{}, apperr.NotFound("not found"))

	_, err := srv.GetBooking(context.Background(), &pb.BookingIdRequest{BookingId: "bk-x", BarberId: "barber-1"})
	assert.Equal(t, codes.NotFound, grpcCode(err))
}

// ─── UpdateBooking ────────────────────────────────────────────────────────────

func TestUpdateBooking_MissingFields(t *testing.T) {
	srv, _ := newServer()
	ctx := context.Background()
	ts := timestamppb.New(futureTime())

	_, err := srv.UpdateBooking(ctx, &pb.UpdateBookingRequest{BarberId: "b", TimeStart: ts})
	assert.Equal(t, codes.InvalidArgument, grpcCode(err))

	_, err = srv.UpdateBooking(ctx, &pb.UpdateBookingRequest{BookingId: "bk-1", TimeStart: ts})
	assert.Equal(t, codes.InvalidArgument, grpcCode(err))

	_, err = srv.UpdateBooking(ctx, &pb.UpdateBookingRequest{BookingId: "bk-1", BarberId: "b"})
	assert.Equal(t, codes.InvalidArgument, grpcCode(err))
}

func TestUpdateBooking_Success(t *testing.T) {
	srv, svc := newServer()
	ts := futureTime()
	b := sampleBooking()
	svc.On("UpdateBookingDetails", mock.Anything, "bk-1", "barber-1", "svc-1", mock.Anything).Return(b, nil)

	resp, err := srv.UpdateBooking(context.Background(), &pb.UpdateBookingRequest{
		BookingId: "bk-1",
		BarberId:  "barber-1",
		ServiceId: "svc-1",
		TimeStart: timestamppb.New(ts),
	})
	require.NoError(t, err)
	assert.Equal(t, "bk-1", resp.Booking.BookingId)
}

// ─── UpdateBookingStatus ──────────────────────────────────────────────────────

func TestUpdateBookingStatus_MissingFields(t *testing.T) {
	srv, _ := newServer()
	_, err := srv.UpdateBookingStatus(context.Background(), &pb.UpdateBookingStatusRequest{BarberId: "b"})
	assert.Equal(t, codes.InvalidArgument, grpcCode(err))
}

func TestUpdateBookingStatus_InvalidStatus(t *testing.T) {
	srv, _ := newServer()
	_, err := srv.UpdateBookingStatus(context.Background(), &pb.UpdateBookingStatusRequest{
		BookingId: "bk-1",
		BarberId:  "barber-1",
		Status:    pb.BookingStatus_BOOKING_STATUS_PENDING,
	})
	assert.Equal(t, codes.InvalidArgument, grpcCode(err))
}

func TestUpdateBookingStatus_Success(t *testing.T) {
	srv, svc := newServer()
	b := sampleBooking()
	b.Status = model.StatusCompleted
	svc.On("UpdateBookingStatus", mock.Anything, "bk-1", "barber-1", model.StatusCompleted).Return(b, nil)

	resp, err := srv.UpdateBookingStatus(context.Background(), &pb.UpdateBookingStatusRequest{
		BookingId: "bk-1",
		BarberId:  "barber-1",
		Status:    pb.BookingStatus_BOOKING_STATUS_COMPLETED,
	})
	require.NoError(t, err)
	assert.Equal(t, pb.BookingStatus_BOOKING_STATUS_COMPLETED, resp.Booking.Status)
}

// ─── DeleteBooking ────────────────────────────────────────────────────────────

func TestDeleteBooking_MissingFields(t *testing.T) {
	srv, _ := newServer()
	ctx := context.Background()

	_, err := srv.DeleteBooking(ctx, &pb.BookingIdRequest{BarberId: "b"})
	assert.Equal(t, codes.InvalidArgument, grpcCode(err))

	_, err = srv.DeleteBooking(ctx, &pb.BookingIdRequest{BookingId: "bk-1"})
	assert.Equal(t, codes.InvalidArgument, grpcCode(err))
}

func TestDeleteBooking_Success(t *testing.T) {
	srv, svc := newServer()
	svc.On("DeleteBooking", mock.Anything, "bk-1", "barber-1").Return(nil)

	_, err := srv.DeleteBooking(context.Background(), &pb.BookingIdRequest{BookingId: "bk-1", BarberId: "barber-1"})
	require.NoError(t, err)
}

func TestDeleteBooking_NotFound(t *testing.T) {
	srv, svc := newServer()
	svc.On("DeleteBooking", mock.Anything, "bk-x", "barber-1").Return(apperr.NotFound("not found"))

	_, err := srv.DeleteBooking(context.Background(), &pb.BookingIdRequest{BookingId: "bk-x", BarberId: "barber-1"})
	assert.Equal(t, codes.NotFound, grpcCode(err))
}

// ─── GetSlots ─────────────────────────────────────────────────────────────────

func TestGetSlots_MissingFields(t *testing.T) {
	srv, _ := newServer()
	ctx := context.Background()

	_, err := srv.GetSlots(ctx, &pb.SlotsRequest{Date: "2026-03-16"})
	assert.Equal(t, codes.InvalidArgument, grpcCode(err))

	_, err = srv.GetSlots(ctx, &pb.SlotsRequest{BarberId: "b"})
	assert.Equal(t, codes.InvalidArgument, grpcCode(err))
}

func TestGetSlots_InvalidDate(t *testing.T) {
	srv, _ := newServer()
	_, err := srv.GetSlots(context.Background(), &pb.SlotsRequest{BarberId: "b", Date: "not-a-date"})
	assert.Equal(t, codes.InvalidArgument, grpcCode(err))
}

func TestGetSlots_Success(t *testing.T) {
	srv, svc := newServer()
	result := &model.SlotsResult{BarberID: "b", Date: "2026-03-16", Slots: []model.Slot{}}
	svc.On("GetSlots", mock.Anything, "b", mock.Anything).Return(result, nil)

	resp, err := srv.GetSlots(context.Background(), &pb.SlotsRequest{BarberId: "b", Date: "2026-03-16"})
	require.NoError(t, err)
	assert.Equal(t, "b", resp.BarberId)
}

// ─── GetFreeSlots ─────────────────────────────────────────────────────────────

func TestGetFreeSlots_MissingBarberID(t *testing.T) {
	srv, _ := newServer()
	_, err := srv.GetFreeSlots(context.Background(), &pb.FreeSlotsRequest{})
	assert.Equal(t, codes.InvalidArgument, grpcCode(err))
}

func TestGetFreeSlots_InvalidDate(t *testing.T) {
	srv, _ := newServer()
	_, err := srv.GetFreeSlots(context.Background(), &pb.FreeSlotsRequest{BarberId: "b", Date: "bad"})
	assert.Equal(t, codes.InvalidArgument, grpcCode(err))
}

func TestGetFreeSlots_DefaultDate(t *testing.T) {
	srv, svc := newServer()
	result := &model.SlotsResult{BarberID: "b", Date: "2026-03-16", Slots: []model.Slot{}}
	svc.On("GetFreeSlots", mock.Anything, "b", mock.Anything).Return(result, nil)

	resp, err := srv.GetFreeSlots(context.Background(), &pb.FreeSlotsRequest{BarberId: "b"})
	require.NoError(t, err)
	assert.Equal(t, "b", resp.BarberId)
}

func TestGetFreeSlots_WithDate(t *testing.T) {
	srv, svc := newServer()
	result := &model.SlotsResult{BarberID: "b", Date: "2026-03-20", Slots: []model.Slot{}}
	svc.On("GetFreeSlots", mock.Anything, "b", mock.Anything).Return(result, nil)

	resp, err := srv.GetFreeSlots(context.Background(), &pb.FreeSlotsRequest{BarberId: "b", Date: "2026-03-20"})
	require.NoError(t, err)
	assert.Equal(t, "b", resp.BarberId)
}

// ─── toGRPCError ─────────────────────────────────────────────────────────────

func TestToGRPCError_AppError(t *testing.T) {
	cases := []struct {
		err      error
		expected codes.Code
	}{
		{apperr.NotFound("x"), codes.NotFound},
		{apperr.AlreadyExists("x"), codes.AlreadyExists},
		{apperr.InvalidArgument("x"), codes.InvalidArgument},
		{apperr.FailedPrecondition("x"), codes.FailedPrecondition},
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
