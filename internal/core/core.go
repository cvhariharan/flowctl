package core

import (
	"context"
	"net/http"
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

	enforcer *casbin.Enforcer

	flowDirectory string
	httpClient    *http.Client

	remoteOptionsCache   map[string]remoteOptionsCacheEntry
	remoteOptionsCacheMu sync.RWMutex
}

func NewCore(flowsDirectory string, s repo.Store, sch scheduler.TaskScheduler, keeper *secrets.Keeper, enforcer *casbin.Enforcer) (*Core, error) {
	c := &Core{
		store:              s,
		scheduler:          sch,
		flowDirectory:      flowsDirectory,
		flows:              make(map[string]models.Flow),
		keeper:             keeper,
		enforcer:           enforcer,
		httpClient:         &http.Client{Timeout: 10 * time.Second},
		remoteOptionsCache: make(map[string]remoteOptionsCacheEntry),
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

// ResolveGroupEmails resolves a group name to member email addresses.
// This implements the messengers.GroupResolver interface.
func (c *Core) ResolveGroupEmails(ctx context.Context, groupName string) ([]string, error) {
	members, err := c.store.GetGroupMembersByName(ctx, groupName)
	if err != nil {
		return nil, err
	}
	emails := make([]string, 0, len(members))
	for _, m := range members {
		emails = append(emails, m.Username)
	}
	return emails, nil
}
