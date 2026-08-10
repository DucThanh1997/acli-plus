package jirarest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	jira "acli-plus/internal/domain/jira"
)

// defaultSearchFields is what a JQL search requests when the caller names no
// fields. The endpoint that replaced /rest/api/3/search returns only ids unless
// fields are listed, so an empty list would print blank rows.
var defaultSearchFields = []string{
	"summary", "status", "issuetype", "assignee", "reporter", "priority",
	"project", "parent", "labels", "resolution", "created", "updated", "duedate",
}

func issuePath(key string, suffix ...string) string {
	path := apiBasePath + "/issue/" + url.PathEscape(key)
	if len(suffix) > 0 {
		path += "/" + strings.Join(suffix, "/")
	}
	return path
}

// GetWorkItem fetches one work item.
func (c *Client) GetWorkItem(ctx context.Context, key string, fields []string) (jira.WorkItem, error) {
	query := url.Values{}
	if len(fields) > 0 {
		query.Set("fields", strings.Join(fields, ","))
	}

	var raw json.RawMessage
	status, err := c.do(ctx, http.MethodGet, issuePath(key), query, nil, &raw)
	if err != nil {
		return jira.WorkItem{}, notFound(status, err, jira.ErrWorkItemNotFound, key)
	}
	return decodeWorkItem(raw)
}

// CreateWorkItem creates one work item.
func (c *Client) CreateWorkItem(ctx context.Context, in jira.NewWorkItemInput) (jira.WorkItem, error) {
	body := map[string]any{"fields": in.Fields}
	var out struct {
		ID  string `json:"id"`
		Key string `json:"key"`
	}
	if _, err := c.do(ctx, http.MethodPost, apiBasePath+"/issue", nil, body, &out); err != nil {
		return jira.WorkItem{}, err
	}
	return jira.WorkItem{ID: out.ID, Key: out.Key}, nil
}

// CreateWorkItemsBulk creates several work items in one call. Jira caps a bulk
// request at 50 items; the application layer chunks larger inputs.
func (c *Client) CreateWorkItemsBulk(ctx context.Context, inputs []jira.NewWorkItemInput) ([]jira.WorkItem, error) {
	updates := make([]map[string]any, 0, len(inputs))
	for _, in := range inputs {
		updates = append(updates, map[string]any{"fields": in.Fields})
	}

	var out struct {
		Issues []struct {
			ID  string `json:"id"`
			Key string `json:"key"`
		} `json:"issues"`
		Errors []struct {
			FailedElementNumber int `json:"failedElementNumber"`
			ElementErrors       struct {
				ErrorMessages []string          `json:"errorMessages"`
				Errors        map[string]string `json:"errors"`
			} `json:"elementErrors"`
		} `json:"errors"`
	}
	body := map[string]any{"issueUpdates": updates}
	if _, err := c.do(ctx, http.MethodPost, apiBasePath+"/issue/bulk", nil, body, &out); err != nil {
		return nil, err
	}

	items := make([]jira.WorkItem, 0, len(out.Issues))
	for _, issue := range out.Issues {
		items = append(items, jira.WorkItem{ID: issue.ID, Key: issue.Key})
	}
	// A bulk call reports per-item failures with a 201, so surface them rather
	// than letting a partial success look complete.
	if len(out.Errors) > 0 {
		messages := make([]string, 0, len(out.Errors))
		for _, failure := range out.Errors {
			detail := strings.Join(failure.ElementErrors.ErrorMessages, "; ")
			for field, message := range failure.ElementErrors.Errors {
				detail = strings.TrimSpace(detail + " " + field + ": " + message)
			}
			messages = append(messages, fmt.Sprintf("item %d: %s", failure.FailedElementNumber+1, detail))
		}
		return items, fmt.Errorf("%d of %d work items failed: %s",
			len(out.Errors), len(inputs), strings.Join(messages, " | "))
	}
	return items, nil
}

// UpdateWorkItem overwrites fields on an existing work item.
func (c *Client) UpdateWorkItem(ctx context.Context, in jira.EditWorkItemInput) error {
	query := url.Values{"notifyUsers": {strconv.FormatBool(in.Notify)}}
	body := map[string]any{"fields": in.Fields}
	status, err := c.do(ctx, http.MethodPut, issuePath(in.Key), query, body, nil)
	return notFound(status, err, jira.ErrWorkItemNotFound, in.Key)
}

// DeleteWorkItem permanently deletes a work item. Jira refuses to delete a
// parent that still has subtasks unless withSubtasks is set.
func (c *Client) DeleteWorkItem(ctx context.Context, key string, withSubtasks bool) error {
	query := url.Values{"deleteSubtasks": {strconv.FormatBool(withSubtasks)}}
	status, err := c.do(ctx, http.MethodDelete, issuePath(key), query, nil, nil)
	return notFound(status, err, jira.ErrWorkItemNotFound, key)
}

