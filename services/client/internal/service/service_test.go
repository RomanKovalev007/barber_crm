package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/RomanKovalev007/barber_crm/services/client/internal/apperr"
	"github.com/RomanKovalev007/barber_crm/services/client/internal/model"
	"github.com/RomanKovalev007/barber_crm/services/client/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// ─── mock ─────────────────────────────────────────────────────────────────────

type MockRepo struct{ mock.Mock }

func (m *MockRepo) UpsertByBooking(ctx context.Context, barberID, phone, name, bookingID string, lastVisit time.Time) error {
	return m.Called(ctx, barberID, phone, name, bookingID, lastVisit).Error(0)
}
func (m *MockRepo) GetByID(ctx context.Context, id string) (*model.Client, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Client), args.Error(1)
}
func (m *MockRepo) GetByPhone(ctx context.Context, barberID, phone string) (*model.Client, error) {
	args := m.Called(ctx, barberID, phone)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Client), args.Error(1)
}
func (m *MockRepo) List(ctx context.Context, barberID, search string) ([]model.Client, error) {
	args := m.Called(ctx, barberID, search)
	return args.Get(0).([]model.Client), args.Error(1)
}
func (m *MockRepo) Update(ctx context.Context, id, name, notes string) (*model.Client, error) {
	args := m.Called(ctx, id, name, notes)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Client), args.Error(1)
}
func (m *MockRepo) Delete(ctx context.Context, id, barberID string) error {
	return m.Called(ctx, id, barberID).Error(0)
}
// ─── helper ───────────────────────────────────────────────────────────────────

func newSvc(r clientRepo) *Service {
	return New(r, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

var (
	ctx      = context.Background()
	now      = time.Date(2026, 3, 16, 10, 0, 0, 0, time.UTC)
	testClient = &model.Client{
		ID:          "cl-1",
		BarberID:    "b-1",
		Phone:       "+79001234567",
		Name:        "Ivan",
		Notes:       "VIP",
		VisitsCount: 3,
		LastVisit:   &now,
		CreatedAt:   now,
	}
)

// ─── UpsertByBooking ──────────────────────────────────────────────────────────

func TestUpsertByBooking_Success(t *testing.T) {
	r := new(MockRepo)
	r.On("UpsertByBooking", ctx, "b-1", "+7", "Ivan", "bk-1", now).Return(nil)

	err := newSvc(r).UpsertByBooking(ctx, "b-1", "+7", "Ivan", "bk-1", now)

	require.NoError(t, err)
	r.AssertExpectations(t)
}

func TestUpsertByBooking_RepoError(t *testing.T) {
	r := new(MockRepo)
	r.On("UpsertByBooking", ctx, "b-1", "+7", "Ivan", "bk-1", now).Return(errors.New("db error"))

	err := newSvc(r).UpsertByBooking(ctx, "b-1", "+7", "Ivan", "bk-1", now)

	var appErr *apperr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeInternal, appErr.Code)
}

// ─── GetClient ────────────────────────────────────────────────────────────────

func TestGetClient_Success(t *testing.T) {
	r := new(MockRepo)
	r.On("GetByID", ctx, "cl-1").Return(testClient, nil)

	c, err := newSvc(r).GetClient(ctx, "cl-1", "b-1")

	require.NoError(t, err)
	assert.Equal(t, testClient, c)
	r.AssertExpectations(t)
}

func TestGetClient_OwnershipMismatch(t *testing.T) {
	r := new(MockRepo)
	r.On("GetByID", ctx, "cl-1").Return(testClient, nil)

	_, err := newSvc(r).GetClient(ctx, "cl-1", "b-2")

	var appErr *apperr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeNotFound, appErr.Code)
}

func TestGetClient_EmptyID(t *testing.T) {
	_, err := newSvc(new(MockRepo)).GetClient(ctx, "", "b-1")

	var appErr *apperr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeInvalidArgument, appErr.Code)
}

func TestGetClient_NotFound(t *testing.T) {
	r := new(MockRepo)
	r.On("GetByID", ctx, "cl-x").Return(nil, repository.ErrNotFound)

	_, err := newSvc(r).GetClient(ctx, "cl-x", "b-1")

	var appErr *apperr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeNotFound, appErr.Code)
}

func TestGetClient_RepoError(t *testing.T) {
	r := new(MockRepo)
	r.On("GetByID", ctx, "cl-1").Return(nil, errors.New("db error"))

	_, err := newSvc(r).GetClient(ctx, "cl-1", "b-1")

	var appErr *apperr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeInternal, appErr.Code)
}

// ─── GetClientByPhone ─────────────────────────────────────────────────────────

func TestGetClientByPhone_Success(t *testing.T) {
	r := new(MockRepo)
	r.On("GetByPhone", ctx, "b-1", "+7").Return(testClient, nil)

	c, err := newSvc(r).GetClientByPhone(ctx, "b-1", "+7")

	require.NoError(t, err)
	assert.Equal(t, testClient, c)
	r.AssertExpectations(t)
}

