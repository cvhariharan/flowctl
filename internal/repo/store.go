package repo

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type Store interface {
	Querier
	OverwriteGroupsForUserTx(ctx context.Context, userUUID uuid.UUID, groups []string) error
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

func (p *PostgresStore) OverwriteGroupsForUserTx(ctx context.Context, userUUID uuid.UUID, groups []string) error {
	tx, err := p.db.Begin()
	if err != nil {
		return fmt.Errorf("could not start transaction: %w", err)
	}
	defer tx.Rollback()

	q := Queries{db: tx}

	if err := q.RemoveAllGroupsForUserByUUID(ctx, userUUID); err != nil {
		return err
	}

	for _, group := range groups {
		gid, err := uuid.Parse(group)
		if err != nil {
			return fmt.Errorf("group ID should be a UUID: %w", err)
		}

		if err := q.AddGroupToUserByUUID(ctx, AddGroupToUserByUUIDParams{
			UserUuid:  userUUID,
			GroupUuid: gid,
		}); err != nil {
			return fmt.Errorf("could not add group %s to user %s: %w", group, userUUID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("coudl not commit transaction: %w", err)
	}

	return nil
}
