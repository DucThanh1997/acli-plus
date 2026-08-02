package confluence

import (
	"net/url"
	"strings"
)

// PageRef locates a Confluence resource parsed from a user-supplied URL or a
// bare page id. A page URL yields SpaceKey + PageID; a space URL yields only
// SpaceKey; a bare numeric id yields only PageID (Host comes from config).
type PageRef struct {
	Host     string // e.g. acme.atlassian.net; empty for a bare page id
	SpaceKey string // e.g. DEV or ~712020abc; empty for a bare page id
	PageID   string // empty for a space URL
}

// HasPage reports whether the ref points at a specific page.
func (r PageRef) HasPage() bool { return r.PageID != "" }

// ParseRef parses a Confluence page or space URL, or a bare numeric page id.
// It returns ErrShortLink for /wiki/x/ short links and ErrInvalidRef otherwise.
func ParseRef(input string) (PageRef, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return PageRef{}, ErrInvalidRef
	}
	if isAllDigits(trimmed) {
		return PageRef{PageID: trimmed}, nil
	}

	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Host == "" {
		return PageRef{}, ErrInvalidRef
	}

	segments := pathSegments(parsed.Path)
	if isShortLink(segments) {
		return PageRef{}, ErrShortLink
	}

	ref := PageRef{
		Host:     parsed.Host,
		SpaceKey: segmentAfter(segments, "spaces"),
		PageID:   segmentAfter(segments, "pages"),
	}
	if ref.SpaceKey == "" && ref.PageID == "" {
		return PageRef{}, ErrInvalidRef
	}
	return ref, nil
}

func isAllDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}

func pathSegments(path string) []string {
	raw := strings.Split(strings.Trim(path, "/"), "/")
	segments := make([]string, 0, len(raw))
	for _, seg := range raw {
		if seg == "" {
			continue
		}
		if decoded, err := url.PathUnescape(seg); err == nil {
			segments = append(segments, decoded)
			continue
		}
		segments = append(segments, seg)
	}
	return segments
}

// isShortLink detects the /wiki/x/<hash> tiny-link form.
func isShortLink(segments []string) bool {
	for i, seg := range segments {
		if seg == "wiki" && i+1 < len(segments) && segments[i+1] == "x" {
			return true
		}
	}
	return false
}

// segmentAfter returns the path segment immediately following key, or "".
func segmentAfter(segments []string, key string) string {
	for i, seg := range segments {
		if seg == key && i+1 < len(segments) {
			return segments[i+1]
		}
	}
	return ""
}
