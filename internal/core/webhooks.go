package core

import (
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/cvhariharan/flowctl/internal/core/models"
	"github.com/cvhariharan/flowctl/internal/repo"
	"github.com/google/uuid"
)

var webhookTypes = map[string]struct{}{
	"generic": {},
	"slack":   {},
	"teams":   {},
}

func (c *Core) CreateNamespaceWebhook(ctx context.Context, webhook models.WebhookDestination, namespaceID string) (models.WebhookDestination, error) {
	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return models.WebhookDestination{}, fmt.Errorf("invalid namespace UUID: %w", err)
	}

	name := strings.TrimSpace(webhook.Name)
	if name == "" {
		return models.WebhookDestination{}, errors.New("webhook name is required")
	}

	webhookType := normalizeWebhookType(webhook.Type)
	if err := validateWebhookType(webhookType); err != nil {
		return models.WebhookDestination{}, err
	}

	webhook.URL = strings.TrimSpace(webhook.URL)
	if webhook.URL == "" {
		return models.WebhookDestination{}, errors.New("webhook URL is required")
	}
	if err := validateWebhookURL(webhook.URL); err != nil {
		return models.WebhookDestination{}, err
	}

	if err := validateWebhookHeaders(webhook.Headers); err != nil {
		return models.WebhookDestination{}, err
	}

	if strings.TrimSpace(webhook.Template.Body) == "" {
		return models.WebhookDestination{}, errors.New("template body is required")
	}

	if webhook.ContentType == "" {
		webhook.ContentType = "application/json"
	}
	if webhook.Template.Format == "" {
		webhook.Template.Format = "json"
	}

	if err := c.ensureWebhookNameUnique(ctx, namespaceUUID, name, uuid.Nil); err != nil {
		return models.WebhookDestination{}, err
	}

	encryptedURL, err := c.encryptWebhookValue(ctx, webhook.URL)
	if err != nil {
		return models.WebhookDestination{}, err
	}

	encryptedHeaders, err := c.encryptWebhookHeaders(ctx, webhook.Headers)
	if err != nil {
		return models.WebhookDestination{}, err
	}

	var description sql.NullString
	if strings.TrimSpace(webhook.Description) != "" {
		description = sql.NullString{String: webhook.Description, Valid: true}
	}

	created, err := c.store.CreateNamespaceWebhook(ctx, repo.CreateNamespaceWebhookParams{
		Name:             name,
		Description:      description,
		Type:             webhookType,
		EncryptedUrl:     encryptedURL,
		EncryptedHeaders: encryptedHeaders,
		ContentType:      webhook.ContentType,
		TemplateBody:     webhook.Template.Body,
		TemplateFormat:   webhook.Template.Format,
		IsActive:         webhook.IsActive,
		Uuid:             namespaceUUID,
	})
	if err != nil {
		return models.WebhookDestination{}, err
	}

	return models.WebhookDestination{
		ID:            created.Uuid.String(),
		Name:          created.Name,
		Description:   webhook.Description,
		Type:          created.Type,
		URL:           webhook.URL,
		Headers:       webhook.Headers,
		ContentType:   created.ContentType,
		Template:      models.WebhookTemplate{Format: created.TemplateFormat, Body: created.TemplateBody},
		IsActive:      created.IsActive,
		NamespaceUUID: namespaceID,
		CreatedAt:     created.CreatedAt.Format(models.TimeFormat),
		UpdatedAt:     created.UpdatedAt.Format(models.TimeFormat),
	}, nil
}

func (c *Core) GetNamespaceWebhookByID(ctx context.Context, id string, namespaceID string) (models.WebhookDestination, error) {
	uuidID, err := uuid.Parse(id)
	if err != nil {
		return models.WebhookDestination{}, err
	}

	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return models.WebhookDestination{}, fmt.Errorf("invalid namespace UUID: %w", err)
	}

	webhook, err := c.store.GetNamespaceWebhookByUUID(ctx, repo.GetNamespaceWebhookByUUIDParams{
		Uuid:   uuidID,
		Uuid_2: namespaceUUID,
	})
	if err != nil {
		return models.WebhookDestination{}, err
	}

	return c.buildWebhookDestination(ctx, webhook, namespaceID)
}

func (c *Core) ListNamespaceWebhooks(ctx context.Context, namespaceID string) ([]models.WebhookDestination, error) {
	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return nil, fmt.Errorf("invalid namespace UUID: %w", err)
	}

	webhooks, err := c.store.ListNamespaceWebhooks(ctx, namespaceUUID)
	if err != nil {
		return nil, err
	}

	results := make([]models.WebhookDestination, 0, len(webhooks))
	for _, hook := range webhooks {
		wh, err := c.buildWebhookDestination(ctx, hook, namespaceID)
		if err != nil {
			return nil, err
		}
		results = append(results, wh)
	}

	return results, nil
}

