package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	jira "acli-plus/internal/domain/jira"
	"acli-plus/internal/markdown"
)

// JiraService implements the Jira use cases against a gateway. It caches the
// two site-wide lookups the commands need repeatedly — the current user and the
// field catalog — so a single command never fetches either twice.
type JiraService struct {
	gw   jira.Gateway
	host string

	currentUser *jira.User
	fields      []jira.Field
}

// NewJiraService wires the service to a Jira gateway. host is carried along so
// results can be printed as browse links.
func NewJiraService(gw jira.Gateway, host string) *JiraService {
	return &JiraService{gw: gw, host: host}
}

// Host returns the site the service talks to.
func (s *JiraService) Host() string { return s.host }

// JiraResult reports what a write command did, or would do under --dry-run.
type JiraResult struct {
	// Detail is the user-facing summary line, e.g. `created TEAM-12 "Fix login"`.
	Detail string
	// Keys are the work items, projects or filters the command touched.
	Keys     []string
	DryRun   bool
	Aborted  bool
	Warnings []string
}

// aborted builds the result for a command the user declined at the prompt.
func aborted() JiraResult { return JiraResult{Detail: "aborted; no changes made", Aborted: true} }

// confirmOrAbort asks for confirmation unless --yes/--force was passed. It
// returns false when the caller should stop without making changes.
func confirmOrAbort(opts WriteOptions, prompt string) (bool, error) {
	if opts.SkipConfirm {
		return true, nil
	}
	return ask(opts.Confirm, prompt)
}

// accountIDPattern matches an Atlassian account id, which is either a 24-char
// alphanumeric string (not hex — real ids contain letters past f) or a
// "<siteId>:<uuid>" pair. Anything else is treated as a name or email to look up.
var accountIDPattern = regexp.MustCompile(`^([0-9a-zA-Z]{24}|[0-9a-zA-Z]+:[0-9a-fA-F-]{36})$`)

// CurrentUser returns the account the stored credentials belong to.
func (s *JiraService) CurrentUser(ctx context.Context) (jira.User, error) {
	if s.currentUser != nil {
		return *s.currentUser, nil
	}
	user, err := s.gw.CurrentUser(ctx)
	if err != nil {
		return jira.User{}, err
	}
	s.currentUser = &user
	return user, nil
}

// ResolveAccountID turns what a user typed into an account id. It accepts "@me"
// for the authenticated account, an account id as-is, and otherwise searches by
// email or display name. An empty value means "unassigned", and "-1" is Jira's
// own token for a project's default assignee; both pass straight through.
func (s *JiraService) ResolveAccountID(ctx context.Context, value string) (string, error) {
	value = strings.TrimSpace(value)
	switch strings.ToLower(value) {
	case "", "none", "unassigned", "null":
		return "", nil
	case "@me", "me":
		user, err := s.CurrentUser(ctx)
		if err != nil {
			return "", err
		}
		return user.AccountID, nil
	case "-1", "default":
		return "-1", nil
	}
	if accountIDPattern.MatchString(value) {
		return value, nil
	}

	users, err := s.gw.FindUsers(ctx, value)
	if err != nil {
		return "", err
	}
	switch matches := exactUserMatches(users, value); len(matches) {
	case 1:
		return matches[0].AccountID, nil
	case 0:
		if len(users) == 1 {
			return users[0].AccountID, nil
		}
		if len(users) == 0 {
			return "", fmt.Errorf("%w: %s", jira.ErrUserNotFound, value)
		}
		return "", ambiguousUserError(value, users)
	default:
		return matches[0].AccountID, nil
	}
}

// exactUserMatches narrows a fuzzy search to accounts whose email or display
// name equals the query, so "ann@acme.com" is not ambiguous just because the
// site also has "ann.lee@acme.com".
func exactUserMatches(users []jira.User, value string) []jira.User {
	var matches []jira.User
	for _, user := range users {
		if strings.EqualFold(user.Email, value) || strings.EqualFold(user.DisplayName, value) {
			matches = append(matches, user)
		}
	}
	return matches
}

