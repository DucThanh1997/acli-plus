package app

import (
	"context"
	"errors"
	"fmt"

	confluence "acli-plus/internal/domain/confluence"
)

// versionStamp is written as the version message on every acli-plus write. Its
// presence on a page's latest version is how the tool recognizes its own writes
// (and, by absence, edits made outside the tool).
const versionStamp = "via acli-plus"

// Action names the outcome of a command for user-facing output.
type Action string

const (
	ActionCreate  Action = "create"
	ActionUpdate  Action = "update"
	ActionDelete  Action = "delete"
	ActionAborted Action = "aborted"
)

// ConfirmFunc asks the user a yes/no question. The command layer supplies it so
// the service can request confirmation without doing terminal I/O itself.
type ConfirmFunc func(prompt string) (bool, error)

// WriteOptions carries per-command behavior flags.
type WriteOptions struct {
	DryRun      bool
	SkipConfirm bool // set by --yes/--force
	Confirm     ConfirmFunc
}

// Result describes what a command did (or would do, when DryRun is set).
type Result struct {
	Action   Action
	Page     confluence.Page
	Warnings []string
	DryRun   bool
}

// CreateInput publishes Content as a child under Parent (a page or space URL).
type CreateInput struct {
	Content Content
	Parent  confluence.PageRef
	Opts    WriteOptions
}

// UpdateInput publishes Content to the exact page identified by Target.
type UpdateInput struct {
	Content Content
	Target  confluence.PageRef
	Opts    WriteOptions
}

// DeleteInput trashes the page identified by Target.
type DeleteInput struct {
	Target confluence.PageRef
	Opts   WriteOptions
}

// PageService implements the create/update/delete use cases against a gateway.
type PageService struct {
	gw confluence.Gateway
}

// NewPageService wires the service to a Confluence gateway.
func NewPageService(gw confluence.Gateway) *PageService {
	return &PageService{gw: gw}
}

// Create publishes Content under Parent. If a page with the same title already
// exists in the space (titles are unique per space), it is updated instead of
// duplicated.
func (s *PageService) Create(ctx context.Context, in CreateInput) (Result, error) {
	spaceID, parentID, err := s.resolveParent(ctx, in.Parent)
	if err != nil {
		return Result{}, err
	}

	existing, found, err := s.gw.FindPageByTitle(ctx, spaceID, in.Content.Title)
	if err != nil {
		return Result{}, err
	}
	if found {
		fresh, err := s.gw.GetPage(ctx, existing.ID)
		if err != nil {
			return Result{}, err
		}
		return s.writeUpdate(ctx, fresh, in.Content, in.Opts)
	}

	if in.Opts.DryRun {
		return Result{
			Action:   ActionCreate,
			DryRun:   true,
			Warnings: in.Content.Warnings,
			Page:     confluence.Page{Title: in.Content.Title, SpaceID: spaceID, ParentID: parentID},
		}, nil
	}

	page, err := s.gw.CreatePage(ctx, confluence.NewPageInput{
		SpaceID:  spaceID,
		ParentID: parentID,
		Title:    in.Content.Title,
		Body:     in.Content.Body,
	})
	if err != nil {
		return Result{}, err
	}
	return Result{Action: ActionCreate, Page: page, Warnings: in.Content.Warnings}, nil
}

// Update overwrites the exact page in Target. If that page id no longer resolves,
// it inserts the content into the URL's space instead.
func (s *PageService) Update(ctx context.Context, in UpdateInput) (Result, error) {
	if in.Target.PageID == "" {
		return Result{}, fmt.Errorf("update needs a page URL (with a page id); got %q", in.Target.SpaceKey)
	}

	page, err := s.gw.GetPage(ctx, in.Target.PageID)
	if err != nil {
		if errors.Is(err, confluence.ErrPageNotFound) {
			return s.insertIntoSpace(ctx, in)
		}
		return Result{}, err
	}
	return s.writeUpdate(ctx, page, in.Content, in.Opts)
}

