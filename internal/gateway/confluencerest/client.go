package confluencerest

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	confluence "acli-plus/internal/domain/confluence"
)

const (
	apiBasePath    = "/wiki/api/v2"
	defaultTimeout = 30 * time.Second
)

// Client is the Confluence Cloud REST (API v2) adapter.
type Client struct {
	baseURL    string
	authHeader string
	http       *http.Client
}

// Compile-time check that the adapter satisfies the domain port.
var _ confluence.Gateway = (*Client)(nil)

// New builds a client for a host using Basic auth (email:token).
func New(host, email, token string) *Client {
	encoded := base64.StdEncoding.EncodeToString([]byte(email + ":" + token))
	return &Client{
		baseURL:    "https://" + host + apiBasePath,
		authHeader: "Basic " + encoded,
		http:       &http.Client{Timeout: defaultTimeout},
	}
}

// do executes a request and decodes a JSON response into out (when non-nil).
// It returns the HTTP status alongside any error so callers can map 404/401.
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
		return 0, fmt.Errorf("calling confluence: %w", err)
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, fmt.Errorf("reading response: %w", err)
	}

	switch {
	case resp.StatusCode == http.StatusUnauthorized, resp.StatusCode == http.StatusForbidden:
		return resp.StatusCode, confluence.ErrAuth
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

// apiError renders a Confluence error envelope into a readable error.
func apiError(status int, payload []byte) error {
	var envelope struct {
		Errors []struct {
			Title  string `json:"title"`
			Detail string `json:"detail"`
		} `json:"errors"`
		Message string `json:"message"`
	}
	_ = json.Unmarshal(payload, &envelope)

	switch {
	case len(envelope.Errors) > 0 && envelope.Errors[0].Title != "":
		return fmt.Errorf("confluence api %d: %s", status, envelope.Errors[0].Title)
	case envelope.Message != "":
		return fmt.Errorf("confluence api %d: %s", status, envelope.Message)
	default:
		return fmt.Errorf("confluence api %d", status)
	}
}

// VerifyAuth performs a cheap authenticated call to confirm credentials work.
func (c *Client) VerifyAuth(ctx context.Context) error {
	_, err := c.do(ctx, http.MethodGet, "/spaces", url.Values{"limit": {"1"}}, nil, nil)
	return err
}

// ResolveSpaceID maps a space key to its numeric id.
func (c *Client) ResolveSpaceID(ctx context.Context, spaceKey string) (string, error) {
	query := url.Values{"keys": {spaceKey}, "limit": {"1"}}
	var out struct {
		Results []struct {
			ID  flexID `json:"id"`
			Key string `json:"key"`
		} `json:"results"`
	}
	if _, err := c.do(ctx, http.MethodGet, "/spaces", query, nil, &out); err != nil {
		return "", err
	}
	if len(out.Results) == 0 {
		return "", fmt.Errorf("%w: %s", confluence.ErrSpaceNotFound, spaceKey)
	}
	return string(out.Results[0].ID), nil
}
