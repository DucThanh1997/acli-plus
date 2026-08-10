package jirarest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"

	jira "acli-plus/internal/domain/jira"
)

// ListComments returns every comment on a work item, oldest first.
func (c *Client) ListComments(ctx context.Context, key string) ([]jira.Comment, error) {
	var out struct {
		Comments []commentDTO `json:"comments"`
	}
	query := url.Values{"maxResults": {"100"}, "orderBy": {"created"}}
	status, err := c.do(ctx, http.MethodGet, issuePath(key, "comment"), query, nil, &out)
	if err != nil {
		return nil, notFound(status, err, jira.ErrWorkItemNotFound, key)
	}
	comments := make([]jira.Comment, 0, len(out.Comments))
	for _, dto := range out.Comments {
		comments = append(comments, dto.toDomain())
	}
	return comments, nil
}

// GetComment returns one comment along with its untouched ADF body, which
// comment-visibility needs so it can change the restriction without rewriting
// what the comment says.
func (c *Client) GetComment(ctx context.Context, key, commentID string) (jira.Comment, jira.Document, error) {
	var dto commentDTO
	path := issuePath(key, "comment", url.PathEscape(commentID))
	status, err := c.do(ctx, http.MethodGet, path, nil, nil, &dto)
	if err != nil {
		if status == http.StatusNotFound {
			return jira.Comment{}, nil, notFound(status, err, jira.ErrWorkItemNotFound, key+" comment "+commentID)
		}
		return jira.Comment{}, nil, err
	}
	return dto.toDomain(), jira.Document(dto.Body), nil
}

// CreateComment adds a comment to a work item.
func (c *Client) CreateComment(ctx context.Context, key string, body jira.Document, vis jira.CommentVisibility) (jira.Comment, error) {
	payload := map[string]any{"body": body}
	if vis.Set() {
		payload["visibility"] = visibilityPayload(vis)
	}

	var dto commentDTO
	status, err := c.do(ctx, http.MethodPost, issuePath(key, "comment"), nil, payload, &dto)
	if err != nil {
		return jira.Comment{}, notFound(status, err, jira.ErrWorkItemNotFound, key)
	}
	return dto.toDomain(), nil
}

// UpdateComment replaces a comment's body and visibility. Visibility is always
// sent, so passing a zero value clears an existing restriction.
func (c *Client) UpdateComment(ctx context.Context, key, commentID string, body jira.Document, vis jira.CommentVisibility) (jira.Comment, error) {
	payload := map[string]any{"body": body, "visibility": nil}
	if vis.Set() {
		payload["visibility"] = visibilityPayload(vis)
	}

	var dto commentDTO
	path := issuePath(key, "comment", url.PathEscape(commentID))
	status, err := c.do(ctx, http.MethodPut, path, nil, payload, &dto)
	if err != nil {
		return jira.Comment{}, notFound(status, err, jira.ErrWorkItemNotFound, key+" comment "+commentID)
	}
	return dto.toDomain(), nil
}

// DeleteComment removes a comment.
func (c *Client) DeleteComment(ctx context.Context, key, commentID string) error {
	path := issuePath(key, "comment", url.PathEscape(commentID))
	status, err := c.do(ctx, http.MethodDelete, path, nil, nil, nil)
	return notFound(status, err, jira.ErrWorkItemNotFound, key+" comment "+commentID)
}

func visibilityPayload(vis jira.CommentVisibility) map[string]string {
	return map[string]string{"type": vis.Type, "value": vis.Value}
}

// ListAttachments reads the attachment field off a work item; Jira has no
// dedicated per-issue attachment listing endpoint.
func (c *Client) ListAttachments(ctx context.Context, key string) ([]jira.Attachment, error) {
	var out struct {
		Fields struct {
			Attachment []attachmentDTO `json:"attachment"`
		} `json:"fields"`
	}
	query := url.Values{"fields": {"attachment"}}
	status, err := c.do(ctx, http.MethodGet, issuePath(key), query, nil, &out)
	if err != nil {
		return nil, notFound(status, err, jira.ErrWorkItemNotFound, key)
	}
	attachments := make([]jira.Attachment, 0, len(out.Fields.Attachment))
	for _, dto := range out.Fields.Attachment {
		attachments = append(attachments, dto.toDomain())
	}
	return attachments, nil
}

// DeleteAttachment removes one attachment by its id.
func (c *Client) DeleteAttachment(ctx context.Context, attachmentID string) error {
	path := apiBasePath + "/attachment/" + url.PathEscape(attachmentID)
	status, err := c.do(ctx, http.MethodDelete, path, nil, nil, nil)
	if status == http.StatusNotFound {
		return notFound(status, err, jira.ErrWorkItemNotFound, "attachment "+attachmentID)
	}
	return err
}

// ListLinkTypes returns the link types configured on the site.
func (c *Client) ListLinkTypes(ctx context.Context) ([]jira.LinkType, error) {
	var out struct {
		IssueLinkTypes []linkTypeDTO `json:"issueLinkTypes"`
	}
	if _, err := c.do(ctx, http.MethodGet, apiBasePath+"/issueLinkType", nil, nil, &out); err != nil {
		return nil, err
	}
	types := make([]jira.LinkType, 0, len(out.IssueLinkTypes))
	for _, dto := range out.IssueLinkTypes {
		types = append(types, dto.toDomain())
	}
	return types, nil
}

// CreateLink links two work items.
func (c *Client) CreateLink(ctx context.Context, in jira.NewLinkInput) error {
	payload := map[string]any{
		"type":         map[string]string{"name": in.Type},
		"inwardIssue":  map[string]string{"key": in.Inward},
		"outwardIssue": map[string]string{"key": in.Outward},
	}
	if in.Comment != "" {
		payload["comment"] = map[string]any{"body": adfComment(in.Comment)}
	}
	_, err := c.do(ctx, http.MethodPost, apiBasePath+"/issueLink", nil, payload, nil)
	return err
}

// adfComment wraps link commentary in a minimal ADF document.
func adfComment(text string) json.RawMessage {
	encoded, err := json.Marshal(map[string]any{
		"type":    "doc",
		"version": 1,
		"content": []any{map[string]any{
			"type":    "paragraph",
			"content": []any{map[string]any{"type": "text", "text": text}},
		}},
	})
	if err != nil {
		return json.RawMessage(`{"type":"doc","version":1,"content":[]}`)
	}
	return encoded
}
