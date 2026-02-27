package services

import (
	"log/slog"

	"github.com/RomanKovalev007/barber_crm/services/booking/internal/repo"
	"github.com/redis/go-redis/v9"
)

type BookingIntr interface {

}

type bookingService struct {

}

func New(repo *repo.Repo, rc *redis.Client, ttl int, jwt string, log *slog.Logger) BookingIntr {
	return bookingService{}
}