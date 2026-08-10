package jira

import (
	"encoding/json"
	"time"
)

// User is an Atlassian account as it appears on a work item.
type User struct {
	AccountID   string
	DisplayName string
	Email       string
	Active      bool
}

// Name returns the friendliest identifier available for display.
func (u User) Name() string {
	switch {
	case u.DisplayName != "":
		return u.DisplayName
	case u.Email != "":
		return u.Email
	default:
		return u.AccountID
	}
}

// WorkItem is a Jira issue. The typed fields cover what the commands print;
// Raw carries the untouched API payload so --json can show everything the site
// returned, including custom fields this struct does not model.
type WorkItem struct {
	ID          string
	Key         string
	Summary     string
	Description string // ADF flattened to plain text for terminal output
	Status      string
	StatusCat   string
	Type        string
	ProjectKey  string
	Priority    string
	Resolution  string
	Assignee    User
	Reporter    User
	Labels      []string
	Components  []string
	FixVersions []string
	ParentKey   string
	Created     time.Time
	Updated     time.Time
	DueDate     string
	Raw         json.RawMessage
}

// FieldValues maps Jira field ids (summary, labels, customfield_10011, ...) to
// values already shaped the way the REST API expects them. Commands build it
// from flags; the gateway marshals it straight into the request's "fields".
type FieldValues map[string]any

// Merge copies non-conflicting entries from other, letting other win on clashes.
// It is how explicit --field flags override values derived from named flags.
func (f FieldValues) Merge(other FieldValues) FieldValues {
	if f == nil {
		f = FieldValues{}
	}
	for key, value := range other {
		f[key] = value
	}
	return f
}

// NewWorkItemInput creates one work item. Fields holds everything, including
// project/issuetype/summary, so custom fields need no special casing.
type NewWorkItemInput struct {
	Fields FieldValues
}

// EditWorkItemInput overwrites fields on an existing work item. Notify mirrors
// Jira's notifyUsers parameter (email on change), on by default in Jira itself.
type EditWorkItemInput struct {
	Key    string
	Fields FieldValues
	Notify bool
}

// SearchInput runs a JQL query. The API returns at most MaxResults per call and
// a token for the next page; an empty PageToken starts at the first page.
type SearchInput struct {
	JQL        string
	Fields     []string
	Expand     []string
	MaxResults int
	PageToken  string
}

// SearchPage is one page of JQL results. NextPageToken is empty on the last
// page. The API no longer reports a total, so callers must not expect one.
type SearchPage struct {
	Items         []WorkItem
	NextPageToken string
}

// Transition is one workflow move available from the current status.
type Transition struct {
	ID     string
	Name   string
	ToName string
	// HasScreen reports that Jira would normally show a field screen for this
	// transition, meaning it may reject the move without required fields.
	HasScreen bool
}

// Comment is a work item comment.
type Comment struct {
	ID         string
	Author     User
	Body       string // ADF flattened to plain text
	Created    time.Time
	Updated    time.Time
	Visibility CommentVisibility
	Raw        json.RawMessage
}

// CommentVisibility restricts a comment to a group or project role. A zero
// value means the comment is visible to anyone who can see the work item.
type CommentVisibility struct {
	Type  string // "group" or "role"
	Value string
}

// Set reports whether a visibility restriction is present.
func (v CommentVisibility) Set() bool { return v.Type != "" && v.Value != "" }

// Attachment is a file attached to a work item.
type Attachment struct {
	ID       string
	Filename string
	MimeType string
	Size     int64
	Author   User
	Created  time.Time
}

// LinkType is a configured relationship between two work items, e.g.
// "Blocks" with inward "is blocked by" and outward "blocks".
type LinkType struct {
	ID      string
	Name    string
	Inward  string
	Outward string
}

// NewLinkInput links two work items. Type is the link type name; Inward and
// Outward are the work item keys on each end of that named direction.
type NewLinkInput struct {
	Type    string
	Inward  string
	Outward string
	Comment string
}
