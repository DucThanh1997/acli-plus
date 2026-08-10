package jira

import (
	"fmt"

	"github.com/spf13/cobra"

	"acli-plus/internal/app"
	jiradomain "acli-plus/internal/domain/jira"
)

// bodyFlags collect a comment body, which may be typed inline or read from a
// Markdown or ADF file.
type bodyFlags struct {
	body string
	file string
}

func (b *bodyFlags) register(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&b.body, "body", "b", "", "comment text (plain text, Markdown, or ADF JSON)")
	cmd.Flags().StringVar(&b.file, "body-file", "", "read the comment from a Markdown or ADF file")
}

func (b *bodyFlags) source() app.DescriptionSource {
	if b.file != "" {
		return app.DescriptionSource{File: b.file}
	}
	return app.DescriptionSource{Text: b.body}
}

// visibilityFlags restrict a comment to a group or a project role.
type visibilityFlags struct {
	kind   string
	value  string
	public bool
}

func (v *visibilityFlags) register(cmd *cobra.Command, prefix string, withPublic bool) {
	cmd.Flags().StringVar(&v.kind, prefix+"type", "", "restrict the comment to a \"group\" or \"role\"")
	cmd.Flags().StringVar(&v.value, prefix+"value", "", "name of the group or role")
	if withPublic {
		cmd.Flags().BoolVar(&v.public, "public", false, "remove the restriction so anyone who can see the work item can read it")
	}
}

func (v *visibilityFlags) visibility() (jiradomain.CommentVisibility, error) {
	if v.public {
		return jiradomain.CommentVisibility{}, nil
	}
	if (v.kind == "") != (v.value == "") {
		return jiradomain.CommentVisibility{}, fmt.Errorf("visibility needs both a type and a value")
	}
	return jiradomain.CommentVisibility{Type: v.kind, Value: v.value}, nil
}

func newCommentCreateCmd(deps Deps) *cobra.Command {
	var (
		body       bodyFlags
		visibility visibilityFlags
	)
	cmd := &cobra.Command{
		Use:     "comment-create <key>",
		Short:   "Add a comment to a work item",
		Example: "  acli-plus jira workitem comment-create TEAM-1 -b \"Deployed to staging\"",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key, host, err := singleKey(args[0])
			if err != nil {
				return err
			}
			vis, err := visibility.visibility()
			if err != nil {
				return err
			}
			service, err := deps.Service(host)
			if err != nil {
				return err
			}
			result, err := service.CreateComment(cmd.Context(), key, body.source(), vis, deps.Options())
			if err != nil {
				return err
			}
			printResult(result, service.Host())
			return nil
		},
	}
	body.register(cmd)
	visibility.register(cmd, "visibility-", false)
	return cmd
}

func newCommentListCmd(deps Deps) *cobra.Command {
	var out format
	cmd := &cobra.Command{
		Use:   "comment-list <key>",
		Short: "Show the comments on a work item",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key, host, err := singleKey(args[0])
			if err != nil {
				return err
			}
			service, err := deps.Service(host)
			if err != nil {
				return err
			}
			comments, err := service.ListComments(cmd.Context(), key)
			if err != nil {
				return err
			}

			if out.json {
				return printJSON(comments)
			}
			for i, comment := range comments {
				if i > 0 {
					fmt.Println()
				}
				restriction := ""
				if comment.Visibility.Set() {
					restriction = fmt.Sprintf(" [%s: %s]", comment.Visibility.Type, comment.Visibility.Value)
				}
				fmt.Printf("%s  %s  %s%s\n", comment.ID, dateTime(comment.Created), comment.Author.Name(), restriction)
				fmt.Println(comment.Body)
			}
			if len(comments) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "no comments")
			}
			return nil
		},
	}
	out.register(cmd, false)
	return cmd
}

func newCommentUpdateCmd(deps Deps) *cobra.Command {
	var (
		body      bodyFlags
		commentID string
	)
	cmd := &cobra.Command{
		Use:   "comment-update <key>",
		Short: "Replace the text of a comment",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if commentID == "" {
				return fmt.Errorf("--id is required (see 'comment-list' for comment ids)")
			}
			key, host, err := singleKey(args[0])
			if err != nil {
				return err
			}
			service, err := deps.Service(host)
			if err != nil {
				return err
			}
			result, err := service.UpdateComment(cmd.Context(), key, commentID, body.source(), deps.Options())
			if err != nil {
				return err
			}
			printResult(result, service.Host())
			return nil
		},
	}
	cmd.Flags().StringVar(&commentID, "id", "", "id of the comment to change")
	body.register(cmd)
	return cmd
}

