package grpc

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/RomanKovalev007/barber_crm/api/proto/analytics/v1"
	"github.com/RomanKovalev007/barber_crm/services/analytics/internal/apperr"
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
		return nil, toGRPCError(err)
	}
	return resp, nil
}

func toGRPCError(err error) error {
	var e *apperr.AppError
	if errors.As(err, &e) {
		return status.Error(e.GRPCCode(), e.Message)
	}
	return status.Error(codes.Internal, "internal error")
}
