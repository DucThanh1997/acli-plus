package jirarest

import (
	"context"
	"errors"
	"net/http"
	"net/url"

	jira "acli-plus/internal/domain/jira"
)

// errNothingToUpdate is returned when an update call would send an empty body.
var errNothingToUpdate = errors.New("nothing to update: set at least one field")

// ListFields returns every system and custom field on the site. This is the
// lookup that turns a human field name into the customfield_NNNNN id that
// create and edit need.
func (c *Client) ListFields(ctx context.Context) ([]jira.Field, error) {
	var out []fieldDTO
	if _, err := c.do(ctx, http.MethodGet, apiBasePath+"/field", nil, nil, &out); err != nil {
		return nil, err
	}
	fields := make([]jira.Field, 0, len(out))
	for _, dto := range out {
		fields = append(fields, dto.toDomain())
	}
	return fields, nil
}

// CreateField creates a custom field.
func (c *Client) CreateField(ctx context.Context, in jira.NewFieldInput) (jira.Field, error) {
	payload := map[string]any{"name": in.Name, "type": in.Type}
	if in.Description != "" {
		payload["description"] = in.Description
	}
	if in.SearcherKey != "" {
		payload["searcherKey"] = in.SearcherKey
	}

	var dto fieldDTO
	if _, err := c.do(ctx, http.MethodPost, apiBasePath+"/field", nil, payload, &dto); err != nil {
		return jira.Field{}, err
	}
	return dto.toDomain(), nil
}

// DeleteField moves a custom field to the trash.
func (c *Client) DeleteField(ctx context.Context, fieldID string) error {
	path := apiBasePath + "/field/" + url.PathEscape(fieldID)
	status, err := c.do(ctx, http.MethodDelete, path, nil, nil, nil)
	return notFound(status, err, jira.ErrFieldNotFound, fieldID)
}

// RestoreField restores a custom field from the trash.
func (c *Client) RestoreField(ctx context.Context, fieldID string) error {
	path := apiBasePath + "/field/" + url.PathEscape(fieldID) + "/restore"
	status, err := c.do(ctx, http.MethodPost, path, nil, nil, nil)
	return notFound(status, err, jira.ErrFieldNotFound, fieldID)
}
