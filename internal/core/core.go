package core

import (
	"context"
	"sync"
	"time"

	"github.com/casbin/casbin/v2"
	"github.com/cvhariharan/flowctl/internal/core/models"
	"github.com/cvhariharan/flowctl/internal/repo"
	"github.com/cvhariharan/flowctl/internal/scheduler"
	"github.com/cvhariharan/flowctl/internal/streamlogger"
	"gocloud.dev/secrets"
)

const (
	TimeFormat = time.RFC3339
)

type Core struct {
	store      repo.Store
	scheduler  scheduler.TaskScheduler
	rwf        sync.RWMutex
	flows      map[string]models.Flow
	keeper     *secrets.Keeper
	LogManager streamlogger.LogManager

	// store the mapping between logID and flowID
	logMap   map[string]string
	enforcer *casbin.Enforcer

	flowDirectory string
	messengers    []string
}

func NewCore(flowsDirectory string, s repo.Store, sch scheduler.TaskScheduler, keeper *secrets.Keeper, enforcer *casbin.Enforcer, messengerNames []string) (*Core, error) {
	c := &Core{
		store:         s,
		scheduler:     sch,
		flowDirectory: flowsDirectory,
		flows:         make(map[string]models.Flow),
		logMap:        make(map[string]string),
		keeper:        keeper,
		enforcer:      enforcer,
		messengers:    messengerNames,
	}

	if err := c.LoadFlows(context.Background()); err != nil {
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

func (co *Core) GetMessengers(ctx context.Context) []string {
	return co.messengers
}
