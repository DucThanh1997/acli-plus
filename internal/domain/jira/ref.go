package jira

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// ErrInvalidKey indicates a value is neither a work item key nor a Jira URL
// containing one.
var ErrInvalidKey = errors.New("not a work item key (expected e.g. TEAM-123) or a Jira work item URL")

// keyPattern matches a work item key: a project key (letters, digits and
// underscores, starting with a letter) followed by a dash and the number.
var keyPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*-[0-9]+$`)

// Ref locates a work item. Host is set only when the input was a full URL, in
// which case it selects the site the same way a Confluence page URL does.
type Ref struct {
	Host string
	Key  string
}

// ParseRef accepts a bare key ("TEAM-123", case-insensitive) or any Jira URL
// that carries one: /browse/TEAM-123, a board URL with ?selectedIssue=TEAM-123,
// or the /jira/software/projects/.../issues/TEAM-123 form. The key is upper-cased
// because Jira keys are canonically upper-case even though lookups are lenient.
func ParseRef(input string) (Ref, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return Ref{}, ErrInvalidKey
	}
	if keyPattern.MatchString(trimmed) {
		return Ref{Key: strings.ToUpper(trimmed)}, nil
	}
	if !strings.Contains(trimmed, "/") {
		return Ref{}, fmt.Errorf("%w: %q", ErrInvalidKey, trimmed)
	}

	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Host == "" {
		return Ref{}, fmt.Errorf("%w: %q", ErrInvalidKey, trimmed)
	}

	if key := keyFromQuery(parsed); key != "" {
		return Ref{Host: parsed.Host, Key: key}, nil
	}
	for _, segment := range strings.Split(parsed.Path, "/") {
		if keyPattern.MatchString(segment) {
			return Ref{Host: parsed.Host, Key: strings.ToUpper(segment)}, nil
		}
	}
	return Ref{}, fmt.Errorf("%w: %q", ErrInvalidKey, trimmed)
}

// ParseRefs parses a comma-separated list, the form acli uses for --key.
func ParseRefs(input string) ([]Ref, error) {
	refs := make([]Ref, 0, 4)
	for _, part := range strings.Split(input, ",") {
		if strings.TrimSpace(part) == "" {
			continue
		}
		ref, err := ParseRef(part)
		if err != nil {
			return nil, err
		}
		refs = append(refs, ref)
	}
	if len(refs) == 0 {
		return nil, ErrInvalidKey
	}
	return refs, nil
}

// Keys reduces refs to their work item keys, preserving order.
func Keys(refs []Ref) []string {
	keys := make([]string, len(refs))
	for i, ref := range refs {
		keys[i] = ref.Key
	}
	return keys
}

// HostOf returns the first host present in refs, or "" when all are bare keys.
// A mixed list is not an error: the first URL decides the site.
func HostOf(refs []Ref) string {
	for _, ref := range refs {
		if ref.Host != "" {
			return ref.Host
		}
	}
	return ""
}

// BrowseURL builds the canonical link to a work item on a host.
func BrowseURL(host, key string) string {
	if host == "" || key == "" {
		return ""
	}
	return "https://" + host + "/browse/" + key
}

// keyFromQuery pulls a key out of the query parameters Jira uses to deep-link a
// work item from a board or backlog view.
func keyFromQuery(parsed *url.URL) string {
	query := parsed.Query()
	for _, param := range []string{"selectedIssue", "issueKey", "modal-issue-key"} {
		if value := query.Get(param); keyPattern.MatchString(value) {
			return strings.ToUpper(value)
		}
	}
	return ""
}
