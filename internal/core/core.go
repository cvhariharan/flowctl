package core

import (
	"context"
	"sync"

	"github.com/casbin/casbin/v2"
	"github.com/cvhariharan/flowctl/internal/core/models"
	"github.com/cvhariharan/flowctl/internal/repo"
	"github.com/cvhariharan/flowctl/internal/scheduler"
	"github.com/cvhariharan/flowctl/internal/streamlogger"
	"github.com/redis/go-redis/v9"
	"gocloud.dev/secrets"
)

const (
	TimeFormat = "2006-01-02T15:04:05Z"
)

type Core struct {
	redisClient redis.UniversalClient
	store       repo.Store
	scheduler   scheduler.TaskScheduler
	rwf         sync.RWMutex
	flows       map[string]models.Flow
	keeper      *secrets.Keeper
	LogManager  streamlogger.LogManager

	// store the mapping between logID and flowID
	logMap   map[string]string
	enforcer *casbin.Enforcer

	flowDirectory string
}

func NewCore(flowsDirectory string, s repo.Store, sch scheduler.TaskScheduler, redisClient redis.UniversalClient, keeper *secrets.Keeper, enforcer *casbin.Enforcer) (*Core, error) {
	c := &Core{
		store:         s,
		redisClient:   redisClient,
		scheduler:     sch,
		flowDirectory: flowsDirectory,
		flows:         make(map[string]models.Flow),
		logMap:        make(map[string]string),
		keeper:        keeper,
		enforcer:      enforcer,
	}

	if err := c.LoadFlows(); err != nil {
		return nil, err
	}
	if err := c.InitializeRBACPolicies(); err != nil {
		return nil, err
	}

	// Grant all superusers admin access to all namespaces
	if err := c.GrantSuperusersAdminAccessToAllNamespaces(context.Background()); err != nil {
		return nil, err
	}

	return c, nil
}
