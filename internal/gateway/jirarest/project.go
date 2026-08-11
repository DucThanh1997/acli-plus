package jirarest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"

	jira "acli-plus/internal/domain/jira"
)

func projectPath(keyOrID string, suffix ...string) string {
	path := apiBasePath + "/project/" + url.PathEscape(keyOrID)
	for _, part := range suffix {
		path += "/" + part
	}
	return path
}

// ListProjects returns the projects visible to the caller.
func (c *Client) ListProjects(ctx context.Context, q jira.ProjectQuery) ([]jira.Project, error) {
	query := url.Values{"expand": {"description,lead"}}
	if q.Query != "" {
		query.Set("query", q.Query)
	}
	if q.TypeKey != "" {
		query.Set("typeKey", q.TypeKey)
	}
	if q.Status != "" {
		query.Set("status", q.Status)
	}

	raws, _, err := c.pagedValues(ctx, apiBasePath+"/project/search", query, q.MaxResults)
	if err != nil {
		return nil, err
	}
	return decodeEach(raws, func(dto projectDTO, raw json.RawMessage) jira.Project {
		return dto.toDomain(c.host, raw)
	})
}

// GetProject fetches one project by key or id.
func (c *Client) GetProject(ctx context.Context, keyOrID string) (jira.Project, error) {
	var raw json.RawMessage
	query := url.Values{"expand": {"description,lead"}}
	status, err := c.do(ctx, http.MethodGet, projectPath(keyOrID), query, nil, &raw)
	if err != nil {
		return jira.Project{}, notFound(status, err, jira.ErrProjectNotFound, keyOrID)
	}
	var dto projectDTO
	if err := json.Unmarshal(raw, &dto); err != nil {
		return jira.Project{}, err
	}
	return dto.toDomain(c.host, raw), nil
}

// CreateProject creates a project.
func (c *Client) CreateProject(ctx context.Context, in jira.NewProjectInput) (jira.Project, error) {
	payload := map[string]any{
		"key":            in.Key,
		"name":           in.Name,
		"leadAccountId":  in.LeadAccountID,
		"projectTypeKey": in.TypeKey,
	}
	if in.TemplateKey != "" {
		payload["projectTemplateKey"] = in.TemplateKey
	}
	if in.Description != "" {
		payload["description"] = in.Description
	}
	if in.AssigneeType != "" {
		payload["assigneeType"] = in.AssigneeType
	}

	var out struct {
		ID   flexID `json:"id"`
		Key  string `json:"key"`
		Self string `json:"self"`
	}
	if _, err := c.do(ctx, http.MethodPost, apiBasePath+"/project", nil, payload, &out); err != nil {
		return jira.Project{}, err
	}

	key := out.Key
	if key == "" {
		key = in.Key
	}
	return jira.Project{
		ID:   string(out.ID),
		Key:  key,
		Name: in.Name,
		URL:  "https://" + c.host + "/browse/" + key,
	}, nil
}

// UpdateProject changes a project's mutable attributes, sending only the ones
// the caller set so untouched attributes keep their current values.
func (c *Client) UpdateProject(ctx context.Context, in jira.UpdateProjectInput) (jira.Project, error) {
	payload := map[string]any{}
	for key, value := range map[string]string{
		"key":           in.NewKey,
		"name":          in.Name,
		"description":   in.Description,
		"leadAccountId": in.LeadAccountID,
		"assigneeType":  in.AssigneeType,
	} {
		if value != "" {
			payload[key] = value
		}
	}
	if len(payload) == 0 {
		return jira.Project{}, errNothingToUpdate
	}

	var raw json.RawMessage
	status, err := c.do(ctx, http.MethodPut, projectPath(in.KeyOrID), nil, payload, &raw)
	if err != nil {
		return jira.Project{}, notFound(status, err, jira.ErrProjectNotFound, in.KeyOrID)
	}
	var dto projectDTO
	if err := json.Unmarshal(raw, &dto); err != nil {
		return jira.Project{}, err
	}
	return dto.toDomain(c.host, raw), nil
}

// DeleteProject deletes a project. With enableUndo the project goes to the
// trash and can be restored; without it the deletion is immediate.
func (c *Client) DeleteProject(ctx context.Context, keyOrID string, enableUndo bool) error {
	query := url.Values{"enableUndo": {strconv.FormatBool(enableUndo)}}
	status, err := c.do(ctx, http.MethodDelete, projectPath(keyOrID), query, nil, nil)
	return notFound(status, err, jira.ErrProjectNotFound, keyOrID)
}

// ArchiveProject archives a project (Premium and Enterprise plans).
func (c *Client) ArchiveProject(ctx context.Context, keyOrID string) error {
	status, err := c.do(ctx, http.MethodPost, projectPath(keyOrID, "archive"), nil, nil, nil)
	return notFound(status, err, jira.ErrProjectNotFound, keyOrID)
}

// RestoreProject restores an archived or trashed project.
func (c *Client) RestoreProject(ctx context.Context, keyOrID string) error {
	status, err := c.do(ctx, http.MethodPost, projectPath(keyOrID, "restore"), nil, nil, nil)
	return notFound(status, err, jira.ErrProjectNotFound, keyOrID)
}