func (c *Core) UpdateNamespaceWebhook(ctx context.Context, id string, update models.WebhookDestinationUpdate, namespaceID string) (models.WebhookDestination, error) {
	uuidID, err := uuid.Parse(id)
	if err != nil {
		return models.WebhookDestination{}, err
	}

	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return models.WebhookDestination{}, fmt.Errorf("invalid namespace UUID: %w", err)
	}

	existing, err := c.store.GetNamespaceWebhookByUUID(ctx, repo.GetNamespaceWebhookByUUIDParams{
		Uuid:   uuidID,
		Uuid_2: namespaceUUID,
	})
	if err != nil {
		return models.WebhookDestination{}, err
	}

	name := strings.TrimSpace(update.Name)
	if name == "" {
		return models.WebhookDestination{}, errors.New("webhook name is required")
	}

	webhookType := normalizeWebhookType(update.Type)
	if err := validateWebhookType(webhookType); err != nil {
		return models.WebhookDestination{}, err
	}

	if name != existing.Name {
		inUse, err := c.webhookNameInUse(ctx, namespaceUUID, existing.Name)
		if err != nil {
			return models.WebhookDestination{}, err
		}
		if inUse {
			return models.WebhookDestination{}, fmt.Errorf("webhook %s is in use; duplicate it instead of renaming", existing.Name)
		}
		if err := c.ensureWebhookNameUnique(ctx, namespaceUUID, name, uuidID); err != nil {
			return models.WebhookDestination{}, err
		}
	}

	if strings.TrimSpace(update.Template.Body) == "" {
		return models.WebhookDestination{}, errors.New("template body is required")
	}
	if update.Template.Format == "" {
		update.Template.Format = existing.TemplateFormat
	}

	contentType := update.ContentType
	if contentType == "" {
		contentType = existing.ContentType
	}

	isActive := existing.IsActive
	if update.IsActive != nil {
		isActive = *update.IsActive
	}

	encryptedURL := existing.EncryptedUrl
	if update.URL != nil {
		newURL := strings.TrimSpace(*update.URL)
		if newURL == "" {
			return models.WebhookDestination{}, errors.New("webhook URL cannot be empty")
		}
		if err := validateWebhookURL(newURL); err != nil {
			return models.WebhookDestination{}, err
		}
		encryptedURL, err = c.encryptWebhookValue(ctx, newURL)
		if err != nil {
			return models.WebhookDestination{}, err
		}
	}

	encryptedHeaders := existing.EncryptedHeaders
	if update.Headers != nil {
		if err := validateWebhookHeaders(*update.Headers); err != nil {
			return models.WebhookDestination{}, err
		}
		encryptedHeaders, err = c.encryptWebhookHeaders(ctx, *update.Headers)
		if err != nil {
			return models.WebhookDestination{}, err
		}
	}

	var description sql.NullString
	if strings.TrimSpace(update.Description) != "" {
		description = sql.NullString{String: update.Description, Valid: true}
	}

	updated, err := c.store.UpdateNamespaceWebhook(ctx, repo.UpdateNamespaceWebhookParams{
		Uuid:             uuidID,
		Name:             name,
		Description:      description,
		Type:             webhookType,
		EncryptedUrl:     encryptedURL,
		EncryptedHeaders: encryptedHeaders,
		ContentType:      contentType,
		TemplateBody:     update.Template.Body,
		TemplateFormat:   update.Template.Format,
		IsActive:         isActive,
		Uuid_2:           namespaceUUID,
	})
	if err != nil {
		return models.WebhookDestination{}, err
	}

	return c.buildWebhookDestination(ctx, updated, namespaceID)
}

func (c *Core) DeleteNamespaceWebhook(ctx context.Context, id string, namespaceID string) error {
	uuidID, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid webhook UUID: %w", err)
	}

	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return fmt.Errorf("invalid namespace UUID: %w", err)
	}

	return c.store.DeleteNamespaceWebhook(ctx, repo.DeleteNamespaceWebhookParams{
		Uuid:   uuidID,
		Uuid_2: namespaceUUID,
	})
}

