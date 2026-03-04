package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"

	"github.com/RomanKovalev007/barber_crm/pkg/auth"
	"github.com/RomanKovalev007/barber_crm/services/staff/internal/kafka"
	"github.com/RomanKovalev007/barber_crm/services/staff/internal/model"
)

type staffRepo interface {
	GetBarber(ctx context.Context, id string) (*model.Barber, error)
	GetBarberByLogin(ctx context.Context, login string) (*model.Barber, error)
	ListBarbers(ctx context.Context) ([]model.Barber, error)

	AddSchedule(ctx context.Context, barberID string, day *model.ScheduleDay) (*model.ScheduleDay, error)
	GetSchedule(ctx context.Context, barberID string, week string) ([]model.ScheduleDay, error)

	CreateService(ctx context.Context, s *model.Service) error
	UpdateService(ctx context.Context, s *model.Service) error
	DeleteService(ctx context.Context, id string, barberID string) error
	ListServices(ctx context.Context, barberID string, includeInactive bool) ([]model.Service, error)
}

type eventProducer interface {
	Publish(ctx context.Context, topic, key string, payload any) error
}

type Service struct {
	repo      staffRepo
	redisdb   *redis.Client
	producer  eventProducer
	ttl       int
	jwtSecret string
	logger    *slog.Logger
}

func New(repo staffRepo, redisdb *redis.Client, producer eventProducer, ttl int, jwtSecret string, logger *slog.Logger) *Service {
	return &Service{
		repo:      repo,
		redisdb:   redisdb,
		producer:  producer,
		ttl:       ttl,
		jwtSecret: jwtSecret,
		logger:    logger,
	}
}

// barbers

func (s *Service) Login(ctx context.Context, login, password string) (*model.Barber, string, string, error) {
	barber, err := s.repo.GetBarberByLogin(ctx, login)
	if err != nil {
		s.logger.Warn("login failed: barber not found", "login", login)
		return nil, "", "", fmt.Errorf("invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(barber.PasswordHash), []byte(password)); err != nil {
		s.logger.Warn("login failed: wrong password", "login", login)
		return nil, "", "", fmt.Errorf("invalid credentials")
	}

	accessToken, err := auth.GenerateAccessToken(barber.ID, s.jwtSecret)
	if err != nil {
		s.logger.Error("failed to generate access token", "barber_id", barber.ID, "error", err)
		return nil, "", "", fmt.Errorf("generate access token: %w", err)
	}

	refreshToken, err := auth.GenerateRefreshToken(barber.ID, s.jwtSecret)
	if err != nil {
		s.logger.Error("failed to generate refresh token", "barber_id", barber.ID, "error", err)
		return nil, "", "", fmt.Errorf("generate refresh token: %w", err)
	}

	if err := s.redisdb.Set(ctx, "session:"+barber.ID, refreshToken, time.Duration(s.ttl)*time.Minute).Err(); err != nil {
		s.logger.Error("failed to save session", "barber_id", barber.ID, "error", err)
		return nil, "", "", fmt.Errorf("save session: %w", err)
	}

	s.logger.Info("barber logged in", "barber_id", barber.ID)
	return barber, accessToken, refreshToken, nil
}

func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	claims, err := auth.ValidateToken(refreshToken, s.jwtSecret)
	if err != nil {
		s.logger.Warn("logout failed: invalid refresh token")
		return fmt.Errorf("invalid refresh token")
	}
	if err := s.redisdb.Del(ctx, "session:"+claims.BarberID).Err(); err != nil {
		s.logger.Error("failed to delete session", "barber_id", claims.BarberID, "error", err)
		return err
	}
	s.logger.Info("barber logged out", "barber_id", claims.BarberID)
	return nil
}

func (s *Service) RefreshToken(ctx context.Context, refreshTokenStr string) (string, string, error) {
	claims, err := auth.ValidateToken(refreshTokenStr, s.jwtSecret)
	if err != nil {
		s.logger.Warn("refresh token failed: invalid token")
		return "", "", fmt.Errorf("invalid refresh token")
	}

	stored, err := s.redisdb.Get(ctx, "session:"+claims.BarberID).Result()
	if err != nil || stored != refreshTokenStr {
		s.logger.Warn("refresh token failed: session mismatch or not found", "barber_id", claims.BarberID)
		return "", "", fmt.Errorf("invalid refresh token")
	}

	accessToken, err := auth.GenerateAccessToken(claims.BarberID, s.jwtSecret)
	if err != nil {
		s.logger.Error("failed to generate access token", "barber_id", claims.BarberID, "error", err)
		return "", "", err
	}

	newRefresh, err := auth.GenerateRefreshToken(claims.BarberID, s.jwtSecret)
	if err != nil {
		s.logger.Error("failed to generate refresh token", "barber_id", claims.BarberID, "error", err)
		return "", "", err
	}

	if err := s.redisdb.Set(ctx, "session:"+claims.BarberID, newRefresh, time.Duration(s.ttl)*time.Minute).Err(); err != nil {
		s.logger.Error("failed to save session", "barber_id", claims.BarberID, "error", err)
		return "", "", fmt.Errorf("save session: %w", err)
	}

	s.logger.Info("token refreshed", "barber_id", claims.BarberID)
	return accessToken, newRefresh, nil
}