func newCommentDeleteCmd(deps Deps) *cobra.Command {
	var commentID string
	cmd := &cobra.Command{
		Use:   "comment-delete <key>",
		Short: "Delete a comment",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if commentID == "" {
				return fmt.Errorf("--id is required (see 'comment-list' for comment ids)")
			}
			key, host, err := singleKey(args[0])
			if err != nil {
				return err
			}
			service, err := deps.Service(host)
			if err != nil {
				return err
			}
			result, err := service.DeleteComment(cmd.Context(), key, commentID, deps.Options())
			if err != nil {
				return err
			}
			printResult(result, service.Host())
			return nil
		},
	}
	cmd.Flags().StringVar(&commentID, "id", "", "id of the comment to delete")
	return cmd
}

func newCommentVisibilityCmd(deps Deps) *cobra.Command {
	var (
		visibility visibilityFlags
		commentID  string
	)
	cmd := &cobra.Command{
		Use:   "comment-visibility <key>",
		Short: "Change who can read a comment",
		Example: "  acli-plus jira workitem comment-visibility TEAM-1 --id 10101 --type role --value Administrators\n" +
			"  acli-plus jira workitem comment-visibility TEAM-1 --id 10101 --public",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if commentID == "" {
				return fmt.Errorf("--id is required (see 'comment-list' for comment ids)")
			}
			vis, err := visibility.visibility()
			if err != nil {
				return err
			}
			if !vis.Set() && !visibility.public {
				return fmt.Errorf("pass --type and --value to restrict the comment, or --public to open it up")
			}
			key, host, err := singleKey(args[0])
			if err != nil {
				return err
			}
			service, err := deps.Service(host)
			if err != nil {
				return err
			}
			result, err := service.SetCommentVisibility(cmd.Context(), key, commentID, vis, deps.Options())
			if err != nil {
				return err
			}
			printResult(result, service.Host())
			return nil
		},
	}
	cmd.Flags().StringVar(&commentID, "id", "", "id of the comment to change")
	visibility.register(cmd, "", true)
	return cmd
}

func newAttachmentListCmd(deps Deps) *cobra.Command {
	var out format
	cmd := &cobra.Command{
		Use:   "attachment-list <key>",
		Short: "Show the files attached to a work item",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key, host, err := singleKey(args[0])
			if err != nil {
				return err
			}
			service, err := deps.Service(host)
			if err != nil {
				return err
			}
			attachments, err := service.ListAttachments(cmd.Context(), key)
			if err != nil {
				return err
			}
			table := newRows("ID", "FILENAME", "SIZE", "TYPE", "AUTHOR", "CREATED")
			for _, attachment := range attachments {
				table.add(attachment.ID, attachment.Filename, humanSize(attachment.Size),
					attachment.MimeType, attachment.Author.Name(), shortDate(attachment.Created))
			}
			return table.render(out, attachments)
		},
	}
	out.register(cmd, true)
	return cmd
}

func newAttachmentDeleteCmd(deps Deps) *cobra.Command {
	var ids []string
	cmd := &cobra.Command{
		Use:     "attachment-delete",
		Short:   "Delete attachments by id",
		Example: "  acli-plus jira workitem attachment-delete --id 10001,10002",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if len(ids) == 0 {
				return fmt.Errorf("--id is required (see 'attachment-list' for attachment ids)")
			}
			service, err := deps.Service("")
			if err != nil {
				return err
			}
			result, err := service.DeleteAttachments(cmd.Context(), ids, deps.Options())
			if err != nil {
				return err
			}
			printResult(result, service.Host())
			return nil
		},
	}
	cmd.Flags().StringSliceVar(&ids, "id", nil, "attachment id(s), comma-separated")
	return cmd
}

func newWatcherListCmd(deps Deps) *cobra.Command {
	var out format
	cmd := &cobra.Command{
		Use:   "watcher-list <key>",
		Short: "Show who is watching a work item",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key, host, err := singleKey(args[0])
			if err != nil {
				return err
			}
			service, err := deps.Service(host)
			if err != nil {
				return err
			}
			watchers, err := service.ListWatchers(cmd.Context(), key)
			if err != nil {
				return err
			}
			table := newRows("ACCOUNT ID", "NAME", "EMAIL")
			for _, watcher := range watchers {
				table.add(watcher.AccountID, watcher.DisplayName, watcher.Email)
			}
			return table.render(out, watchers)
		},
	}
	out.register(cmd, true)
	return cmd
}

func newWatcherRemoveCmd(deps Deps) *cobra.Command {
	var watchers []string
	cmd := &cobra.Command{
		Use:     "watcher-remove <key>",
		Short:   "Remove watchers from a work item",
		Example: "  acli-plus jira workitem watcher-remove TEAM-1 --watcher ann@acme.com",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(watchers) == 0 {
				return fmt.Errorf("--watcher is required")
			}
			key, host, err := singleKey(args[0])
			if err != nil {
				return err
			}
			service, err := deps.Service(host)
			if err != nil {
				return err
			}
			result, err := service.RemoveWatchers(cmd.Context(), key, watchers, deps.Options())
			if err != nil {
				return err
			}
			printResult(result, service.Host())
			return nil
		},
	}
	cmd.Flags().StringSliceVar(&watchers, "watcher", nil, "email, display name, or account id; comma-separated")
	return cmd
}
