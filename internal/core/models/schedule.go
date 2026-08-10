package models

import "time"

type Schedule struct {
	UUID          string                 `json:"uuid" yaml:"-" huml:"-"`
	FlowSlug      string                 `json:"flow_slug" yaml:"-" huml:"-"`
	FlowName      string                 `json:"flow_name" yaml:"-" huml:"-"`
	Name          string                 `json:"name" yaml:"name,omitempty" huml:"name" validate:"max=150"`
	Cron          string                 `json:"cron" yaml:"cron" huml:"cron"`
	Timezone      string                 `json:"timezone" yaml:"timezone" huml:"timezone"`
	Inputs        map[string]interface{} `json:"inputs" yaml:"-" huml:"-"`
	CreatedByID   string                 `json:"created_by_id" yaml:"-" huml:"-"`
	CreatedByName string                 `json:"created_by_name" yaml:"-" huml:"-"`
	IsActive      bool                   `json:"is_active" yaml:"-" huml:"-"`
	IsUserCreated bool                   `json:"is_user_created" yaml:"-" huml:"-"`
	CreatedAt     time.Time              `json:"created_at" yaml:"-" huml:"-"`
	UpdatedAt     time.Time              `json:"updated_at" yaml:"-" huml:"-"`
}

type NextScheduledRun struct {
	At   time.Time
	Name string
}
