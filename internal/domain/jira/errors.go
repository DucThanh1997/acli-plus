// Package jira holds the Jira Cloud domain: the entities the commands operate
// on and the ports the application layer depends on. It has no knowledge of
// HTTP, Cobra, or the on-disk config.
package jira

import "errors"

// ErrAuth indicates the site rejected the stored credentials (401/403).
var ErrAuth = errors.New("authentication failed; check your email and API token (run 'acli-plus setup')")

// ErrWorkItemNotFound indicates the work item key or id does not resolve.
var ErrWorkItemNotFound = errors.New("work item not found")

// ErrProjectNotFound indicates the project key or id does not resolve.
var ErrProjectNotFound = errors.New("project not found")

// ErrBoardNotFound indicates no board matched the given name or id.
var ErrBoardNotFound = errors.New("board not found")

// ErrSprintNotFound indicates no sprint matched the given name or id.
var ErrSprintNotFound = errors.New("sprint not found")

// ErrFieldNotFound indicates a field name could not be mapped to a field id.
var ErrFieldNotFound = errors.New("field not found")

// ErrTransitionNotFound indicates the requested status is not reachable from the
// work item's current status. Jira exposes only the transitions available now,
// so this is a workflow constraint rather than a bad name.
var ErrTransitionNotFound = errors.New("no transition to that status is available from the current status")

// ErrLinkTypeNotFound indicates the given link type is not configured on the site.
var ErrLinkTypeNotFound = errors.New("issue link type not found")

// ErrUserNotFound indicates an assignee/reporter lookup matched nobody.
var ErrUserNotFound = errors.New("user not found")

// ErrNotLicensed indicates the operation needs a plan the site does not have.
// Archiving work items, for example, is Premium and Enterprise only.
var ErrNotLicensed = errors.New("this operation is not available on your Jira plan")
