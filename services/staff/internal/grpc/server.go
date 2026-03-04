package grpc

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	pb "github.com/RomanKovalev007/barber_crm/api/proto/staff/v1"
	"github.com/RomanKovalev007/barber_crm/services/staff/internal/model"
)

type staffService interface {
	Login(ctx context.Context, login string, password string) (*model.Barber, string, string, error)
	Logout(ctx context.Context, refreshToken string) error
	RefreshToken(ctx context.Context, refreshTokenStr string) (string, string, error)
	GetBarber(ctx context.Context, id string) (*model.Barber, error)
	ListBarbers(ctx context.Context) ([]model.Barber, error)

	AddSchedule(ctx context.Context, barberID string, day *model.ScheduleDay) (*model.ScheduleDay, error)
	GetSchedule(ctx context.Context, barberID string, week string) ([]model.ScheduleDay, error)

	ListServices(ctx context.Context, barberID string, includeInactive bool) ([]model.Service, error)
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

// barbers

func (s *Server) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	barber, accessToken, refreshToken, err := s.svc.Login(ctx, req.Login, req.Password)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
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
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) RefreshToken(ctx context.Context, req *pb.RefreshTokenRequest) (*pb.RefreshTokenResponse, error) {
	access, refresh, err := s.svc.RefreshToken(ctx, req.RefreshToken)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}
	return &pb.RefreshTokenResponse{AccessToken: access, RefreshToken: refresh, ExpiresIn: 3600}, nil
}

func (s *Server) GetBarber(ctx context.Context, req *pb.GetBarberRequest) (*pb.BarberResponse, error) {
	barber, err := s.svc.GetBarber(ctx, req.Id)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}
	return barberToProto(barber), nil
}

func (s *Server) ListBarbers(ctx context.Context, _ *pb.ListBarbersRequest) (*pb.ListBarbersResponse, error) {
	barbers, err := s.svc.ListBarbers(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	var pbBarbers []*pb.BarberResponse
	for _,b := range barbers {
		pbBarbers = append(pbBarbers, barberToProto(&b))
	}
	return &pb.ListBarbersResponse{Barbers: pbBarbers}, nil
}

// schedule

func (s *Server) GetSchedule(ctx context.Context, req *pb.GetScheduleRequest) (*pb.GetScheduleResponse, error) {
	days, err := s.svc.GetSchedule(ctx, req.BarberId, req.Week)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	var pbDays []*pb.ScheduleDay
	for _, d := range days {
		pbDays = append(pbDays, scheduleToProto(&d))
	}
	return &pb.GetScheduleResponse{Week: req.Week, Days: pbDays}, nil
}

func (s *Server) AddSchedule(ctx context.Context, req *pb.AddScheduleRequest) (*pb.ScheduleDay, error) {
	day := &model.ScheduleDay{
		BarberID: req.BarberId,
		Date: req.Date, 
		StartTime: req.StartTime, 
		EndTime: req.EndTime,
	}

	result, err := s.svc.AddSchedule(ctx, req.BarberId, day)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return scheduleToProto(result), nil
}

// services

func (s *Server) ListServices(ctx context.Context, req *pb.ListServicesRequest) (*pb.ListServicesResponse, error) {
	services, err := s.svc.ListServices(ctx, req.BarberId, req.IncludeInactive)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	var pbServices []*pb.ServiceResponse
	for _, svc := range services {
		pbServices = append(pbServices, serviceToProto(&svc))
	}
	return &pb.ListServicesResponse{Services: pbServices}, nil
}

func (s *Server) CreateService(ctx context.Context, req *pb.CreateServiceRequest) (*pb.ServiceResponse, error) {
	svc := &model.Service{
		BarberID: req.BarberId,
		Name: req.Name,
		Price: int(req.Price),
		IsActive: true,
	}
	if err := s.svc.CreateService(ctx, svc); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return serviceToProto(svc), nil
}

func (s *Server) UpdateService(ctx context.Context, req *pb.UpdateServiceRequest) (*pb.ServiceResponse, error) {
	svc := &model.Service{
		ID: req.Id,
		BarberID: req.BarberId, 
		Name: req.Name, 
		Price: int(req.Price),
		IsActive: req.IsActive,
	}
	if err := s.svc.UpdateService(ctx, svc); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return serviceToProto(svc), nil
}

func (s *Server) DeleteService(ctx context.Context, req *pb.DeleteServiceRequest) (*emptypb.Empty, error) {
	if err := s.svc.DeleteService(ctx, req.Id, req.BarberId); err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}
	return &emptypb.Empty{}, nil
}

// help functions

func barberToProto(b *model.Barber) *pb.BarberResponse {
	var services []*pb.ServiceResponse
	for i := range b.Services {
		services = append(services, serviceToProto(&b.Services[i]))
	}
	return &pb.BarberResponse{
		Id: b.ID, 
		Name: b.Name,
		Services: services,
	}
}

func serviceToProto(s *model.Service) *pb.ServiceResponse {
	return &pb.ServiceResponse{
		Id: s.ID,
		Name: s.Name, 
		Price: int32(s.Price),
		IsActive: s.IsActive,
	}
}

func scheduleToProto(s *model.ScheduleDay) *pb.ScheduleDay {
	return &pb.ScheduleDay{
		Id: s.ID,
		BarberId: s.BarberID,
		Date: s.Date,
		StartTime: s.StartTime,
		EndTime: s.EndTime,
	}
}
