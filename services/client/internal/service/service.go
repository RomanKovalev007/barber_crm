package service

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/RomanKovalev007/barber_crm/services/client/internal/apperr"
	"github.com/RomanKovalev007/barber_crm/services/client/internal/model"
	"github.com/RomanKovalev007/barber_crm/services/client/internal/repository"
)

type clientRepo interface {
	UpsertByBooking(ctx context.Context, barberID, phone, name, bookingID string, lastVisit time.Time) error
	GetByID(ctx context.Context, id string) (*model.Client, error)
	GetByPhone(ctx context.Context, barberID, phone string) (*model.Client, error)
	List(ctx context.Context, barberID, search string) ([]model.Client, error)
	Update(ctx context.Context, id, name, notes string) (*model.Client, error)
	Delete(ctx context.Context, id, barberID string) error
}

type Service struct {
	repo   clientRepo
	logger *slog.Logger
}

func New(repo clientRepo, log *slog.Logger) *Service {
	return &Service{repo: repo, logger: log}
}

func (s *Service) UpsertByBooking(ctx context.Context, barberID, phone, name, bookingID string, lastVisit time.Time) error {
	if err := s.repo.UpsertByBooking(ctx, barberID, phone, name, bookingID, lastVisit); err != nil {
		s.logger.Error("upsert client by booking", "booking_id", bookingID, "barber_id", barberID, "error", err)
		return apperr.Internal("failed to upsert client")
	}
	s.logger.Info("client upserted", "booking_id", bookingID, "barber_id", barberID, "phone", phone)
	return nil
}

func (s *Service) GetClient(ctx context.Context, id, barberID string) (*model.Client, error) {
	if id == "" {
		return nil, apperr.InvalidArgument("client_id is required")
	}
	c, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, apperr.NotFound("client not found")
		}
		s.logger.Error("get client", "client_id", id, "error", err)
		return nil, apperr.Internal("failed to get client")
	}
	if c.BarberID != barberID {
		s.logger.Warn("get client: ownership mismatch", "client_id", id, "barber_id", barberID)
		return nil, apperr.NotFound("client not found")
	}
	return c, nil
}

func (s *Service) GetClientByPhone(ctx context.Context, barberID, phone string) (*model.Client, error) {
	if barberID == "" || phone == "" {
		return nil, apperr.InvalidArgument("barber_id and phone are required")
	}
	c, err := s.repo.GetByPhone(ctx, barberID, phone)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, apperr.NotFound("client not found")
		}
		s.logger.Error("get client by phone", "barber_id", barberID, "phone", phone, "error", err)
		return nil, apperr.Internal("failed to get client")
	}
	return c, nil
}

func (s *Service) ListClients(ctx context.Context, barberID, search string) ([]model.Client, error) {
	if barberID == "" {
		return nil, apperr.InvalidArgument("barber_id is required")
	}
	clients, err := s.repo.List(ctx, barberID, search)
	if err != nil {
		s.logger.Error("list clients", "barber_id", barberID, "error", err)
		return nil, apperr.Internal("failed to list clients")
	}
	return clients, nil
}

func (s *Service) UpdateClient(ctx context.Context, id, barberID, name, notes string) (*model.Client, error) {
	if id == "" {
		return nil, apperr.InvalidArgument("client_id is required")
	}
	if name == "" {
		return nil, apperr.InvalidArgument("name is required")
	}
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, apperr.NotFound("client not found")
		}
		s.logger.Error("update client: get", "client_id", id, "error", err)
		return nil, apperr.Internal("failed to get client")
	}
	if existing.BarberID != barberID {
		s.logger.Warn("update client: ownership mismatch", "client_id", id, "barber_id", barberID)
		return nil, apperr.NotFound("client not found")
	}
	c, err := s.repo.Update(ctx, id, name, notes)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, apperr.NotFound("client not found")
		}
		s.logger.Error("update client", "client_id", id, "error", err)
		return nil, apperr.Internal("failed to update client")
	}
	s.logger.Info("client updated", "client_id", id, "barber_id", barberID)
	return c, nil
}

func (s *Service) DeleteClient(ctx context.Context, id, barberID string) error {
	if id == "" {
		return apperr.InvalidArgument("client_id is required")
	}
	if barberID == "" {
		return apperr.InvalidArgument("barber_id is required")
	}
	err := s.repo.Delete(ctx, id, barberID)
	if err != nil {
		s.logger.Error("delete client", "client_id", id, "barber_id", barberID, "error", err)
		return apperr.Internal("failed to delete client")
	}
	s.logger.Info("client deleted", "client_id", id, "barber_id", barberID)
	
	return nil
}
