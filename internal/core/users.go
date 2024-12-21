package core

import (
	"context"
	"fmt"

	"github.com/cvhariharan/autopilot/internal/models"
)

func (c *Core) GetUserByUsername(ctx context.Context, username string) (models.User, error) {
	user, err := c.store.GetUserByUsername(ctx, username)
	if err != nil {
		return models.User{}, fmt.Errorf("could not get user %s: %w", username, err)
	}

	var p string
	if user.Password.Valid {
		p = user.Password.String
	}

	return models.User{
		UUID:     user.Uuid.String(),
		Username: user.Username,
		Password: p,
	}, nil
}