func TestGetClientByPhone_EmptyBarberID(t *testing.T) {
	_, err := newSvc(new(MockRepo)).GetClientByPhone(ctx, "", "+7")

	var appErr *apperr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeInvalidArgument, appErr.Code)
}

func TestGetClientByPhone_EmptyPhone(t *testing.T) {
	_, err := newSvc(new(MockRepo)).GetClientByPhone(ctx, "b-1", "")

	var appErr *apperr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeInvalidArgument, appErr.Code)
}

func TestGetClientByPhone_NotFound(t *testing.T) {
	r := new(MockRepo)
	r.On("GetByPhone", ctx, "b-1", "+7").Return(nil, repository.ErrNotFound)

	_, err := newSvc(r).GetClientByPhone(ctx, "b-1", "+7")

	var appErr *apperr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeNotFound, appErr.Code)
}

func TestGetClientByPhone_RepoError(t *testing.T) {
	r := new(MockRepo)
	r.On("GetByPhone", ctx, "b-1", "+7").Return(nil, errors.New("db error"))

	_, err := newSvc(r).GetClientByPhone(ctx, "b-1", "+7")

	var appErr *apperr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeInternal, appErr.Code)
}

// ─── ListClients ──────────────────────────────────────────────────────────────

func TestListClients_Success(t *testing.T) {
	r := new(MockRepo)
	r.On("List", ctx, "b-1", "").Return([]model.Client{*testClient}, nil)

	clients, err := newSvc(r).ListClients(ctx, "b-1", "")

	require.NoError(t, err)
	require.Len(t, clients, 1)
	assert.Equal(t, "cl-1", clients[0].ID)
}

func TestListClients_WithSearch(t *testing.T) {
	r := new(MockRepo)
	r.On("List", ctx, "b-1", "Ivan").Return([]model.Client{*testClient}, nil)

	clients, err := newSvc(r).ListClients(ctx, "b-1", "Ivan")

	require.NoError(t, err)
	assert.Len(t, clients, 1)
}

func TestListClients_EmptyBarberID(t *testing.T) {
	_, err := newSvc(new(MockRepo)).ListClients(ctx, "", "")

	var appErr *apperr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeInvalidArgument, appErr.Code)
}

func TestListClients_RepoError(t *testing.T) {
	r := new(MockRepo)
	r.On("List", ctx, "b-1", "").Return([]model.Client(nil), errors.New("db error"))

	_, err := newSvc(r).ListClients(ctx, "b-1", "")

	var appErr *apperr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeInternal, appErr.Code)
}

// ─── UpdateClient ─────────────────────────────────────────────────────────────

func TestUpdateClient_Success(t *testing.T) {
	updated := &model.Client{ID: "cl-1", BarberID: "b-1", Name: "Petr", Notes: "updated"}
	r := new(MockRepo)
	r.On("GetByID", ctx, "cl-1").Return(testClient, nil)
	r.On("Update", ctx, "cl-1", "Petr", "updated").Return(updated, nil)

	c, err := newSvc(r).UpdateClient(ctx, "cl-1", "b-1", "Petr", "updated")

	require.NoError(t, err)
	assert.Equal(t, "Petr", c.Name)
	r.AssertExpectations(t)
}

func TestUpdateClient_OwnershipMismatch(t *testing.T) {
	r := new(MockRepo)
	r.On("GetByID", ctx, "cl-1").Return(testClient, nil)

	_, err := newSvc(r).UpdateClient(ctx, "cl-1", "b-2", "Petr", "")

	var appErr *apperr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeNotFound, appErr.Code)
}

func TestUpdateClient_EmptyID(t *testing.T) {
	_, err := newSvc(new(MockRepo)).UpdateClient(ctx, "", "b-1", "Ivan", "")

	var appErr *apperr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeInvalidArgument, appErr.Code)
}

func TestUpdateClient_EmptyName(t *testing.T) {
	_, err := newSvc(new(MockRepo)).UpdateClient(ctx, "cl-1", "b-1", "", "")

	var appErr *apperr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeInvalidArgument, appErr.Code)
}

func TestUpdateClient_NotFound(t *testing.T) {
	r := new(MockRepo)
	r.On("GetByID", ctx, "cl-x").Return(nil, repository.ErrNotFound)

	_, err := newSvc(r).UpdateClient(ctx, "cl-x", "b-1", "Ivan", "")

	var appErr *apperr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeNotFound, appErr.Code)
}

func TestUpdateClient_RepoError(t *testing.T) {
	r := new(MockRepo)
	r.On("GetByID", ctx, "cl-1").Return(testClient, nil)
	r.On("Update", ctx, "cl-1", "Ivan", "").Return(nil, errors.New("db error"))

	_, err := newSvc(r).UpdateClient(ctx, "cl-1", "b-1", "Ivan", "")

	var appErr *apperr.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.CodeInternal, appErr.Code)
}
