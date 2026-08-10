package app

import (
	"context"
	"fmt"
	"strings"

	jira "acli-plus/internal/domain/jira"
)

// ListComments returns a work item's comments, oldest first.
func (s *JiraService) ListComments(ctx context.Context, key string) ([]jira.Comment, error) {
	return s.gw.ListComments(ctx, key)
}

// CreateComment adds a comment. The body is Markdown or plain text, or a file
// containing either, and is converted to ADF before sending.
func (s *JiraService) CreateComment(ctx context.Context, key string, src DescriptionSource, vis jira.CommentVisibility, opts WriteOptions) (JiraResult, error) {
	rendered, err := RenderDescription(src)
	if err != nil {
		return JiraResult{}, err
	}
	if rendered.Body.Empty() {
		return JiraResult{}, fmt.Errorf("a comment body is required (--body or --body-file)")
	}

	if opts.DryRun {
		return JiraResult{Detail: "comment on " + key, Keys: []string{key}, DryRun: true, Warnings: rendered.Warnings}, nil
	}

	comment, err := s.gw.CreateComment(ctx, key, rendered.Body, vis)
	if err != nil {
		return JiraResult{}, err
	}
	return JiraResult{
		Detail:   fmt.Sprintf("commented on %s (comment %s)", key, comment.ID),
		Keys:     []string{key},
		Warnings: rendered.Warnings,
	}, nil
}

// UpdateComment replaces a comment's body, keeping its current visibility.
func (s *JiraService) UpdateComment(ctx context.Context, key, commentID string, src DescriptionSource, opts WriteOptions) (JiraResult, error) {
	rendered, err := RenderDescription(src)
	if err != nil {
		return JiraResult{}, err
	}
	if rendered.Body.Empty() {
		return JiraResult{}, fmt.Errorf("a comment body is required (--body or --body-file)")
	}

	existing, _, err := s.gw.GetComment(ctx, key, commentID)
	if err != nil {
		return JiraResult{}, err
	}

	if opts.DryRun {
		return JiraResult{Detail: fmt.Sprintf("update comment %s on %s", commentID, key), Keys: []string{key}, DryRun: true, Warnings: rendered.Warnings}, nil
	}

	if _, err := s.gw.UpdateComment(ctx, key, commentID, rendered.Body, existing.Visibility); err != nil {
		return JiraResult{}, err
	}
	return JiraResult{
		Detail:   fmt.Sprintf("updated comment %s on %s", commentID, key),
		Keys:     []string{key},
		Warnings: rendered.Warnings,
	}, nil
}

// SetCommentVisibility changes who can see a comment without touching its text.
// It reads the comment first because Jira's update call replaces the whole
// comment, body included.
func (s *JiraService) SetCommentVisibility(ctx context.Context, key, commentID string, vis jira.CommentVisibility, opts WriteOptions) (JiraResult, error) {
	_, body, err := s.gw.GetComment(ctx, key, commentID)
	if err != nil {
		return JiraResult{}, err
	}

	target := "everyone who can see the work item"
	if vis.Set() {
		target = vis.Type + " " + vis.Value
	}
	if opts.DryRun {
		return JiraResult{
			Detail: fmt.Sprintf("restrict comment %s on %s to %s", commentID, key, target),
			Keys:   []string{key},
			DryRun: true,
		}, nil
	}

	if _, err := s.gw.UpdateComment(ctx, key, commentID, body, vis); err != nil {
		return JiraResult{}, err
	}
	return JiraResult{
		Detail: fmt.Sprintf("comment %s on %s is now visible to %s", commentID, key, target),
		Keys:   []string{key},
	}, nil
}

// DeleteComment removes a comment after confirmation.
func (s *JiraService) DeleteComment(ctx context.Context, key, commentID string, opts WriteOptions) (JiraResult, error) {
	if opts.DryRun {
		return JiraResult{Detail: fmt.Sprintf("delete comment %s on %s", commentID, key), Keys: []string{key}, DryRun: true}, nil
	}

	ok, err := confirmOrAbort(opts, fmt.Sprintf("Delete comment %s on %s? This cannot be undone.", commentID, key))
	if err != nil {
		return JiraResult{}, err
	}
	if !ok {
		return aborted(), nil
	}

	if err := s.gw.DeleteComment(ctx, key, commentID); err != nil {
		return JiraResult{}, err
	}
	return JiraResult{Detail: fmt.Sprintf("deleted comment %s on %s", commentID, key), Keys: []string{key}}, nil
}

// ListAttachments returns the files attached to a work item.
func (s *JiraService) ListAttachments(ctx context.Context, key string) ([]jira.Attachment, error) {
	return s.gw.ListAttachments(ctx, key)
}

// DeleteAttachments removes attachments by id after confirmation.
func (s *JiraService) DeleteAttachments(ctx context.Context, ids []string, opts WriteOptions) (JiraResult, error) {
	if opts.DryRun {
		return JiraResult{Detail: "delete attachment(s) " + strings.Join(ids, ", "), DryRun: true}, nil
	}

	prompt := fmt.Sprintf("Delete %d attachment(s): %s? This cannot be undone.", len(ids), strings.Join(ids, ", "))
	ok, err := confirmOrAbort(opts, prompt)
	if err != nil {
		return JiraResult{}, err
	}
	if !ok {
		return aborted(), nil
	}

	for _, id := range ids {
		if err := s.gw.DeleteAttachment(ctx, id); err != nil {
			return JiraResult{}, fmt.Errorf("attachment %s: %w", id, err)
		}
	}
	return JiraResult{Detail: "deleted attachment(s) " + strings.Join(ids, ", ")}, nil
}
