package bookingrpc

import (
	"context"
	"errors"

	pb "github.com/RomanKovalev007/barber_crm/api/proto/booking/v1"
	"github.com/RomanKovalev007/barber_crm/services/booking/internal/model"
	"github.com/RomanKovalev007/barber_crm/services/booking/internal/repo"
	"github.com/RomanKovalev007/barber_crm/services/booking/internal/services"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type bookingServer struct {
	pb.UnimplementedBookingServiceServer
	svc services.BookingIntr
}

func NewServer(service services.BookingIntr) *bookingServer {
	return &bookingServer{svc: service}
}

func (s *bookingServer) CreateBooking(ctx context.Context, req *pb.CreateBookingRequest) (*pb.BookingResponse, error) {
	if req.ClientName == "" || req.BarberId == "" || req.ServId == "" {
		return nil, status.Error(codes.InvalidArgument, "client_name, barber_id and serv_id are required")
	}
	if req.Date == nil || req.TimeStart == nil || req.TimeEnd == nil {
		return nil, status.Error(codes.InvalidArgument, "date, time_start and time_end are required")
	}

	b := &model.Booking{
		ClientName: req.ClientName,
		BarberID:   req.BarberId,
		ServID:     req.ServId,
		Date:       req.Date.AsTime(),
		TimeStart:  req.TimeStart.AsTime(),
		TimeEnd:    req.TimeEnd.AsTime(),
	}

	created, err := s.svc.CreateBooking(ctx, b)
	if err != nil {
		if errors.Is(err, services.ErrActiveBookingExists) {
			return nil, status.Error(codes.AlreadyExists, "ACTIVE_BOOKINGS_EXISTS")
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

func (s *bookingServer) UpdateBooking(ctx context.Context, req *pb.Booking) (*pb.BookingResponse, error) {
	if req.BookingId == "" {
		return nil, status.Error(codes.InvalidArgument, "booking_id is required")
	}
	if req.ClientName == "" || req.BarberId == "" || req.ServId == "" {
		return nil, status.Error(codes.InvalidArgument, "client_name, barber_id and serv_id are required")
	}
	if req.Date == nil || req.TimeStart == nil || req.TimeEnd == nil {
		return nil, status.Error(codes.InvalidArgument, "date, time_start and time_end are required")
	}

	b := &model.Booking{
		ID:         req.BookingId,
		ClientName: req.ClientName,
		BarberID:   req.BarberId,
		ServID:     req.ServId,
		Date:       req.Date.AsTime(),
		TimeStart:  req.TimeStart.AsTime(),
		TimeEnd:    req.TimeEnd.AsTime(),
	}

	updated, err := s.svc.UpdateBooking(ctx, b)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "booking not found")
		}
		return nil, status.Errorf(codes.Internal, "update booking: %v", err)
	}
	return &pb.BookingResponse{Booking: toProto(updated)}, nil
}

func (s *bookingServer) DeleteBooking(ctx context.Context, req *pb.BookingIdRequest) (*pb.DeleteResponse, error) {
	if req.BookingId == "" {
		return nil, status.Error(codes.InvalidArgument, "booking_id is required")
	}
	result, err := s.svc.DeleteBooking(ctx, req.BookingId)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "booking not found")
		}
		return nil, status.Errorf(codes.Internal, "delete booking: %v", err)
	}
	return &pb.DeleteResponse{
		BookingId:  result.BookingID,
		ClientName: result.ClientName,
	}, nil
}

func (s *bookingServer) GetWorkDay(ctx context.Context, req *pb.SlotsRequest) (*pb.SlotsResponse, error) {
	if req.BarberId == "" {
		return nil, status.Error(codes.InvalidArgument, "barber_id is required")
	}
	if req.Date == nil {
		return nil, status.Error(codes.InvalidArgument, "date is required")
	}
	result, err := s.svc.GetWorkDay(ctx, req.BarberId, req.Date.AsTime())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get work day: %v", err)
	}
	return toSlotsProto(result), nil
}

func (s *bookingServer) GetFree(ctx context.Context, req *pb.SlotsRequest) (*pb.SlotsResponse, error) {
	if req.BarberId == "" {
		return nil, status.Error(codes.InvalidArgument, "barber_id is required")
	}
	if req.Date == nil {
		return nil, status.Error(codes.InvalidArgument, "date is required")
	}
	result, err := s.svc.GetFree(ctx, req.BarberId, req.Date.AsTime())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get free slots: %v", err)
	}
	return toSlotsProto(result), nil
}

func toProto(b *model.Booking) *pb.Booking {
	return &pb.Booking{
		BookingId:  b.ID,
		ClientName: b.ClientName,
		BarberId:   b.BarberID,
		ServId:     b.ServID,
		Date:       timestamppb.New(b.Date),
		TimeStart:  timestamppb.New(b.TimeStart),
		TimeEnd:    timestamppb.New(b.TimeEnd),
	}
}

func toSlotsProto(r *model.SlotsResult) *pb.SlotsResponse {
	slots := make([]*pb.Slot, 0, len(r.Slots))
	for _, s := range r.Slots {
		slots = append(slots, &pb.Slot{
			Status:    pb.SlotStatus(s.Status),
			TimeStart: timestamppb.New(s.TimeStart),
			TimeEnd:   timestamppb.New(s.TimeEnd),
		})
	}
	return &pb.SlotsResponse{
		BarberId: r.BarberID,
		Date:     timestamppb.New(r.Date),
		Slots:    slots,
		Priority: r.Priority,
	}
}
