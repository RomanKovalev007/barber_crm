package repo

import "github.com/jackc/pgx/v5/pgxpool"

type Repo struct {

}

func New(pool *pgxpool.Pool) *Repo {
	return &Repo{}
}