func (c *Core) DuplicateNamespaceWebhook(ctx context.Context, id string, name string, description string, namespaceID string) (models.WebhookDestination, error) {
	uuidID, err := uuid.Parse(id)
	if err != nil {
		return models.WebhookDestination{}, err
	}

	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return models.WebhookDestination{}, fmt.Errorf("invalid namespace UUID: %w", err)
	}

	existing, err := c.store.GetNamespaceWebhookByUUID(ctx, repo.GetNamespaceWebhookByUUIDParams{
		Uuid:   uuidID,
		Uuid_2: namespaceUUID,
	})
	if err != nil {
		return models.WebhookDestination{}, err
	}

	newName := strings.TrimSpace(name)
	if newName == "" {
		return models.WebhookDestination{}, errors.New("webhook name is required")
	}
	if err := c.ensureWebhookNameUnique(ctx, namespaceUUID, newName, uuid.Nil); err != nil {
		return models.WebhookDestination{}, err
	}

	desc := description
	if strings.TrimSpace(desc) == "" && existing.Description.Valid {
		desc = existing.Description.String
	}

	var descNull sql.NullString
	if strings.TrimSpace(desc) != "" {
		descNull = sql.NullString{String: desc, Valid: true}
	}

	created, err := c.store.CreateNamespaceWebhook(ctx, repo.CreateNamespaceWebhookParams{
		Name:             newName,
		Description:      descNull,
		Type:             existing.Type,
		EncryptedUrl:     existing.EncryptedUrl,
		EncryptedHeaders: existing.EncryptedHeaders,
		ContentType:      existing.ContentType,
		TemplateBody:     existing.TemplateBody,
		TemplateFormat:   existing.TemplateFormat,
		IsActive:         existing.IsActive,
		Uuid:             namespaceUUID,
	})
	if err != nil {
		return models.WebhookDestination{}, err
	}

	return c.buildWebhookDestination(ctx, created, namespaceID)
}

func (c *Core) buildWebhookDestination(ctx context.Context, webhook interface{}, namespaceID string) (models.WebhookDestination, error) {
	switch w := webhook.(type) {
	case repo.NamespaceWebhook:
		return c.decodeWebhookRow(ctx, w.Uuid, w.Name, w.Description, w.Type, w.EncryptedUrl, w.EncryptedHeaders, w.ContentType, w.TemplateBody, w.TemplateFormat, w.IsActive, w.CreatedAt, w.UpdatedAt, namespaceID)
	case repo.GetNamespaceWebhookByUUIDRow:
		return c.decodeWebhookRow(ctx, w.Uuid, w.Name, w.Description, w.Type, w.EncryptedUrl, w.EncryptedHeaders, w.ContentType, w.TemplateBody, w.TemplateFormat, w.IsActive, w.CreatedAt, w.UpdatedAt, w.NamespaceUuid.String())
	case repo.GetNamespaceWebhookByIDRow:
		return c.decodeWebhookRow(ctx, w.Uuid, w.Name, w.Description, w.Type, w.EncryptedUrl, w.EncryptedHeaders, w.ContentType, w.TemplateBody, w.TemplateFormat, w.IsActive, w.CreatedAt, w.UpdatedAt, w.NamespaceUuid.String())
	case repo.GetNamespaceWebhookByNameRow:
		return c.decodeWebhookRow(ctx, w.Uuid, w.Name, w.Description, w.Type, w.EncryptedUrl, w.EncryptedHeaders, w.ContentType, w.TemplateBody, w.TemplateFormat, w.IsActive, w.CreatedAt, w.UpdatedAt, w.NamespaceUuid.String())
	case repo.ListNamespaceWebhooksRow:
		return c.decodeWebhookRow(ctx, w.Uuid, w.Name, w.Description, w.Type, w.EncryptedUrl, w.EncryptedHeaders, w.ContentType, w.TemplateBody, w.TemplateFormat, w.IsActive, w.CreatedAt, w.UpdatedAt, w.NamespaceUuid.String())
	default:
		return models.WebhookDestination{}, errors.New("unsupported webhook type")
	}
}

func (c *Core) decodeWebhookRow(ctx context.Context, webhookUUID uuid.UUID, name string, description sql.NullString, webhookType string, encryptedURL string, encryptedHeaders sql.NullString, contentType string, templateBody string, templateFormat string, isActive bool, createdAt, updatedAt time.Time, namespaceUUID string) (models.WebhookDestination, error) {
	urlValue, err := c.decryptWebhookValue(ctx, encryptedURL)
	if err != nil {
		return models.WebhookDestination{}, err
	}

	headers, err := c.decryptWebhookHeaders(ctx, encryptedHeaders)
	if err != nil {
		return models.WebhookDestination{}, err
	}

	desc := ""
	if description.Valid {
		desc = description.String
	}

	return models.WebhookDestination{
		ID:            webhookUUID.String(),
		Name:          name,
		Description:   desc,
		Type:          webhookType,
		URL:           urlValue,
		Headers:       headers,
		ContentType:   contentType,
		Template:      models.WebhookTemplate{Format: templateFormat, Body: templateBody},
		IsActive:      isActive,
		NamespaceUUID: namespaceUUID,
		CreatedAt:     createdAt.Format(models.TimeFormat),
		UpdatedAt:     updatedAt.Format(models.TimeFormat),
	}, nil
}

