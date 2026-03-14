package grpc

import (
	"context"
	"errors"
	"testing"

	pb "github.com/RomanKovalev007/barber_crm/api/proto/analytics/v1"
	"github.com/RomanKovalev007/barber_crm/services/analytics/internal/apperr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ─── mock ─────────────────────────────────────────────────────────────────────

type mockService struct{ mock.Mock }

func (m *mockService) GetBarberStats(ctx context.Context, req *pb.GetBarberStatsRequest) (*pb.BarberStatsResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*pb.BarberStatsResponse), args.Error(1)
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func newServer() (*Server, *mockService) {
	svc := &mockService{}
	return NewServer(svc), svc
}

func grpcCode(err error) codes.Code {
	s, ok := status.FromError(err)
	if !ok {
		return codes.Unknown
	}
	return s.Code()
}

// ─── tests ────────────────────────────────────────────────────────────────────

func TestGetBarberStats_Success(t *testing.T) {
	srv, svc := newServer()
	ctx := context.Background()
	req := &pb.GetBarberStatsRequest{BarberId: "b-1"}
	want := &pb.BarberStatsResponse{BarberId: "b-1", BookingsTotal: 10}

	svc.On("GetBarberStats", ctx, req).Return(want, nil)

	resp, err := srv.GetBarberStats(ctx, req)

	require.NoError(t, err)
	assert.Equal(t, int64(10), resp.BookingsTotal)
	svc.AssertExpectations(t)
}

func TestGetBarberStats_InvalidArgument(t *testing.T) {
	srv, svc := newServer()
	ctx := context.Background()
	req := &pb.GetBarberStatsRequest{}

	svc.On("GetBarberStats", ctx, req).Return(nil, apperr.InvalidArgument("barber_id is required"))

	_, err := srv.GetBarberStats(ctx, req)

	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, grpcCode(err))
}

func TestGetBarberStats_InternalAppError(t *testing.T) {
	srv, svc := newServer()
	ctx := context.Background()
	req := &pb.GetBarberStatsRequest{BarberId: "b-1"}

	svc.On("GetBarberStats", ctx, req).Return(nil, apperr.Internal("db failure"))

	_, err := srv.GetBarberStats(ctx, req)

	require.Error(t, err)
	assert.Equal(t, codes.Internal, grpcCode(err))
}

func TestGetBarberStats_UnknownError(t *testing.T) {
	srv, svc := newServer()
	ctx := context.Background()
	req := &pb.GetBarberStatsRequest{BarberId: "b-1"}

	svc.On("GetBarberStats", ctx, req).Return(nil, errors.New("unexpected"))

	_, err := srv.GetBarberStats(ctx, req)

	require.Error(t, err)
	assert.Equal(t, codes.Internal, grpcCode(err))
}
