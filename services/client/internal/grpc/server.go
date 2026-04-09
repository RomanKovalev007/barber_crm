package grpc

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/RomanKovalev007/barber_crm/api/proto/client/v1"
	"github.com/RomanKovalev007/barber_crm/services/client/internal/apperr"
	"github.com/RomanKovalev007/barber_crm/services/client/internal/model"
)

type clientService interface {
	ListClients(ctx context.Context, barberID, search string) ([]model.Client, error)
	GetClient(ctx context.Context, id, barberID string) (*model.Client, error)
	GetClientByPhone(ctx context.Context, barberID, phone string) (*model.Client, error)
	UpdateClient(ctx context.Context, id, barberID, name, notes string) (*model.Client, error)
	DeleteClient(ctx context.Context, id, barberID string) error
}

type Server struct {
	pb.UnimplementedClientServiceServer
	svc clientService
}

func NewServer(svc clientService) *Server {
	return &Server{svc: svc}
}

func (s *Server) ListClients(ctx context.Context, req *pb.ListClientsRequest) (*pb.ListClientsResponse, error) {
	if req.BarberId == "" {
		return nil, status.Error(codes.InvalidArgument, "barber_id is required")
	}
	clients, err := s.svc.ListClients(ctx, req.BarberId, req.Search)
	if err != nil {
		return nil, toGRPCError(err)
	}
	result := make([]*pb.Client, 0, len(clients))
	for _, c := range clients {
		result = append(result, toProto(&c))
	}
	return &pb.ListClientsResponse{Clients: result}, nil
}

func (s *Server) GetClient(ctx context.Context, req *pb.GetClientRequest) (*pb.ClientResponse, error) {
	if req.ClientId == "" || req.BarberId == "" {
		return nil, status.Error(codes.InvalidArgument, "client_id and barber_id are required")
	}
	c, err := s.svc.GetClient(ctx, req.ClientId, req.BarberId)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &pb.ClientResponse{Client: toProto(c)}, nil
}

func (s *Server) GetClientByPhone(ctx context.Context, req *pb.GetClientByPhoneRequest) (*pb.ClientResponse, error) {
	if req.BarberId == "" || req.Phone == "" {
		return nil, status.Error(codes.InvalidArgument, "barber_id and phone are required")
	}
	c, err := s.svc.GetClientByPhone(ctx, req.BarberId, req.Phone)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &pb.ClientResponse{Client: toProto(c)}, nil
}

func (s *Server) UpdateClient(ctx context.Context, req *pb.UpdateClientRequest) (*pb.ClientResponse, error) {
	if req.ClientId == "" || req.BarberId == "" {
		return nil, status.Error(codes.InvalidArgument, "client_id and barber_id are required")
	}
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	c, err := s.svc.UpdateClient(ctx, req.ClientId, req.BarberId, req.Name, req.Notes)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &pb.ClientResponse{Client: toProto(c)}, nil
}

func (s *Server) DeleteClient(ctx context.Context, req *pb.GetClientRequest) (*emptypb.Empty, error) {
	if req.ClientId == "" || req.BarberId == "" {
		return nil, status.Error(codes.InvalidArgument, "client_id and barber_id are required")
	}
	if err := s.svc.DeleteClient(ctx, req.ClientId, req.BarberId); err != nil {
		return nil, toGRPCError(err)
	}
	return &emptypb.Empty{}, nil
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func toProto(c *model.Client) *pb.Client {
	p := &pb.Client{
		ClientId:    c.ID,
		BarberId:    c.BarberID,
		Name:        c.Name,
		Phone:       c.Phone,
		Notes:       c.Notes,
		VisitsCount: c.VisitsCount,
		CreatedAt:   timestamppb.New(c.CreatedAt),
	}
	if c.LastVisit != nil {
		p.LastVisit = timestamppb.New(*c.LastVisit)
	}
	return p
}

func toGRPCError(err error) error {
	var e *apperr.AppError
	if errors.As(err, &e) {
		return status.Error(e.GRPCCode(), e.Message)
	}
	return status.Error(codes.Internal, "internal error")
}
