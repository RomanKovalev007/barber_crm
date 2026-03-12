package bookingrpc

import (
	"context"
	"errors"
	"time"

	pb "github.com/RomanKovalev007/barber_crm/api/proto/booking/v1"
	"github.com/RomanKovalev007/barber_crm/services/booking/internal/model"
	"github.com/RomanKovalev007/barber_crm/services/booking/internal/repo"
	"github.com/RomanKovalev007/barber_crm/services/booking/internal/services"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type bookingServer struct {
	pb.UnimplementedBookingServiceServer
	svc services.BookingIntr
}

func NewServer(svc services.BookingIntr) *bookingServer {
	return &bookingServer{svc: svc}
}

func (s *bookingServer) CreateBooking(ctx context.Context, req *pb.CreateBookingRequest) (*pb.BookingResponse, error) {
	if req.ClientName == "" || req.BarberId == "" || req.ServiceId == "" || req.ClientPhone == "" {
		return nil, status.Error(codes.InvalidArgument, "client_name, client_phone, barber_id and service_id are required")
	}
	if req.TimeStart == nil {
		return nil, status.Error(codes.InvalidArgument, "time_start is required")
	}

	b := &model.Booking{
		ClientName:  req.ClientName,
		ClientPhone: req.ClientPhone,
		BarberID:    req.BarberId,
		ServiceID:   req.ServiceId,
		TimeStart:   req.TimeStart.AsTime(),
	}

	created, err := s.svc.CreateBooking(ctx, b)
	if err != nil {
		if errors.Is(err, services.ErrActiveBookingExists) {
			return nil, status.Error(codes.AlreadyExists, "ACTIVE_BOOKING_EXISTS")
		}
		return nil, status.Errorf(codes.Internal, "create booking: %v", err)
	}
	return &pb.BookingResponse{Booking: toProto(created)}, nil
}

func (s *bookingServer) GetBooking(ctx context.Context, req *pb.BookingIdRequest) (*pb.BookingResponse, error) {
	if req.BookingId == "" {
		return nil, status.Error(codes.InvalidArgument, "booking_id is required")
	}
	b, err := s.svc.GetBooking(ctx, req.BookingId)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "booking not found")
		}
		return nil, status.Errorf(codes.Internal, "get booking: %v", err)
	}
	return &pb.BookingResponse{Booking: toProto(b)}, nil
}

func (s *bookingServer) UpdateBooking(ctx context.Context, req *pb.UpdateBookingRequest) (*pb.BookingResponse, error) {
	if req.BookingId == "" || req.BarberId == "" {
		return nil, status.Error(codes.InvalidArgument, "booking_id and barber_id are required")
	}
	if req.TimeStart == nil {
		return nil, status.Error(codes.InvalidArgument, "time_start is required")
	}

	updated, err := s.svc.UpdateBookingDetails(ctx, req.BookingId, req.BarberId, req.ServiceId, req.TimeStart.AsTime())
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "booking not found")
		}
		return nil, status.Errorf(codes.Internal, "update booking: %v", err)
	}
	return &pb.BookingResponse{Booking: toProto(updated)}, nil
}

func (s *bookingServer) UpdateBookingStatus(ctx context.Context, req *pb.UpdateBookingStatusRequest) (*pb.BookingResponse, error) {
	if req.BookingId == "" || req.BarberId == "" {
		return nil, status.Error(codes.InvalidArgument, "booking_id and barber_id are required")
	}

	newStatus, err := bookingStatusFromProto(req.Status)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	updated, err := s.svc.UpdateBookingStatus(ctx, req.BookingId, req.BarberId, newStatus)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "booking not found")
		}
		if errors.Is(err, services.ErrInvalidStatusTransition) {
			return nil, status.Error(codes.FailedPrecondition, "invalid status transition")
		}
		return nil, status.Errorf(codes.Internal, "update booking status: %v", err)
	}
	return &pb.BookingResponse{Booking: toProto(updated)}, nil
}

func (s *bookingServer) DeleteBooking(ctx context.Context, req *pb.BookingIdRequest) (*emptypb.Empty, error) {
	if req.BookingId == "" {
		return nil, status.Error(codes.InvalidArgument, "booking_id is required")
	}
	if err := s.svc.DeleteBooking(ctx, req.BookingId); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "booking not found")
		}
		return nil, status.Errorf(codes.Internal, "delete booking: %v", err)
	}
	return &emptypb.Empty{}, nil
}

