package grpc

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	pb "github.com/RomanKovalev007/barber_crm/api/proto/staff/v1"
	"github.com/RomanKovalev007/barber_crm/services/staff/internal/apperr"
	"github.com/RomanKovalev007/barber_crm/services/staff/internal/model"
)

type staffService interface {
	Login(ctx context.Context, login string, password string) (*model.Barber, string, string, error)
	Logout(ctx context.Context, refreshToken string) error
	RefreshToken(ctx context.Context, refreshTokenStr string) (string, string, error)
	GetBarber(ctx context.Context, id string) (*model.Barber, error)
	ListBarbers(ctx context.Context, limit, offset int) ([]model.Barber, int, error)

	UpsertSchedule(ctx context.Context, barberID string, day *model.ScheduleDay) (*model.ScheduleDay, error)
	UpsertWeekSchedule(ctx context.Context, barberID string, days []*model.ScheduleDay) ([]*model.ScheduleDay, error)
	DeleteSchedule(ctx context.Context, barberID, date string) error
	GetSchedule(ctx context.Context, barberID string, week string) ([]model.ScheduleDay, error)

	GetService(ctx context.Context, id, barberID string) (*model.Service, error)
	ListServices(ctx context.Context, barberID string, includeInactive bool, limit, offset int) ([]model.Service, int, error)
	CreateService(ctx context.Context, svc *model.Service) error
	DeleteService(ctx context.Context, id string, barberID string) error
	UpdateService(ctx context.Context, svc *model.Service) error
}

type Server struct {
	pb.UnimplementedStaffServiceServer
	svc staffService
}

func NewServer(svc staffService) *Server {
	return &Server{svc: svc}
}

func toGRPCError(err error) error {
	var e *apperr.AppError
	if errors.As(err, &e) {
		return status.Error(e.GRPCCode(), e.Message)
	}
	return status.Error(codes.Internal, "internal error")
}

// auth

func (s *Server) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	barber, accessToken, refreshToken, err := s.svc.Login(ctx, req.Login, req.Password)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &pb.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    3600,
		Barber:       barberToProto(barber),
	}, nil
}