func (c *Core) encryptWebhookValue(ctx context.Context, value string) (string, error) {
	enc, err := c.keeper.Encrypt(ctx, []byte(value))
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(enc), nil
}

func (c *Core) decryptWebhookValue(ctx context.Context, encryptedValue string) (string, error) {
	if encryptedValue == "" {
		return "", nil
	}
	encryptedBytes, err := hex.DecodeString(encryptedValue)
	if err != nil {
		return "", fmt.Errorf("could not decode encrypted value: %w", err)
	}
	decryptedValue, err := c.keeper.Decrypt(ctx, encryptedBytes)
	if err != nil {
		return "", fmt.Errorf("could not decrypt value: %w", err)
	}
	return string(decryptedValue), nil
}

func (c *Core) encryptWebhookHeaders(ctx context.Context, headers []models.WebhookHeader) (sql.NullString, error) {
	if len(headers) == 0 {
		return sql.NullString{}, nil
	}
	data, err := json.Marshal(headers)
	if err != nil {
		return sql.NullString{}, err
	}
	encrypted, err := c.encryptWebhookValue(ctx, string(data))
	if err != nil {
		return sql.NullString{}, err
	}
	return sql.NullString{String: encrypted, Valid: true}, nil
}

func (c *Core) decryptWebhookHeaders(ctx context.Context, encryptedHeaders sql.NullString) ([]models.WebhookHeader, error) {
	if !encryptedHeaders.Valid || encryptedHeaders.String == "" {
		return nil, nil
	}
	raw, err := c.decryptWebhookValue(ctx, encryptedHeaders.String)
	if err != nil {
		return nil, err
	}
	var headers []models.WebhookHeader
	if err := json.Unmarshal([]byte(raw), &headers); err != nil {
		return nil, err
	}
	return headers, nil
}

func validateWebhookURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid webhook URL: %w", err)
	}
	if strings.ToLower(parsed.Scheme) != "https" {
		return errors.New("webhook URL must use https")
	}
	if parsed.Host == "" {
		return errors.New("webhook URL must include a host")
	}
	return nil
}

func validateWebhookHeaders(headers []models.WebhookHeader) error {
	for _, header := range headers {
		if strings.TrimSpace(header.Key) == "" {
			return errors.New("header key is required")
		}
		if strings.TrimSpace(header.Value) == "" {
			return errors.New("header value is required")
		}
		if strings.ContainsAny(header.Key, "\r\n\x00") || strings.ContainsAny(header.Value, "\r\n\x00") {
			return errors.New("header values cannot include control characters")
		}
	}
	return nil
}

func validateWebhookType(webhookType string) error {
	if _, ok := webhookTypes[webhookType]; !ok {
		return fmt.Errorf("invalid webhook type %q", webhookType)
	}
	return nil
}

func normalizeWebhookType(webhookType string) string {
	return strings.ToLower(strings.TrimSpace(webhookType))
}

func (c *Core) ensureWebhookNameUnique(ctx context.Context, namespaceUUID uuid.UUID, name string, exclude uuid.UUID) error {
	existing, err := c.store.GetNamespaceWebhookByName(ctx, repo.GetNamespaceWebhookByNameParams{
		Uuid: namespaceUUID,
		Name: name,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}

	if existing.Uuid == exclude {
		return nil
	}

	return fmt.Errorf("webhook name %q already exists", name)
}

func (c *Core) webhookNameInUse(ctx context.Context, namespaceUUID uuid.UUID, webhookName string) (bool, error) {
	namespace, err := c.store.GetNamespaceByUUID(ctx, namespaceUUID)
	if err != nil {
		return false, err
	}

	c.rwf.RLock()
	defer c.rwf.RUnlock()

	for _, flow := range c.flows {
		if flow.Meta.Namespace != namespace.Name {
			continue
		}
		for _, notify := range flow.Notify {
			if notify.Channel != "webhook" {
				continue
			}
			for _, name := range notify.WebhookNames {
				if name == webhookName {
					return true, nil
				}
			}
		}
	}

	return false, nil
}
