package jira

import (
	"context"
	"encoding/json"
)

// Document is a rich-text value in Atlassian Document Format (ADF), the shape
// the v3 REST API requires for descriptions and comments. The domain treats it
// as opaque; internal/markdown produces it and the gateway forwards it.
type Document json.RawMessage

// MarshalJSON emits the document verbatim, or an empty ADF doc when unset, so a
// zero Document never produces invalid JSON in a request body.
func (d Document) MarshalJSON() ([]byte, error) {
	if len(d) == 0 {
		return []byte(`{"type":"doc","version":1,"content":[]}`), nil
	}
	return d, nil
}

// Empty reports whether the document carries no content.
func (d Document) Empty() bool { return len(d) == 0 }

// ProjectQuery filters a project listing.
type ProjectQuery struct {
	Query      string // matches key or name
	TypeKey    string // software | service_desk | business
	Status     string // live | archived | deleted
	MaxResults int
}

// BoardQuery filters a board search.
type BoardQuery struct {
	Name       string
	ProjectKey string
	Type       string // scrum | kanban
	MaxResults int
}

// FilterQuery filters a saved-filter search.
type FilterQuery struct {
	Name       string
	AccountID  string
	MaxResults int
}

// DashboardQuery filters a dashboard search.
type DashboardQuery struct {
	Name       string
	MaxResults int
}

// AccountGateway covers whole-site calls that are not tied to one entity.
type AccountGateway interface {
	// VerifyAuth checks that the stored credentials are accepted by Jira.
	VerifyAuth(ctx context.Context) error
	// CurrentUser returns the account the credentials belong to (for "@me").
	CurrentUser(ctx context.Context) (User, error)
	// FindUsers searches accounts by display name or email.
	FindUsers(ctx context.Context, query string) ([]User, error)
}

// WorkItemGateway covers everything under "acli-plus jira workitem".
type WorkItemGateway interface {
	// GetWorkItem fetches one work item. A nil fields slice returns Jira's
	// default navigable set; otherwise only the named fields are requested.
	GetWorkItem(ctx context.Context, key string, fields []string) (WorkItem, error)
	// CreateWorkItem creates one work item and returns its key and id.
	CreateWorkItem(ctx context.Context, in NewWorkItemInput) (WorkItem, error)
	// CreateWorkItemsBulk creates up to 50 work items in one call.
	CreateWorkItemsBulk(ctx context.Context, in []NewWorkItemInput) ([]WorkItem, error)
	// UpdateWorkItem overwrites the given fields on an existing work item.
	UpdateWorkItem(ctx context.Context, in EditWorkItemInput) error
	// DeleteWorkItem permanently deletes a work item, optionally with its subtasks.
	DeleteWorkItem(ctx context.Context, key string, withSubtasks bool) error
	// Search runs a JQL query and returns one page of results.
	Search(ctx context.Context, in SearchInput) (SearchPage, error)
	// ArchiveWorkItems archives work items (Premium and Enterprise plans only).
	ArchiveWorkItems(ctx context.Context, keys []string) error
	// UnarchiveWorkItems restores archived work items.
	UnarchiveWorkItems(ctx context.Context, keys []string) error
	// AssignWorkItem sets the assignee. An empty accountID clears the assignee.
	AssignWorkItem(ctx context.Context, key, accountID string) error
	// ListTransitions returns the transitions available from the current status.
	ListTransitions(ctx context.Context, key string) ([]Transition, error)
	// TransitionWorkItem performs a transition, optionally setting fields that
	// the transition screen requires.
	TransitionWorkItem(ctx context.Context, key, transitionID string, fields FieldValues) error
	// RemoveWatcher removes one watcher from a work item.
	RemoveWatcher(ctx context.Context, key, accountID string) error
	// ListWatchers returns the accounts watching a work item.
	ListWatchers(ctx context.Context, key string) ([]User, error)
}

// CommentGateway covers the five comment subcommands.
type CommentGateway interface {
	ListComments(ctx context.Context, key string) ([]Comment, error)
	GetComment(ctx context.Context, key, commentID string) (Comment, Document, error)
	CreateComment(ctx context.Context, key string, body Document, vis CommentVisibility) (Comment, error)
	UpdateComment(ctx context.Context, key, commentID string, body Document, vis CommentVisibility) (Comment, error)
	DeleteComment(ctx context.Context, key, commentID string) error
}

// AttachmentGateway covers attachment listing and removal. Uploading is not
// part of acli's surface and is deliberately absent here.
type AttachmentGateway interface {
	ListAttachments(ctx context.Context, key string) ([]Attachment, error)
	DeleteAttachment(ctx context.Context, attachmentID string) error
}

// LinkGateway covers work item linking.
type LinkGateway interface {
	ListLinkTypes(ctx context.Context) ([]LinkType, error)
	CreateLink(ctx context.Context, in NewLinkInput) error
}

// ProjectGateway covers "acli-plus jira project".
type ProjectGateway interface {
	ListProjects(ctx context.Context, q ProjectQuery) ([]Project, error)
	GetProject(ctx context.Context, keyOrID string) (Project, error)
	CreateProject(ctx context.Context, in NewProjectInput) (Project, error)
	UpdateProject(ctx context.Context, in UpdateProjectInput) (Project, error)
	DeleteProject(ctx context.Context, keyOrID string, enableUndo bool) error
	ArchiveProject(ctx context.Context, keyOrID string) error
	RestoreProject(ctx context.Context, keyOrID string) error
}

// FieldGateway covers "acli-plus jira field".
type FieldGateway interface {
	ListFields(ctx context.Context) ([]Field, error)
	CreateField(ctx context.Context, in NewFieldInput) (Field, error)
	DeleteField(ctx context.Context, fieldID string) error
	RestoreField(ctx context.Context, fieldID string) error
}

// AgileGateway covers boards and sprints, which live on a separate API base
// path (/rest/agile/1.0) from the rest of Jira.
type AgileGateway interface {
	SearchBoards(ctx context.Context, q BoardQuery) ([]Board, error)
	ListSprints(ctx context.Context, boardID int, state string) ([]Sprint, error)
	ListSprintWorkItems(ctx context.Context, sprintID int, jql string, fields []string) ([]WorkItem, error)
}

// ViewGateway covers saved filters and dashboards.
type ViewGateway interface {
	SearchFilters(ctx context.Context, q FilterQuery) ([]Filter, error)
	ListMyFilters(ctx context.Context, favouritesOnly bool) ([]Filter, error)
	AddFilterFavourite(ctx context.Context, filterID string) error
	ChangeFilterOwner(ctx context.Context, filterID, accountID string) error
	SearchDashboards(ctx context.Context, q DashboardQuery) ([]Dashboard, error)
}

// Gateway is the whole Jira port, composed from the focused interfaces above so
// services and tests can depend on just the slice they need.
type Gateway interface {
	AccountGateway
	WorkItemGateway
	CommentGateway
	AttachmentGateway
	LinkGateway
	ProjectGateway
	FieldGateway
	AgileGateway
	ViewGateway
}
