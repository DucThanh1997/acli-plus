package jira

import (
	"encoding/json"
	"time"
)

// Project is a Jira project.
type Project struct {
	ID       string
	Key      string
	Name     string
	TypeKey  string // "software", "service_desk", "business"
	Style    string // "classic" (company-managed) or "next-gen" (team-managed)
	Lead     User
	URL      string
	Archived bool
	Deleted  bool
	Raw      json.RawMessage
}

// NewProjectInput creates a project. LeadAccountID is required by the API; the
// application layer fills it with the current user when the flag is omitted.
type NewProjectInput struct {
	Key           string
	Name          string
	TypeKey       string
	TemplateKey   string
	Description   string
	LeadAccountID string
	AssigneeType  string
}

// UpdateProjectInput changes a project's mutable attributes. Empty fields are
// left untouched, so callers only set what they want to change.
type UpdateProjectInput struct {
	KeyOrID       string
	NewKey        string
	Name          string
	Description   string
	LeadAccountID string
	AssigneeType  string
}

// Field is a system or custom field definition.
type Field struct {
	ID         string
	Key        string
	Name       string
	Custom     bool
	Searchable bool
	// SchemaType is the field's value type ("string", "number", "array", ...)
	// and ItemType is the element type when SchemaType is "array". Together they
	// decide how a --field value typed on the command line must be shaped.
	SchemaType  string
	ItemType    string
	CustomType  string
	ClauseNames []string
}

// NewFieldInput creates a custom field. Type and SearcherKey are the fully
// qualified Jira keys, e.g. "com.atlassian.jira.plugin.system.customfieldtypes:textfield".
type NewFieldInput struct {
	Name        string
	Description string
	Type        string
	SearcherKey string
}

// Board is an Agile board (Scrum or Kanban).
type Board struct {
	ID         int
	Name       string
	Type       string
	ProjectKey string
	URL        string
}

// Sprint is a Scrum sprint on a board.
type Sprint struct {
	ID        int
	BoardID   int
	Name      string
	State     string // "future", "active", "closed"
	Goal      string
	Start     time.Time
	End       time.Time
	Completed time.Time
}

// Filter is a saved JQL search.
type Filter struct {
	ID         string
	Name       string
	JQL        string
	Owner      User
	Favourite  bool
	URL        string
	SharedWith []string
}

// Dashboard is a Jira dashboard.
type Dashboard struct {
	ID         string
	Name       string
	Owner      User
	URL        string
	Favourite  bool
	Popularity int
}
