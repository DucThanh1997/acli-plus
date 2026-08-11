package jirarest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"

	jira "acli-plus/internal/domain/jira"
)

// filterExpand asks for the parts of a filter the listings print; without it
// Jira returns only ids and names.
const filterExpand = "jql,owner,viewUrl,sharePermissions,favourite"

// SearchFilters searches saved filters across the site.
func (c *Client) SearchFilters(ctx context.Context, q jira.FilterQuery) ([]jira.Filter, error) {
	query := url.Values{"expand": {filterExpand}}
	if q.Name != "" {
		query.Set("filterName", q.Name)
	}
	if q.AccountID != "" {
		query.Set("accountId", q.AccountID)
	}

	raws, _, err := c.pagedValues(ctx, apiBasePath+"/filter/search", query, q.MaxResults)
	if err != nil {
		return nil, err
	}
	return decodeEach(raws, func(dto filterDTO, _ json.RawMessage) jira.Filter {
		return dto.toDomain()
	})
}

// ListMyFilters returns the caller's own filters, optionally narrowed to the
// ones they marked as favourites.
func (c *Client) ListMyFilters(ctx context.Context, favouritesOnly bool) ([]jira.Filter, error) {
	query := url.Values{"expand": {filterExpand}, "includeFavourites": {"true"}}
	var out []filterDTO
	if _, err := c.do(ctx, http.MethodGet, apiBasePath+"/filter/my", query, nil, &out); err != nil {
		return nil, err
	}

	filters := make([]jira.Filter, 0, len(out))
	for _, dto := range out {
		filter := dto.toDomain()
		if favouritesOnly && !filter.Favourite {
			continue
		}
		filters = append(filters, filter)
	}
	return filters, nil
}

// AddFilterFavourite marks a filter as a favourite for the current user.
func (c *Client) AddFilterFavourite(ctx context.Context, filterID string) error {
	path := apiBasePath + "/filter/" + url.PathEscape(filterID) + "/favourite"
	query := url.Values{"expand": {filterExpand}}
	status, err := c.do(ctx, http.MethodPut, path, query, nil, nil)
	return notFound(status, err, jira.ErrWorkItemNotFound, "filter "+filterID)
}

// ChangeFilterOwner transfers a filter to another account.
func (c *Client) ChangeFilterOwner(ctx context.Context, filterID, accountID string) error {
	path := apiBasePath + "/filter/" + url.PathEscape(filterID) + "/owner"
	body := map[string]string{"accountId": accountID}
	status, err := c.do(ctx, http.MethodPut, path, nil, body, nil)
	return notFound(status, err, jira.ErrWorkItemNotFound, "filter "+filterID)
}

// SearchDashboards searches dashboards across the site.
func (c *Client) SearchDashboards(ctx context.Context, q jira.DashboardQuery) ([]jira.Dashboard, error) {
	query := url.Values{}
	if q.Name != "" {
		query.Set("dashboardName", q.Name)
	}

	raws, _, err := c.pagedValues(ctx, apiBasePath+"/dashboard/search", query, q.MaxResults)
	if err != nil {
		return nil, err
	}
	return decodeEach(raws, func(dto dashboardDTO, _ json.RawMessage) jira.Dashboard {
		return dto.toDomain()
	})
}

// verify the adapter keeps satisfying the narrow ports as they evolve.
var (
	_ jira.WorkItemGateway   = (*Client)(nil)
	_ jira.CommentGateway    = (*Client)(nil)
	_ jira.AttachmentGateway = (*Client)(nil)
	_ jira.LinkGateway       = (*Client)(nil)
	_ jira.ProjectGateway    = (*Client)(nil)
	_ jira.FieldGateway      = (*Client)(nil)
	_ jira.AgileGateway      = (*Client)(nil)
	_ jira.ViewGateway       = (*Client)(nil)
	_ jira.AccountGateway    = (*Client)(nil)
)
