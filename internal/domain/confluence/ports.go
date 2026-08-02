package confluence

import "context"

// Gateway is the port for talking to a Confluence Cloud site. The application
// layer depends on this interface; the adapter in internal/gateway/confluence
// implements it. All methods take a context so calls honor cancellation and
// timeouts set at the command boundary.
type Gateway interface {
	// VerifyAuth checks that the configured credentials are accepted by the site.
	VerifyAuth(ctx context.Context) error
	// ResolveSpaceID maps a space key (e.g. DEV, ~712020abc) to its numeric id.
	ResolveSpaceID(ctx context.Context, spaceKey string) (string, error)
	// GetPage fetches a page by id, including its version number and message.
	GetPage(ctx context.Context, pageID string) (Page, error)
	// FindPageByTitle finds a page by exact title within a space. Titles are
	// unique per space, so the match (if any) is unambiguous.
	FindPageByTitle(ctx context.Context, spaceID, title string) (Page, bool, error)
	// CreatePage creates a new page and returns it (version 1).
	CreatePage(ctx context.Context, in NewPageInput) (Page, error)
	// UpdatePage overwrites an existing page in place and returns the new state.
	UpdatePage(ctx context.Context, in UpdatePageInput) (Page, error)
	// DeletePage moves a page to the trash (a reversible soft delete).
	DeletePage(ctx context.Context, pageID string) error
}