// Search runs a JQL query against the token-paginated search endpoint. The
// older /rest/api/3/search was removed and now answers 410, so this is the only
// supported path; note it reports no total, only whether a next page exists.
func (c *Client) Search(ctx context.Context, in jira.SearchInput) (jira.SearchPage, error) {
	fields := in.Fields
	if len(fields) == 0 {
		fields = defaultSearchFields
	}

	body := map[string]any{"jql": in.JQL, "fields": fields}
	if in.MaxResults > 0 {
		body["maxResults"] = in.MaxResults
	}
	if in.PageToken != "" {
		body["nextPageToken"] = in.PageToken
	}
	if len(in.Expand) > 0 {
		body["expand"] = strings.Join(in.Expand, ",")
	}

	var out struct {
		Issues        []json.RawMessage `json:"issues"`
		NextPageToken string            `json:"nextPageToken"`
	}
	if _, err := c.do(ctx, http.MethodPost, apiBasePath+"/search/jql", nil, body, &out); err != nil {
		return jira.SearchPage{}, err
	}

	items, err := decodeWorkItems(out.Issues)
	if err != nil {
		return jira.SearchPage{}, err
	}
	return jira.SearchPage{Items: items, NextPageToken: out.NextPageToken}, nil
}

// ArchiveWorkItems archives work items. Archiving is a Premium and Enterprise
// feature, so a site without it answers 404 for the endpoint itself.
func (c *Client) ArchiveWorkItems(ctx context.Context, keys []string) error {
	return c.setArchived(ctx, "archive", keys)
}

// UnarchiveWorkItems restores archived work items.
func (c *Client) UnarchiveWorkItems(ctx context.Context, keys []string) error {
	return c.setArchived(ctx, "unarchive", keys)
}

func (c *Client) setArchived(ctx context.Context, action string, keys []string) error {
	var out struct {
		Errors *struct {
			IssuesNotFound []string `json:"issuesNotFound"`
			IssueIsSubtask []string `json:"issueIsSubtask"`
		} `json:"errors"`
	}
	body := map[string]any{"issueIdsOrKeys": keys}
	status, err := c.do(ctx, http.MethodPut, apiBasePath+"/issue/"+action, nil, body, &out)
	if status == http.StatusNotFound {
		return fmt.Errorf("%w: %s requires Jira Premium or Enterprise", jira.ErrNotLicensed, action)
	}
	if err != nil {
		return err
	}
	if out.Errors != nil {
		if missing := out.Errors.IssuesNotFound; len(missing) > 0 {
			return fmt.Errorf("%w: %s", jira.ErrWorkItemNotFound, strings.Join(missing, ", "))
		}
		if subtasks := out.Errors.IssueIsSubtask; len(subtasks) > 0 {
			return fmt.Errorf("cannot %s a subtask directly: %s", action, strings.Join(subtasks, ", "))
		}
	}
	return nil
}

// AssignWorkItem sets the assignee. An empty accountID clears it; the literal
// "-1" hands the work item to the project's default assignee.
func (c *Client) AssignWorkItem(ctx context.Context, key, accountID string) error {
	body := map[string]any{"accountId": nil}
	if accountID != "" {
		body["accountId"] = accountID
	}
	status, err := c.do(ctx, http.MethodPut, issuePath(key, "assignee"), nil, body, nil)
	return notFound(status, err, jira.ErrWorkItemNotFound, key)
}

// ListTransitions returns the transitions available from the current status.
func (c *Client) ListTransitions(ctx context.Context, key string) ([]jira.Transition, error) {
	var out struct {
		Transitions []transitionDTO `json:"transitions"`
	}
	status, err := c.do(ctx, http.MethodGet, issuePath(key, "transitions"), nil, nil, &out)
	if err != nil {
		return nil, notFound(status, err, jira.ErrWorkItemNotFound, key)
	}
	transitions := make([]jira.Transition, 0, len(out.Transitions))
	for _, dto := range out.Transitions {
		transitions = append(transitions, dto.toDomain())
	}
	return transitions, nil
}

// TransitionWorkItem performs a transition, optionally setting fields the
// transition screen requires (for example a resolution when closing).
func (c *Client) TransitionWorkItem(ctx context.Context, key, transitionID string, fields jira.FieldValues) error {
	body := map[string]any{"transition": map[string]string{"id": transitionID}}
	if len(fields) > 0 {
		body["fields"] = fields
	}
	status, err := c.do(ctx, http.MethodPost, issuePath(key, "transitions"), nil, body, nil)
	return notFound(status, err, jira.ErrWorkItemNotFound, key)
}

// ListWatchers returns the accounts watching a work item.
func (c *Client) ListWatchers(ctx context.Context, key string) ([]jira.User, error) {
	var out struct {
		Watchers []userDTO `json:"watchers"`
	}
	status, err := c.do(ctx, http.MethodGet, issuePath(key, "watchers"), nil, nil, &out)
	if err != nil {
		return nil, notFound(status, err, jira.ErrWorkItemNotFound, key)
	}
	watchers := make([]jira.User, 0, len(out.Watchers))
	for _, dto := range out.Watchers {
		watchers = append(watchers, dto.toDomain())
	}
	return watchers, nil
}

// RemoveWatcher removes one watcher from a work item.
func (c *Client) RemoveWatcher(ctx context.Context, key, accountID string) error {
	query := url.Values{"accountId": {accountID}}
	status, err := c.do(ctx, http.MethodDelete, issuePath(key, "watchers"), query, nil, nil)
	return notFound(status, err, jira.ErrWorkItemNotFound, key)
}
