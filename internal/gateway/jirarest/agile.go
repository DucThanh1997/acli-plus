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

// SearchBoards returns the Agile boards matching the query.
func (c *Client) SearchBoards(ctx context.Context, q jira.BoardQuery) ([]jira.Board, error) {
	query := url.Values{}
	if q.Name != "" {
		query.Set("name", q.Name)
	}
	if q.ProjectKey != "" {
		query.Set("projectKeyOrId", q.ProjectKey)
	}
	if q.Type != "" {
		query.Set("type", q.Type)
	}

	raws, _, err := c.pagedValues(ctx, agileBasePath+"/board", query, q.MaxResults)
	if err != nil {
		return nil, err
	}
	return decodeEach(raws, func(dto boardDTO, _ json.RawMessage) jira.Board {
		return dto.toDomain(c.host)
	})
}

// ListSprints returns the sprints on a board, optionally filtered by state
// (future, active, closed).
func (c *Client) ListSprints(ctx context.Context, boardID int, state string) ([]jira.Sprint, error) {
	query := url.Values{}
	if state != "" {
		query.Set("state", state)
	}

	path := agileBasePath + "/board/" + strconv.Itoa(boardID) + "/sprint"
	raws, status, err := c.pagedValues(ctx, path, query, 0)
	if err != nil {
		switch {
		case status == http.StatusNotFound:
			return nil, fmt.Errorf("%w: %d", jira.ErrBoardNotFound, boardID)
		// A Kanban board has no sprint concept at all, and the Agile API says so
		// with a 400 rather than an empty list. Name the reason instead of
		// passing a raw API error to the user.
		case status == http.StatusBadRequest && strings.Contains(err.Error(), "does not support sprints"):
			return nil, fmt.Errorf("%w: board %d", jira.ErrBoardHasNoSprints, boardID)
		}
		return nil, err
	}

	sprints, err := decodeEach(raws, func(dto sprintDTO, _ json.RawMessage) jira.Sprint {
		return dto.toDomain()
	})
	if err != nil {
		return nil, err
	}
	// originBoardId is absent on some responses; fill it so the caller always
	// knows which board a sprint came from.
	for i := range sprints {
		if sprints[i].BoardID == 0 {
			sprints[i].BoardID = boardID
		}
	}
	return sprints, nil
}

// ListSprintWorkItems returns the work items in a sprint. The Agile API pages
// with startAt over an "issues" array rather than the platform's "values".
func (c *Client) ListSprintWorkItems(ctx context.Context, sprintID int, jql string, fields []string) ([]jira.WorkItem, error) {
	if len(fields) == 0 {
		fields = defaultSearchFields
	}
	path := agileBasePath + "/sprint/" + strconv.Itoa(sprintID) + "/issue"

	collected := make([]jira.WorkItem, 0, pageSize)
	for startAt := 0; ; {
		query := url.Values{
			"startAt":    {strconv.Itoa(startAt)},
			"maxResults": {strconv.Itoa(pageSize)},
			"fields":     {strings.Join(fields, ",")},
		}
		if jql != "" {
			query.Set("jql", jql)
		}

		var out struct {
			Issues []json.RawMessage `json:"issues"`
			Total  int               `json:"total"`
		}
		status, err := c.do(ctx, http.MethodGet, path, query, nil, &out)
		if err != nil {
			return nil, notFound(status, err, jira.ErrSprintNotFound, strconv.Itoa(sprintID))
		}

		items, err := decodeWorkItems(out.Issues)
		if err != nil {
			return nil, err
		}
		collected = append(collected, items...)

		startAt += len(out.Issues)
		if len(out.Issues) == 0 || startAt >= out.Total {
			return collected, nil
		}
	}
}
