// Package confluence holds the domain model for Confluence pages: entities,
// value objects, the gateway port, and the rules for locating a page from a
// user-supplied URL. It depends on nothing outside the standard library.
package confluence

// Page is a Confluence page as the rest of the application cares about it.
type Page struct {
	ID       string
	Title    string
	SpaceID  string
	ParentID string
	Version  Version
}

// Version captures a page's optimistic-lock version number and the message
// stamped on that version. acli-plus stamps its own writes so it can later tell
// whether the latest version was written by the tool or edited elsewhere.
type Version struct {
	Number  int
	Message string
}

// NewPageInput describes a page to create. A create always starts at version 1,
// so no version message can be set here (see UpdatePageInput.Message).
type NewPageInput struct {
	SpaceID  string
	ParentID string // empty means create at the space root
	Title    string
	Body     string // Confluence storage-format XHTML
}

// UpdatePageInput describes an in-place update of an existing page. NextVersion
// MUST be the current version number plus one (Confluence optimistic locking).
type UpdatePageInput struct {
	ID          string
	Title       string
	Body        string // Confluence storage-format XHTML
	NextVersion int
	Message     string
}
