package grpc

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/RomanKovalev007/barber_crm/api/proto/analytics/v1"
)

type analyticsService interface {
	GetBarberStats(ctx context.Context, req *pb.GetBarberStatsRequest) (*pb.BarberStatsResponse, error)
}

type Server struct {
	pb.UnimplementedAnalyticsServiceServer
	svc analyticsService
}

func NewServer(svc analyticsService) *Server {
	return &Server{svc: svc}
}

func (s *Server) GetBarberStats(ctx context.Context, req *pb.GetBarberStatsRequest) (*pb.BarberStatsResponse, error) {
	resp, err := s.svc.GetBarberStats(ctx, req)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return resp, nil
}