func (s *bookingServer) GetSlots(ctx context.Context, req *pb.SlotsRequest) (*pb.SlotsResponse, error) {
	if req.BarberId == "" || req.Date == "" {
		return nil, status.Error(codes.InvalidArgument, "barber_id and date are required")
	}
	date, err := parseDate(req.Date)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid date format, expected YYYY-MM-DD")
	}
	result, err := s.svc.GetSlots(ctx, req.BarberId, date)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get slots: %v", err)
	}
	return toSlotsProto(result), nil
}

func (s *bookingServer) GetFreeSlots(ctx context.Context, req *pb.FreeSlotsRequest) (*pb.FreeSlotsResponse, error) {
	if req.BarberId == "" {
		return nil, status.Error(codes.InvalidArgument, "barber_id is required")
	}

	var date time.Time
	var err error
	if req.Date != "" {
		date, err = parseDate(req.Date)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid date format, expected YYYY-MM-DD")
		}
	} else {
		date = time.Now().UTC().Truncate(24 * time.Hour)
	}

	result, err := s.svc.GetFreeSlots(ctx, req.BarberId, date)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get free slots: %v", err)
	}
	return toFreeSlotsProto(result), nil
}

// helpers

func toProto(b *model.Booking) *pb.Booking {
	return &pb.Booking{
		BookingId:   b.ID,
		ClientName:  b.ClientName,
		ClientPhone: b.ClientPhone,
		BarberId:    b.BarberID,
		ServiceId:   b.ServiceID,
		ServiceName: b.ServiceName,
		TimeStart:   timestamppb.New(b.TimeStart),
		TimeEnd:     timestamppb.New(b.TimeEnd),
		Status:      bookingStatusToProto(b.Status),
	}
}

func slotToProto(s model.Slot) *pb.Slot {
	slot := &pb.Slot{
		Status:    pb.SlotStatus(s.Status),
		TimeStart: timestamppb.New(s.TimeStart),
		TimeEnd:   timestamppb.New(s.TimeEnd),
	}
	if s.Booking != nil {
		slot.Booking = &pb.SlotBooking{
			BookingId:   s.Booking.BookingID,
			ClientName:  s.Booking.ClientName,
			ClientPhone: s.Booking.ClientPhone,
			ServiceName: s.Booking.ServiceName,
		}
	}
	return slot
}

func toSlotsProto(r *model.SlotsResult) *pb.SlotsResponse {
	slots := make([]*pb.Slot, 0, len(r.Slots))
	for _, s := range r.Slots {
		slots = append(slots, slotToProto(s))
	}
	return &pb.SlotsResponse{
		BarberId: r.BarberID,
		Date:     r.Date,
		Slots:    slots,
	}
}

func toFreeSlotsProto(r *model.SlotsResult) *pb.FreeSlotsResponse {
	slots := make([]*pb.Slot, 0, len(r.Slots))
	for _, s := range r.Slots {
		slots = append(slots, slotToProto(s))
	}
	return &pb.FreeSlotsResponse{
		BarberId: r.BarberID,
		Date:     r.Date,
		Slots:    slots,
	}
}

func bookingStatusToProto(s string) pb.BookingStatus {
	switch s {
	case model.StatusPending:
		return pb.BookingStatus_BOOKING_STATUS_PENDING
	case model.StatusCompleted:
		return pb.BookingStatus_BOOKING_STATUS_COMPLETED
	case model.StatusCancelled:
		return pb.BookingStatus_BOOKING_STATUS_CANCELLED
	case model.StatusNoShow:
		return pb.BookingStatus_BOOKING_STATUS_NO_SHOW
	default:
		return pb.BookingStatus_BOOKING_STATUS_UNSPECIFIED
	}
}

func bookingStatusFromProto(s pb.BookingStatus) (string, error) {
	switch s {
	case pb.BookingStatus_BOOKING_STATUS_CANCELLED:
		return model.StatusCancelled, nil
	case pb.BookingStatus_BOOKING_STATUS_COMPLETED:
		return model.StatusCompleted, nil
	case pb.BookingStatus_BOOKING_STATUS_NO_SHOW:
		return model.StatusNoShow, nil
	default:
		return "", errors.New("status must be CANCELLED, COMPLETED or NO_SHOW")
	}
}

func parseDate(s string) (time.Time, error) {
	return time.Parse("2006-01-02", s)
}
