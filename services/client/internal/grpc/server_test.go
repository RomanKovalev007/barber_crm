package grpc

import (
	"context"
	"testing"
	"time"

	pb "github.com/RomanKovalev007/barber_crm/api/proto/client/v1"
	"github.com/RomanKovalev007/barber_crm/services/client/internal/apperr"
	"github.com/RomanKovalev007/barber_crm/services/client/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func ptrTime(t time.Time) *time.Time { return &t }

// ─── mock ─────────────────────────────────────────────────────────────────────

type mockService struct{ mock.Mock }

func (m *mockService) ListClients(ctx context.Context, barberID, search string) ([]model.Client, error) {
	args := m.Called(ctx, barberID, search)
	return args.Get(0).([]model.Client), args.Error(1)
}
func (m *mockService) GetClient(ctx context.Context, id, barberID string) (*model.Client, error) {
	args := m.Called(ctx, id, barberID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Client), args.Error(1)
}
func (m *mockService) GetClientByPhone(ctx context.Context, barberID, phone string) (*model.Client, error) {
	args := m.Called(ctx, barberID, phone)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Client), args.Error(1)
}
func (m *mockService) UpdateClient(ctx context.Context, id, barberID, name, notes string) (*model.Client, error) {
	args := m.Called(ctx, id, barberID, name, notes)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Client), args.Error(1)
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

var testClient = &model.Client{
	ID:          "cl-1",
	BarberID:    "b-1",
	Phone:       "+79001234567",
	Name:        "Ivan",
	Notes:       "VIP",
	VisitsCount: 3,
	LastVisit:   ptrTime(time.Date(2026, 3, 16, 10, 0, 0, 0, time.UTC)),
	CreatedAt:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
}

// ─── ListClients ──────────────────────────────────────────────────────────────

func TestListClients_Success(t *testing.T) {
	srv, svc := newServer()
	ctx := context.Background()
	svc.On("ListClients", ctx, "b-1", "").Return([]model.Client{*testClient}, nil)

	resp, err := srv.ListClients(ctx, &pb.ListClientsRequest{BarberId: "b-1"})

	require.NoError(t, err)
	require.Len(t, resp.Clients, 1)
	assert.Equal(t, "cl-1", resp.Clients[0].ClientId)
	assert.Equal(t, "Ivan", resp.Clients[0].Name)
	svc.AssertExpectations(t)
}

func TestListClients_WithSearch(t *testing.T) {
	srv, svc := newServer()
	ctx := context.Background()
	svc.On("ListClients", ctx, "b-1", "Ivan").Return([]model.Client{*testClient}, nil)

	resp, err := srv.ListClients(ctx, &pb.ListClientsRequest{BarberId: "b-1", Search: "Ivan"})

	require.NoError(t, err)
	assert.Len(t, resp.Clients, 1)
}

func TestListClients_MissingBarberID(t *testing.T) {
	srv, _ := newServer()

	_, err := srv.ListClients(context.Background(), &pb.ListClientsRequest{})

	assert.Equal(t, codes.InvalidArgument, grpcCode(err))
}

func TestListClients_ServiceError(t *testing.T) {
	srv, svc := newServer()
	ctx := context.Background()
	svc.On("ListClients", ctx, "b-1", "").Return([]model.Client(nil), apperr.Internal("db failure"))

	_, err := srv.ListClients(ctx, &pb.ListClientsRequest{BarberId: "b-1"})

	assert.Equal(t, codes.Internal, grpcCode(err))
}

// ─── GetClient ────────────────────────────────────────────────────────────────

func TestGetClient_Success(t *testing.T) {
	srv, svc := newServer()
	ctx := context.Background()
	svc.On("GetClient", ctx, "cl-1", "b-1").Return(testClient, nil)

	resp, err := srv.GetClient(ctx, &pb.GetClientRequest{ClientId: "cl-1", BarberId: "b-1"})

	require.NoError(t, err)
	assert.Equal(t, "cl-1", resp.Client.ClientId)
	assert.Equal(t, int32(3), resp.Client.VisitsCount)
	assert.NotNil(t, resp.Client.LastVisit)
	svc.AssertExpectations(t)
}

func TestGetClient_MissingID(t *testing.T) {
	srv, _ := newServer()

	_, err := srv.GetClient(context.Background(), &pb.GetClientRequest{BarberId: "b-1"})

	assert.Equal(t, codes.InvalidArgument, grpcCode(err))
}

func TestGetClient_MissingBarberID(t *testing.T) {
	srv, _ := newServer()

	_, err := srv.GetClient(context.Background(), &pb.GetClientRequest{ClientId: "cl-1"})

	assert.Equal(t, codes.InvalidArgument, grpcCode(err))
}

func TestGetClient_NotFound(t *testing.T) {
	srv, svc := newServer()
	ctx := context.Background()
	svc.On("GetClient", ctx, "cl-x", "b-1").Return(nil, apperr.NotFound("client not found"))

	_, err := srv.GetClient(ctx, &pb.GetClientRequest{ClientId: "cl-x", BarberId: "b-1"})

	assert.Equal(t, codes.NotFound, grpcCode(err))
}