func (s *Service) GetBarber(ctx context.Context, id string) (*model.Barber, error) {
	if id == "" {
		return nil, fmt.Errorf("barber_id is empty")
	}
	return s.repo.GetBarber(ctx, id)
}

func (s *Service) ListBarbers(ctx context.Context) ([]model.Barber, error) {
	return s.repo.ListBarbers(ctx)
}

// services

func (s *Service) CreateService(ctx context.Context, svc *model.Service) error {
	if svc.BarberID == "" {
		return fmt.Errorf("barber id can not be empty")
	}
	if len(svc.Name) < 2 {
		return fmt.Errorf("name length can be >= 2")
	}
	if svc.Price < 0 {
		return fmt.Errorf("price can not be negative")
	}
	if err := s.repo.CreateService(ctx, svc); err != nil {
		s.logger.Error("failed to create service", "barber_id", svc.BarberID, "error", err)
		return err
	}
	s.logger.Info("service created", "service_id", svc.ID, "barber_id", svc.BarberID)
	if err := s.producer.Publish(ctx, kafka.TopicServiceCreated, svc.BarberID, svc); err != nil {
		s.logger.Warn("failed to publish service.created event", "error", err)
	}
	return nil
}

func (s *Service) UpdateService(ctx context.Context, svc *model.Service) error {
	if svc.ID == "" {
		return fmt.Errorf("id is empty")
	}
	if svc.BarberID == "" {
		return fmt.Errorf("barber_id is empty")
	}
	if len(svc.Name) < 2 {
		return fmt.Errorf("name length can be >= 2")
	}
	if svc.Price < 0 {
		return fmt.Errorf("price can not be negative")
	}
	if err := s.repo.UpdateService(ctx, svc); err != nil {
		s.logger.Error("failed to update service", "service_id", svc.ID, "error", err)
		return err
	}
	s.logger.Info("service updated", "service_id", svc.ID, "barber_id", svc.BarberID)
	if err := s.producer.Publish(ctx, kafka.TopicServiceUpdated, svc.ID, svc); err != nil {
		s.logger.Warn("failed to publish service.updated event", "error", err)
	}
	return nil
}

func (s *Service) DeleteService(ctx context.Context, id, barberID string) error {
	if id == "" {
		return fmt.Errorf("service_id is empty")
	}
	if barberID == "" {
		return fmt.Errorf("barber_id is empty")
	}
	if err := s.repo.DeleteService(ctx, id, barberID); err != nil {
		s.logger.Error("failed to delete service", "service_id", id, "error", err)
		return err
	}
	s.logger.Info("service deleted", "service_id", id, "barber_id", barberID)
	if err := s.producer.Publish(ctx, kafka.TopicServiceDeleted, id, map[string]string{"id": id, "barber_id": barberID}); err != nil {
		s.logger.Warn("failed to publish service.deleted event", "error", err)
	}
	return nil
}

func (s *Service) ListServices(ctx context.Context, barberID string, includeInactive bool) ([]model.Service, error) {
	if barberID == "" {
		return nil, fmt.Errorf("barber_id is empty")
	}
	return s.repo.ListServices(ctx, barberID, includeInactive)
}

// schedule

func (s *Service) GetSchedule(ctx context.Context, barberID, week string) ([]model.ScheduleDay, error) {
	if barberID == "" {
		return nil, fmt.Errorf("barber_id is empty")
	}
	if week == "" {
		return nil, fmt.Errorf("week is empty")
	}
	return s.repo.GetSchedule(ctx, barberID, week)
}

func (s *Service) AddSchedule(ctx context.Context, barberID string, day *model.ScheduleDay) (*model.ScheduleDay, error) {
	if barberID == "" {
		return nil, fmt.Errorf("barber_id is empty")
	}
	if day.Date == "" {
		return nil, fmt.Errorf("date is empty")
	}
	if day.StartTime == "" {
		return nil, fmt.Errorf("start_time is empty")
	}
	if day.EndTime == "" {
		return nil, fmt.Errorf("end_time is empty")
	}
	result, err := s.repo.AddSchedule(ctx, barberID, day)
	if err != nil {
		s.logger.Error("failed to add schedule", "barber_id", barberID, "date", day.Date, "error", err)
		return nil, err
	}
	s.logger.Info("schedule added", "barber_id", barberID, "date", day.Date)
	if err := s.producer.Publish(ctx, kafka.TopicScheduleAdded, barberID, result); err != nil {
		s.logger.Warn("failed to publish schedule.added event", "error", err)
	}
	return result, nil
}
