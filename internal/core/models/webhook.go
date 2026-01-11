package models

type WebhookHeader struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type WebhookTemplate struct {
	Format string `json:"format"`
	Body   string `json:"body"`
}

type WebhookDestination struct {
	ID            string
	Name          string
	Description   string
	Type          string
	URL           string
	Headers       []WebhookHeader
	ContentType   string
	Template      WebhookTemplate
	IsActive      bool
	NamespaceUUID string
	CreatedAt     string
	UpdatedAt     string
}

type WebhookDestinationUpdate struct {
	Name          string
	Description   string
	Type          string
	URL           *string
	Headers       *[]WebhookHeader
	ContentType   string
	Template      WebhookTemplate
	IsActive      *bool
}
