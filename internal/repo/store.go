package repo

import (
	"github.com/jmoiron/sqlx"
)

type Store interface {
	Querier
}

type PostgresStore struct {
	*Queries
	db *sqlx.DB
}

func NewPostgresStore(db *sqlx.DB) Store {
	return &PostgresStore{
		db:      db,
		Queries: New(db),
	}
}
