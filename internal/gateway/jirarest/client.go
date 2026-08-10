// Package jirarest is the Jira Cloud REST adapter. It implements the ports in
// internal/domain/jira over two API bases: /rest/api/3 for the platform and
// /rest/agile/1.0 for boards and sprints.
package jirarest

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	jira "acli-plus/internal/domain/jira"
)

const (
	apiBasePath    = "/rest/api/3"
	agileBasePath  = "/rest/agile/1.0"
	defaultTimeout = 60 * time.Second
)

// Client is the Jira Cloud REST adapter.
type Client struct {
	host       string
	baseURL    string
	authHeader string
	http       *http.Client
}

// Compile-time check that the adapter satisfies the whole domain port.
var _ jira.Gateway = (*Client)(nil)

// New builds a client for a host using Basic auth (email:token) — the same
// credential pair Confluence uses, because both live on one Atlassian site.
func New(host, email, token string) *Client {
	encoded := base64.StdEncoding.EncodeToString([]byte(email + ":" + token))
	return &Client{
		host:       host,
		baseURL:    "https://" + host,
		authHeader: "Basic " + encoded,
		http:       &http.Client{Timeout: defaultTimeout},
	}
}

// Host returns the site this client talks to (used to build browse links).
func (c *Client) Host() string { return c.host }

// do executes a request and decodes a JSON response into out (when non-nil).
// It returns the HTTP status alongside any error so callers can map 404 onto
// the right domain error for the resource they asked for.
func (c *Client) do(ctx context.Context, method, path string, query url.Values, body, out any) (int, error) {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return 0, fmt.Errorf("encoding request: %w", err)
		}
		reader = bytes.NewReader(encoded)
	}

	endpoint := c.baseURL + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return 0, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Authorization", c.authHeader)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, fmt.Errorf("calling jira: %w", err)
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, fmt.Errorf("reading response: %w", err)
	}

	switch {
	case resp.StatusCode == http.StatusUnauthorized, resp.StatusCode == http.StatusForbidden:
		return resp.StatusCode, fmt.Errorf("%w: %s", jira.ErrAuth, apiMessage(payload))
	case resp.StatusCode >= http.StatusBadRequest:
		return resp.StatusCode, apiError(resp.StatusCode, payload)
	}

	if out != nil && len(payload) > 0 {
		if err := json.Unmarshal(payload, out); err != nil {
			return resp.StatusCode, fmt.Errorf("decoding response: %w", err)
		}
	}
	return resp.StatusCode, nil
}

// apiError renders a Jira error envelope into a readable error. Jira reports
// problems two ways: a list of general messages and a field-keyed map.
func apiError(status int, payload []byte) error {
	if message := apiMessage(payload); message != "" {
		return fmt.Errorf("jira api %d: %s", status, message)
	}
	return fmt.Errorf("jira api %d", status)
}

// apiMessage extracts the human-readable part of a Jira error envelope.
func apiMessage(payload []byte) string {
	var envelope struct {
		ErrorMessages []string          `json:"errorMessages"`
		Errors        map[string]string `json:"errors"`
		Message       string            `json:"message"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return strings.TrimSpace(string(payload))
	}

	parts := append([]string(nil), envelope.ErrorMessages...)
	for field, message := range envelope.Errors {
		parts = append(parts, field+": "+message)
	}
	if len(parts) == 0 && envelope.Message != "" {
		parts = append(parts, envelope.Message)
	}
	return strings.Join(parts, "; ")
}

// notFound maps a 404 onto the domain error for the resource being fetched,
// leaving every other error untouched.
func notFound(status int, err error, sentinel error, name string) error {
	if err == nil {
		return nil
	}
	if status == http.StatusNotFound {
		return fmt.Errorf("%w: %s", sentinel, name)
	}
	return err
}

// VerifyAuth performs a cheap authenticated call to confirm the credentials
// work against Jira on this site.
func (c *Client) VerifyAuth(ctx context.Context) error {
	_, err := c.do(ctx, http.MethodGet, apiBasePath+"/myself", nil, nil, nil)
	return err
}

// CurrentUser returns the account the credentials belong to.
func (c *Client) CurrentUser(ctx context.Context) (jira.User, error) {
	var out userDTO
	if _, err := c.do(ctx, http.MethodGet, apiBasePath+"/myself", nil, nil, &out); err != nil {
		return jira.User{}, err
	}
	return out.toDomain(), nil
}

// FindUsers searches accounts by display name or email. Sites that hide email
// addresses in their privacy settings will only match on display name.
func (c *Client) FindUsers(ctx context.Context, query string) ([]jira.User, error) {
	values := url.Values{"query": {query}, "maxResults": {"50"}}
	var out []userDTO
	if _, err := c.do(ctx, http.MethodGet, apiBasePath+"/user/search", values, nil, &out); err != nil {
		return nil, err
	}
	users := make([]jira.User, 0, len(out))
	for _, dto := range out {
		users = append(users, dto.toDomain())
	}
	return users, nil
}