// Delete moves the target page to the trash after confirmation.
func (s *PageService) Delete(ctx context.Context, in DeleteInput) (Result, error) {
	if in.Target.PageID == "" {
		return Result{}, fmt.Errorf("delete needs a page URL (with a page id)")
	}

	page, err := s.gw.GetPage(ctx, in.Target.PageID)
	if err != nil {
		return Result{}, err
	}

	if in.Opts.DryRun {
		return Result{Action: ActionDelete, DryRun: true, Page: page}, nil
	}

	if !in.Opts.SkipConfirm {
		ok, err := ask(in.Opts.Confirm,
			fmt.Sprintf("Delete (move to trash) page %q (id %s)?", page.Title, page.ID))
		if err != nil {
			return Result{}, err
		}
		if !ok {
			return Result{Action: ActionAborted, Page: page}, nil
		}
	}

	if err := s.gw.DeletePage(ctx, page.ID); err != nil {
		return Result{}, err
	}
	return Result{Action: ActionDelete, Page: page}, nil
}

// writeUpdate overwrites an existing page, applying stateless conflict detection:
// if the page's latest version was not stamped by acli-plus, it was edited
// elsewhere, so confirmation is required (unless --yes/--force or --dry-run).
func (s *PageService) writeUpdate(ctx context.Context, page confluence.Page, content Content, opts WriteOptions) (Result, error) {
	if opts.DryRun {
		return Result{Action: ActionUpdate, DryRun: true, Page: page, Warnings: content.Warnings}, nil
	}

	if page.Version.Message != versionStamp && !opts.SkipConfirm {
		ok, err := ask(opts.Confirm,
			fmt.Sprintf("Page %q (id %s, version %d) was modified outside acli-plus. Overwrite?",
				page.Title, page.ID, page.Version.Number))
		if err != nil {
			return Result{}, err
		}
		if !ok {
			return Result{Action: ActionAborted, Page: page}, nil
		}
	}

	updated, err := s.gw.UpdatePage(ctx, confluence.UpdatePageInput{
		ID:          page.ID,
		Title:       content.Title,
		Body:        content.Body,
		NextVersion: page.Version.Number + 1,
		Message:     versionStamp,
	})
	if err != nil {
		return Result{}, err
	}
	return Result{Action: ActionUpdate, Page: updated, Warnings: content.Warnings}, nil
}

// insertIntoSpace is the update fallback when the target page id is gone: create
// the content at the root of the URL's space, with a warning.
func (s *PageService) insertIntoSpace(ctx context.Context, in UpdateInput) (Result, error) {
	if in.Target.SpaceKey == "" {
		return Result{}, fmt.Errorf("%w and no space in URL to insert into", confluence.ErrPageNotFound)
	}

	spaceID, err := s.gw.ResolveSpaceID(ctx, in.Target.SpaceKey)
	if err != nil {
		return Result{}, err
	}

	warnings := append([]string{
		"target page id not found; inserted into space " + in.Target.SpaceKey,
	}, in.Content.Warnings...)

	if in.Opts.DryRun {
		return Result{
			Action:   ActionCreate,
			DryRun:   true,
			Warnings: warnings,
			Page:     confluence.Page{Title: in.Content.Title, SpaceID: spaceID},
		}, nil
	}

	page, err := s.gw.CreatePage(ctx, confluence.NewPageInput{
		SpaceID: spaceID,
		Title:   in.Content.Title,
		Body:    in.Content.Body,
	})
	if err != nil {
		return Result{}, err
	}
	return Result{Action: ActionCreate, Page: page, Warnings: warnings}, nil
}

// resolveParent turns a parent ref into (spaceID, parentID). A page ref creates
// a child under that page; a space ref creates at the space root.
func (s *PageService) resolveParent(ctx context.Context, ref confluence.PageRef) (spaceID, parentID string, err error) {
	if ref.HasPage() {
		parent, err := s.gw.GetPage(ctx, ref.PageID)
		if err != nil {
			return "", "", err
		}
		return parent.SpaceID, parent.ID, nil
	}
	if ref.SpaceKey == "" {
		return "", "", fmt.Errorf("create needs a parent page or space URL")
	}
	spaceID, err = s.gw.ResolveSpaceID(ctx, ref.SpaceKey)
	if err != nil {
		return "", "", err
	}
	return spaceID, "", nil
}

// ask calls the confirmation callback, treating a missing callback as "no".
func ask(confirm ConfirmFunc, prompt string) (bool, error) {
	if confirm == nil {
		return false, nil
	}
	return confirm(prompt)
}