func ambiguousUserError(value string, users []jira.User) error {
	names := make([]string, 0, len(users))
	for _, user := range users {
		names = append(names, fmt.Sprintf("%s (%s)", user.Name(), user.AccountID))
	}
	return fmt.Errorf("%q matches %d accounts; pass an account id instead: %s",
		value, len(users), strings.Join(names, ", "))
}

// FieldCatalog returns every field on the site, fetched at most once.
func (s *JiraService) FieldCatalog(ctx context.Context) ([]jira.Field, error) {
	if s.fields != nil {
		return s.fields, nil
	}
	fields, err := s.gw.ListFields(ctx)
	if err != nil {
		return nil, err
	}
	s.fields = fields
	return fields, nil
}

// ResolveField maps what a user typed onto a field definition, matching on id,
// key, or name (case-insensitive) so "--field Story Points=3" works without
// anyone having to look up customfield_10016.
func (s *JiraService) ResolveField(ctx context.Context, name string) (jira.Field, error) {
	fields, err := s.FieldCatalog(ctx)
	if err != nil {
		return jira.Field{}, err
	}
	for _, field := range fields {
		if field.ID == name || field.Key == name {
			return field, nil
		}
	}
	var matches []jira.Field
	for _, field := range fields {
		if strings.EqualFold(field.Name, name) {
			matches = append(matches, field)
		}
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return jira.Field{}, fmt.Errorf("%w: %q", jira.ErrFieldNotFound, name)
	default:
		ids := make([]string, 0, len(matches))
		for _, field := range matches {
			ids = append(ids, field.ID)
		}
		return jira.Field{}, fmt.Errorf("field name %q is used by %d fields; pass one of these ids: %s",
			name, len(matches), strings.Join(ids, ", "))
	}
}

// DescriptionSource is where a work item's summary and description come from.
// Exactly one of Text, File, or ADF is used; the command layer decides which
// flag maps to which.
type DescriptionSource struct {
	// Text is a plain value from --description or --comment.
	Text string
	// File is a Markdown or ADF file from --description-file or --from-file.
	File string
}

// RenderedDescription is the result of turning a source into ADF.
type RenderedDescription struct {
	Body jira.Document
	// Title is the summary taken from a file's frontmatter or leading H1, and is
	// empty for plain text sources.
	Title    string
	Warnings []string
}

// RenderDescription converts a description source to ADF. A file that already
// contains an ADF document is passed through untouched; anything else is
// treated as Markdown.
func RenderDescription(src DescriptionSource) (RenderedDescription, error) {
	if src.File != "" {
		source, err := os.ReadFile(src.File)
		if err != nil {
			return RenderedDescription{}, fmt.Errorf("reading %s: %w", src.File, err)
		}
		if adf, ok := markdown.ParseADF(source); ok {
			return RenderedDescription{Body: jira.Document(adf)}, nil
		}
		doc, err := markdown.ConvertADF(source)
		if err != nil {
			return RenderedDescription{}, err
		}
		title := doc.Title
		if title == "" {
			title = strings.TrimSuffix(filepath.Base(src.File), filepath.Ext(src.File))
		}
		return RenderedDescription{Body: jira.Document(doc.JSON), Title: title, Warnings: doc.Warnings}, nil
	}

	if strings.TrimSpace(src.Text) == "" {
		return RenderedDescription{}, nil
	}
	if adf, ok := markdown.ParseADF([]byte(src.Text)); ok {
		return RenderedDescription{Body: jira.Document(adf)}, nil
	}
	return RenderedDescription{Body: jira.Document(markdown.TextToADF(src.Text))}, nil
}

// BrowseURL builds the link to a work item on this service's site.
func (s *JiraService) BrowseURL(key string) string { return jira.BrowseURL(s.host, key) }
