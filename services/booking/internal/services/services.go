package services

import (
	"log/slog"

	"github.com/RomanKovalev007/barber_crm/services/booking/internal/repo"
	"github.com/redis/go-redis/v9"
)

type BookingIntr interface {

}

type bookingService struct {
	log *slog.Logger
	repo *repo.BookingRepo
	redis *redis.Client
}

func New(repo *repo.BookingRepo, rc *redis.Client, ttl int, jwt string, log *slog.Logger) BookingIntr {
	return bookingService{
		log: log,
		repo: repo,
		redis: rc,
	}
}