func (s *Server) Logout(ctx context.Context, req *pb.LogoutRequest) (*emptypb.Empty, error) {
	if err := s.svc.Logout(ctx, req.RefreshToken); err != nil {
		return nil, toGRPCError(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) RefreshToken(ctx context.Context, req *pb.RefreshTokenRequest) (*pb.RefreshTokenResponse, error) {
	access, refresh, err := s.svc.RefreshToken(ctx, req.RefreshToken)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &pb.RefreshTokenResponse{AccessToken: access, RefreshToken: refresh, ExpiresIn: 3600}, nil
}

// barbers

func (s *Server) GetBarber(ctx context.Context, req *pb.GetBarberRequest) (*pb.BarberResponse, error) {
	barber, err := s.svc.GetBarber(ctx, req.BarberId)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return barberToProto(barber), nil
}

func (s *Server) ListBarbers(ctx context.Context, req *pb.ListBarbersRequest) (*pb.ListBarbersResponse, error) {
	barbers, total, err := s.svc.ListBarbers(ctx, int(req.Limit), int(req.Offset))
	if err != nil {
		return nil, toGRPCError(err)
	}
	var pbBarbers []*pb.BarberResponse
	for _, b := range barbers {
		pbBarbers = append(pbBarbers, barberToProto(&b))
	}
	return &pb.ListBarbersResponse{Barbers: pbBarbers, Total: int32(total)}, nil
}

// schedule

func (s *Server) GetSchedule(ctx context.Context, req *pb.GetScheduleRequest) (*pb.GetScheduleResponse, error) {
	days, err := s.svc.GetSchedule(ctx, req.BarberId, req.Week)
	if err != nil {
		return nil, toGRPCError(err)
	}
	var pbDays []*pb.ScheduleDay
	for _, d := range days {
		pbDays = append(pbDays, scheduleToProto(&d))
	}
	return &pb.GetScheduleResponse{Week: req.Week, Days: pbDays}, nil
}

func (s *Server) UpsertSchedule(ctx context.Context, req *pb.UpsertScheduleRequest) (*pb.ScheduleDay, error) {
	day := &model.ScheduleDay{
		BarberID:  req.BarberId,
		Date:      req.Date,
		StartTime: req.StartTime,
		EndTime:   req.EndTime,
		PartOfDay: partOfDayFromProto(req.PartOfDay),
	}
	result, err := s.svc.UpsertSchedule(ctx, req.BarberId, day)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return scheduleToProto(result), nil
}

func (s *Server) UpsertWeekSchedule(ctx context.Context, req *pb.UpsertWeekScheduleRequest) (*pb.UpsertWeekScheduleResponse, error) {
	days := make([]*model.ScheduleDay, 0, len(req.Days))
	for _, d := range req.Days {
		days = append(days, &model.ScheduleDay{
			BarberID:  req.BarberId,
			Date:      d.Date,
			StartTime: d.StartTime,
			EndTime:   d.EndTime,
			PartOfDay: partOfDayFromProto(d.PartOfDay),
		})
	}
	result, err := s.svc.UpsertWeekSchedule(ctx, req.BarberId, days)
	if err != nil {
		return nil, toGRPCError(err)
	}
	pbDays := make([]*pb.ScheduleDay, 0, len(result))
	for _, d := range result {
		pbDays = append(pbDays, scheduleToProto(d))
	}
	return &pb.UpsertWeekScheduleResponse{Days: pbDays}, nil
}

func (s *Server) DeleteSchedule(ctx context.Context, req *pb.DeleteScheduleRequest) (*emptypb.Empty, error) {
	if err := s.svc.DeleteSchedule(ctx, req.BarberId, req.Date); err != nil {
		return nil, toGRPCError(err)
	}
	return &emptypb.Empty{}, nil
}

// services

func (s *Server) GetService(ctx context.Context, req *pb.GetServiceRequest) (*pb.ServiceResponse, error) {
	if req.ServiceId == "" || req.BarberId == "" {
		return nil, status.Error(codes.InvalidArgument, "service_id and barber_id are required")
	}
	svc, err := s.svc.GetService(ctx, req.ServiceId, req.BarberId)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return serviceToProto(svc), nil
}

func (s *Server) ListServices(ctx context.Context, req *pb.ListServicesRequest) (*pb.ListServicesResponse, error) {
	services, total, err := s.svc.ListServices(ctx, req.BarberId, req.IncludeInactive, int(req.Limit), int(req.Offset))
	if err != nil {
		return nil, toGRPCError(err)
	}
	var pbServices []*pb.ServiceResponse
	for _, svc := range services {
		pbServices = append(pbServices, serviceToProto(&svc))
	}
	return &pb.ListServicesResponse{Services: pbServices, Total: int32(total)}, nil
}

func (s *Server) CreateService(ctx context.Context, req *pb.CreateServiceRequest) (*pb.ServiceResponse, error) {
	svc := &model.Service{
		BarberID:        req.BarberId,
		Name:            req.Name,
		Price:           int(req.Price),
		DurationMinutes: int(req.DurationMinutes),
		IsActive:        true,
	}
	if err := s.svc.CreateService(ctx, svc); err != nil {
		return nil, toGRPCError(err)
	}
	return serviceToProto(svc), nil
}

func (s *Server) UpdateService(ctx context.Context, req *pb.UpdateServiceRequest) (*pb.ServiceResponse, error) {
	svc := &model.Service{
		ID:              req.ServiceId,
		BarberID:        req.BarberId,
		Name:            req.Name,
		Price:           int(req.Price),
		DurationMinutes: int(req.DurationMinutes),
		IsActive:        req.IsActive,
	}
	if err := s.svc.UpdateService(ctx, svc); err != nil {
		return nil, toGRPCError(err)
	}
	return serviceToProto(svc), nil
}

func (s *Server) DeleteService(ctx context.Context, req *pb.DeleteServiceRequest) (*emptypb.Empty, error) {
	if err := s.svc.DeleteService(ctx, req.ServiceId, req.BarberId); err != nil {
		return nil, toGRPCError(err)
	}
	return &emptypb.Empty{}, nil
}

// helpers

func barberToProto(b *model.Barber) *pb.BarberResponse {
	var services []*pb.ServiceResponse
	for i := range b.Services {
		services = append(services, serviceToProto(&b.Services[i]))
	}
	return &pb.BarberResponse{
		BarberId: b.ID,
		Name:     b.Name,
		Services: services,
	}
}

func serviceToProto(s *model.Service) *pb.ServiceResponse {
	return &pb.ServiceResponse{
		ServiceId:       s.ID,
		Name:            s.Name,
		Price:           int32(s.Price),
		DurationMinutes: int32(s.DurationMinutes),
		IsActive:        s.IsActive,
	}
}

func partOfDayFromProto(p pb.PartOfDay) model.PartOfDay {
	if p == pb.PartOfDay_PART_OF_DAY_PM {
		return model.PartOfDayPM
	}
	return model.PartOfDayAM
}

func partOfDayToProto(p model.PartOfDay) pb.PartOfDay {
	if p == model.PartOfDayPM {
		return pb.PartOfDay_PART_OF_DAY_PM
	}
	return pb.PartOfDay_PART_OF_DAY_AM
}

func scheduleToProto(s *model.ScheduleDay) *pb.ScheduleDay {
	return &pb.ScheduleDay{
		ScheduleDayId: s.ID,
		BarberId:      s.BarberID,
		Date:          s.Date,
		StartTime:     s.StartTime,
		EndTime:       s.EndTime,
		PartOfDay:     partOfDayToProto(s.PartOfDay),
	}
}
