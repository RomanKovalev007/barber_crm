package repo

import "github.com/jackc/pgx/v5/pgxpool"

type BookingRepo struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *BookingRepo {
	return &BookingRepo{
		pool: pool,
	}
}