func TestGetClient_InternalError(t *testing.T) {
	srv, svc := newServer()
	ctx := context.Background()
	svc.On("GetClient", ctx, "cl-1", "b-1").Return(nil, apperr.Internal("db failure"))

	_, err := srv.GetClient(ctx, &pb.GetClientRequest{ClientId: "cl-1", BarberId: "b-1"})

	assert.Equal(t, codes.Internal, grpcCode(err))
}

func TestGetClient_LastVisitZero(t *testing.T) {
	srv, svc := newServer()
	ctx := context.Background()
	clientNoVisit := &model.Client{ID: "cl-2", BarberID: "b-1", Name: "New", CreatedAt: time.Now()}
	svc.On("GetClient", ctx, "cl-2", "b-1").Return(clientNoVisit, nil)

	resp, err := srv.GetClient(ctx, &pb.GetClientRequest{ClientId: "cl-2", BarberId: "b-1"})

	require.NoError(t, err)
	assert.Nil(t, resp.Client.LastVisit)
}

// ─── GetClientByPhone ─────────────────────────────────────────────────────────

func TestGetClientByPhone_Success(t *testing.T) {
	srv, svc := newServer()
	ctx := context.Background()
	svc.On("GetClientByPhone", ctx, "b-1", "+7").Return(testClient, nil)

	resp, err := srv.GetClientByPhone(ctx, &pb.GetClientByPhoneRequest{BarberId: "b-1", Phone: "+7"})

	require.NoError(t, err)
	assert.Equal(t, "cl-1", resp.Client.ClientId)
	svc.AssertExpectations(t)
}

func TestGetClientByPhone_MissingFields(t *testing.T) {
	srv, _ := newServer()

	cases := []*pb.GetClientByPhoneRequest{
		{BarberId: "", Phone: "+7"},
		{BarberId: "b-1", Phone: ""},
		{BarberId: "", Phone: ""},
	}
	for _, req := range cases {
		_, err := srv.GetClientByPhone(context.Background(), req)
		assert.Equal(t, codes.InvalidArgument, grpcCode(err))
	}
}

func TestGetClientByPhone_NotFound(t *testing.T) {
	srv, svc := newServer()
	ctx := context.Background()
	svc.On("GetClientByPhone", ctx, "b-1", "+7").Return(nil, apperr.NotFound("client not found"))

	_, err := srv.GetClientByPhone(ctx, &pb.GetClientByPhoneRequest{BarberId: "b-1", Phone: "+7"})

	assert.Equal(t, codes.NotFound, grpcCode(err))
}

// ─── UpdateClient ─────────────────────────────────────────────────────────────

func TestUpdateClient_Success(t *testing.T) {
	srv, svc := newServer()
	ctx := context.Background()
	updated := &model.Client{ID: "cl-1", Name: "Petr", Notes: "notes", CreatedAt: time.Now()}
	svc.On("UpdateClient", ctx, "cl-1", "b-1", "Petr", "notes").Return(updated, nil)

	resp, err := srv.UpdateClient(ctx, &pb.UpdateClientRequest{ClientId: "cl-1", BarberId: "b-1", Name: "Petr", Notes: "notes"})

	require.NoError(t, err)
	assert.Equal(t, "Petr", resp.Client.Name)
	svc.AssertExpectations(t)
}

func TestUpdateClient_MissingClientID(t *testing.T) {
	srv, _ := newServer()

	_, err := srv.UpdateClient(context.Background(), &pb.UpdateClientRequest{BarberId: "b-1", Name: "Ivan"})

	assert.Equal(t, codes.InvalidArgument, grpcCode(err))
}

func TestUpdateClient_MissingBarberID(t *testing.T) {
	srv, _ := newServer()

	_, err := srv.UpdateClient(context.Background(), &pb.UpdateClientRequest{ClientId: "cl-1", Name: "Ivan"})

	assert.Equal(t, codes.InvalidArgument, grpcCode(err))
}

func TestUpdateClient_MissingName(t *testing.T) {
	srv, _ := newServer()

	_, err := srv.UpdateClient(context.Background(), &pb.UpdateClientRequest{ClientId: "cl-1", BarberId: "b-1"})

	assert.Equal(t, codes.InvalidArgument, grpcCode(err))
}

func TestUpdateClient_NotFound(t *testing.T) {
	srv, svc := newServer()
	ctx := context.Background()
	svc.On("UpdateClient", ctx, "cl-x", "b-1", "Ivan", "").Return(nil, apperr.NotFound("client not found"))

	_, err := srv.UpdateClient(ctx, &pb.UpdateClientRequest{ClientId: "cl-x", BarberId: "b-1", Name: "Ivan"})

	assert.Equal(t, codes.NotFound, grpcCode(err))
}

func TestUpdateClient_InternalError(t *testing.T) {
	srv, svc := newServer()
	ctx := context.Background()
	svc.On("UpdateClient", ctx, "cl-1", "b-1", "Ivan", "").Return(nil, apperr.Internal("db failure"))

	_, err := srv.UpdateClient(ctx, &pb.UpdateClientRequest{ClientId: "cl-1", BarberId: "b-1", Name: "Ivan"})

	assert.Equal(t, codes.Internal, grpcCode(err))
}
