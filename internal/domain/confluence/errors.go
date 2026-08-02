package confluence

import "errors"

// Domain sentinel errors. Adapters map transport/storage failures onto these;
// the command layer maps them to user-facing messages and exit codes.
var (
	// ErrPageNotFound means a page id did not resolve to a live page.
	ErrPageNotFound = errors.New("confluence: page not found")
	// ErrSpaceNotFound means a space key did not resolve to a space.
	ErrSpaceNotFound = errors.New("confluence: space not found")
	// ErrShortLink means a /wiki/x/ short link was supplied; the full URL is required.
	ErrShortLink = errors.New("confluence: short /wiki/x/ links are not supported; paste the full page URL")
	// ErrInvalidRef means the input was neither a recognizable Confluence URL nor a page id.
	ErrInvalidRef = errors.New("confluence: not a recognizable Confluence URL or page id")
	// ErrAuth means the site rejected the supplied credentials.
	ErrAuth = errors.New("confluence: authentication failed")
)
