package confluencerest

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	confluence "acli-plus/internal/domain/confluence"
)

// GetPage fetches a page by id, including its version number and message.
func (c *Client) GetPage(ctx context.Context, pageID string) (confluence.Page, error) {
	query := url.Values{"body-format": {"storage"}}
	var dto pageDTO
	status, err := c.do(ctx, http.MethodGet, "/pages/"+url.PathEscape(pageID), query, nil, &dto)
	if err != nil {
		if status == http.StatusNotFound {
			return confluence.Page{}, fmt.Errorf("%w: %s", confluence.ErrPageNotFound, pageID)
		}
		return confluence.Page{}, err
	}
	return dto.toDomain(), nil
}

// FindPageByTitle finds a page by exact title within a space (titles are unique
// per space). The boolean reports whether a match was found.
func (c *Client) FindPageByTitle(ctx context.Context, spaceID, title string) (confluence.Page, bool, error) {
	query := url.Values{
		"space-id": {spaceID},
		"title":    {title},
		"limit":    {"1"},
	}
	var out struct {
		Results []pageDTO `json:"results"`
	}
	if _, err := c.do(ctx, http.MethodGet, "/pages", query, nil, &out); err != nil {
		return confluence.Page{}, false, err
	}
	if len(out.Results) == 0 {
		return confluence.Page{}, false, nil
	}
	return out.Results[0].toDomain(), true, nil
}

// CreatePage creates a new page (version 1) and returns it.
func (c *Client) CreatePage(ctx context.Context, in confluence.NewPageInput) (confluence.Page, error) {
	request := createPageRequest{
		SpaceID:  in.SpaceID,
		Status:   "current",
		Title:    in.Title,
		ParentID: in.ParentID,
		Body:     bodyDTO{Representation: "storage", Value: in.Body},
	}
	var dto pageDTO
	if _, err := c.do(ctx, http.MethodPost, "/pages", nil, request, &dto); err != nil {
		return confluence.Page{}, err
	}
	return dto.toDomain(), nil
}

// UpdatePage overwrites an existing page in place and returns the new state.
func (c *Client) UpdatePage(ctx context.Context, in confluence.UpdatePageInput) (confluence.Page, error) {
	request := updatePageRequest{
		ID:      in.ID,
		Status:  "current",
		Title:   in.Title,
		Body:    bodyDTO{Representation: "storage", Value: in.Body},
		Version: versionRequest{Number: in.NextVersion, Message: in.Message},
	}
	var dto pageDTO
	status, err := c.do(ctx, http.MethodPut, "/pages/"+url.PathEscape(in.ID), nil, request, &dto)
	if err != nil {
		if status == http.StatusNotFound {
			return confluence.Page{}, fmt.Errorf("%w: %s", confluence.ErrPageNotFound, in.ID)
		}
		return confluence.Page{}, err
	}
	return dto.toDomain(), nil
}

// DeletePage moves a page to the trash (a reversible soft delete).
func (c *Client) DeletePage(ctx context.Context, pageID string) error {
	status, err := c.do(ctx, http.MethodDelete, "/pages/"+url.PathEscape(pageID), nil, nil, nil)
	if err != nil {
		if status == http.StatusNotFound {
			return fmt.Errorf("%w: %s", confluence.ErrPageNotFound, pageID)
		}
		return err
	}
	return nil
}
