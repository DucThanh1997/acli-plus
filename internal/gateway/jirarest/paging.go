package jirarest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
)

// defaultPageLimit caps how many rows a listing returns when the caller sets no
// limit, so a site with thousands of projects does not scroll off the screen.
const defaultPageLimit = 50

// pageSize is what each HTTP request asks for while walking a listing.
const pageSize = 50

// pagedValues walks a startAt/isLast paginated endpoint and collects raw rows
// until it has limit of them or the API reports the last page. Jira's list
// endpoints share this envelope, so every listing here pages the same way
// instead of silently returning only the first page.
//
// The HTTP status of the failing request is returned alongside the error so
// callers can map it onto the right domain error, the same way the single-shot
// requests do.
func (c *Client) pagedValues(ctx context.Context, path string, query url.Values, limit int) ([]json.RawMessage, int, error) {
	if limit <= 0 {
		limit = defaultPageLimit
	}
	if query == nil {
		query = url.Values{}
	}

	collected := make([]json.RawMessage, 0, limit)
	for startAt := 0; len(collected) < limit; {
		page := query
		page.Set("startAt", strconv.Itoa(startAt))
		page.Set("maxResults", strconv.Itoa(min(pageSize, limit-len(collected))))

		var out struct {
			Values  []json.RawMessage `json:"values"`
			IsLast  bool              `json:"isLast"`
			Total   int               `json:"total"`
			MaxRes  int               `json:"maxResults"`
			StartAt int               `json:"startAt"`
		}
		status, err := c.do(ctx, http.MethodGet, path, page, nil, &out)
		if err != nil {
			return nil, status, err
		}

		collected = append(collected, out.Values...)
		startAt += len(out.Values)
		// isLast is authoritative where present; the length check covers the
		// endpoints that omit it and simply return a short final page.
		if out.IsLast || len(out.Values) == 0 || (out.Total > 0 && startAt >= out.Total) {
			break
		}
	}

	if len(collected) > limit {
		collected = collected[:limit]
	}
	return collected, http.StatusOK, nil
}

// decodeEach unmarshals raw rows into T via the supplied converter.
func decodeEach[W any, D any](raws []json.RawMessage, convert func(W, json.RawMessage) D) ([]D, error) {
	out := make([]D, 0, len(raws))
	for _, raw := range raws {
		var wire W
		if err := json.Unmarshal(raw, &wire); err != nil {
			return nil, err
		}
		out = append(out, convert(wire, raw))
	}
	return out, nil
}
