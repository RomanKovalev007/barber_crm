package bookingrpc

import (
	"context"
	"errors"
	"regexp"
	"time"

	pb "github.com/RomanKovalev007/barber_crm/api/proto/booking/v1"
	"github.com/RomanKovalev007/barber_crm/services/booking/internal/apperr"
	"github.com/RomanKovalev007/barber_crm/services/booking/internal/model"
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

func toGRPCError(err error) error {
	var e *apperr.AppError
	if errors.As(err, &e) {
		return status.Error(e.GRPCCode(), e.Message)
	}
	return status.Error(codes.Internal, "internal error")
}

func (s *bookingServer) CreateBooking(ctx context.Context, req *pb.CreateBookingRequest) (*pb.BookingResponse, error) {
	if req.ClientName == "" || req.BarberId == "" || req.ServiceId == "" || req.ClientPhone == "" {
		return nil, status.Error(codes.InvalidArgument, "client_name, client_phone, barber_id and service_id are required")
	}
	if !isValidPhone(req.ClientPhone) {
		return nil, status.Error(codes.InvalidArgument, "client_phone must be 10–15 digits, optionally starting with +")
	}
	if req.TimeStart == nil {
		return nil, status.Error(codes.InvalidArgument, "time_start is required")
	}
	if !req.TimeStart.AsTime().After(time.Now()) {
		return nil, status.Error(codes.InvalidArgument, "time_start must be in the future")
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
		return nil, toGRPCError(err)
	}
	return &pb.BookingResponse{Booking: toProto(created)}, nil
}

func (s *bookingServer) GetBooking(ctx context.Context, req *pb.BookingIdRequest) (*pb.BookingResponse, error) {
	if req.BookingId == "" || req.BarberId == "" {
		return nil, status.Error(codes.InvalidArgument, "booking_id and barber_id are required")
	}
	b, err := s.svc.GetBooking(ctx, req.BookingId, req.BarberId)
	if err != nil {
		return nil, toGRPCError(err)
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
		return nil, toGRPCError(err)
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
		return nil, toGRPCError(err)
	}
	return &pb.BookingResponse{Booking: toProto(updated)}, nil
}

func (s *bookingServer) DeleteBooking(ctx context.Context, req *pb.BookingIdRequest) (*emptypb.Empty, error) {
	if req.BookingId == "" || req.BarberId == "" {
		return nil, status.Error(codes.InvalidArgument, "booking_id and barber_id are required")
	}
	if err := s.svc.DeleteBooking(ctx, req.BookingId, req.BarberId); err != nil {
		return nil, toGRPCError(err)
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
		return nil, toGRPCError(err)
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

	result, err := s.svc.GetFreeSlots(ctx, req.BarberId, req.ServiceId, date)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return toFreeSlotsProto(result), nil
}

func (s *bookingServer) GetBarberSettings(ctx context.Context, req *pb.BarberSettingsRequest) (*pb.BarberSettingsResponse, error) {
	if req.BarberId == "" {
		return nil, status.Error(codes.InvalidArgument, "barber_id is required")
	}
	settings, err := s.svc.GetBarberSettings(ctx, req.BarberId)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return settingsToProto(settings), nil
}

func (s *bookingServer) SetCompactSlots(ctx context.Context, req *pb.SetCompactSlotsRequest) (*pb.BarberSettingsResponse, error) {
	if req.BarberId == "" {
		return nil, status.Error(codes.InvalidArgument, "barber_id is required")
	}
	settings, err := s.svc.SetCompactSlots(ctx, req.BarberId, req.Enabled)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return settingsToProto(settings), nil
}

func (s *bookingServer) SetClientSlotStep(ctx context.Context, req *pb.SetClientSlotStepRequest) (*pb.BarberSettingsResponse, error) {
	if req.BarberId == "" {
		return nil, status.Error(codes.InvalidArgument, "barber_id is required")
	}
	if req.StepMinutes != 15 && req.StepMinutes != 30 && req.StepMinutes != 60 {
		return nil, status.Error(codes.InvalidArgument, "step_minutes must be 15, 30 or 60")
	}
	settings, err := s.svc.SetClientSlotStep(ctx, req.BarberId, req.StepMinutes)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return settingsToProto(settings), nil
}

func settingsToProto(s *model.BarberSettings) *pb.BarberSettingsResponse {
	return &pb.BarberSettingsResponse{
		BarberId:              s.BarberID,
		CompactSlotsEnabled:   s.CompactSlotsEnabled,
		ClientSlotStepMinutes: s.ClientSlotStepMinutes,
	}
}

func (s *bookingServer) GetClientBookings(ctx context.Context, req *pb.GetClientBookingsRequest) (*pb.GetClientBookingsResponse, error) {
	if req.BarberId == "" {
		return nil, status.Error(codes.InvalidArgument, "barber_id is required")
	}
	if !isValidPhone(req.ClientPhone) {
		return nil, status.Error(codes.InvalidArgument, "client_phone is required and must be valid")
	}

	limit := int(req.Limit)
	offset := int(req.Offset)

	bookings, total, err := s.svc.GetClientBookings(ctx, req.BarberId, req.ClientPhone, limit, offset)
	if err != nil {
		return nil, toGRPCError(err)
	}

	pbBookings := make([]*pb.Booking, 0, len(bookings))
	for i := range bookings {
		pbBookings = append(pbBookings, toProto(&bookings[i]))
	}
	return &pb.GetClientBookingsResponse{
		Bookings: pbBookings,
		Total:    int32(total),
	}, nil
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
		Price:       b.Price,
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
			Status:      bookingStatusToProto(s.Booking.Status),
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
	case pb.BookingStatus_BOOKING_STATUS_PENDING:
		return model.StatusPending, nil
	case pb.BookingStatus_BOOKING_STATUS_CANCELLED:
		return model.StatusCancelled, nil
	case pb.BookingStatus_BOOKING_STATUS_COMPLETED:
		return model.StatusCompleted, nil
	case pb.BookingStatus_BOOKING_STATUS_NO_SHOW:
		return model.StatusNoShow, nil
	default:
		return "", errors.New("status must be PENDING, CANCELLED, COMPLETED or NO_SHOW")
	}
}

func parseDate(s string) (time.Time, error) {
	return time.Parse("2006-01-02", s)
}

var phoneRe = regexp.MustCompile(`^\+?[0-9]{10,15}$`)

func isValidPhone(phone string) bool {
	return phoneRe.MatchString(phone)
}
