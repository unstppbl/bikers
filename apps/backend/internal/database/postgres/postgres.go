// Package postgres implements repository contracts using PostgreSQL. Application
// decisions are supplied by managers; this package owns SQL, mapping and locks.
package postgres

import (
	"context"
	"errors"

	"bikers.kz/backend/internal/database"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// Close is available to wiring that gives the Store ownership of its pool.
func (s *Store) Close() { s.pool.Close() }

var _ database.Repository = (*Store)(nil)

type querier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func repositoryError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return database.ErrNotFound
	}
	var pgerr *pgconn.PgError
	if errors.As(err, &pgerr) && pgerr.Code == "23505" {
		return database.ErrConflict
	}
	return err
}
