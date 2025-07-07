package core

import (
	"github.com/cvhariharan/autopilot/internal/core/models"
	"github.com/cvhariharan/autopilot/internal/repo"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
)

type Core struct {
	redisClient redis.UniversalClient
	store       repo.Store
	q           *asynq.Client
	flows       map[string]models.Flow

	// store the mapping between logID and flowID
	logMap map[string]string
}

func NewCore(flows map[string]models.Flow, s repo.Store, q *asynq.Client, redisClient redis.UniversalClient) *Core {
	return &Core{
		store:       s,
		redisClient: redisClient,
		q:           q,
		flows:       flows,
		logMap:      make(map[string]string),
	}
}
