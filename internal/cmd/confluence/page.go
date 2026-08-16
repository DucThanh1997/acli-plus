package confluence

import (
	"github.com/spf13/cobra"

	"acli-plus/internal/app"
	confluence "acli-plus/internal/domain/confluence"
)

func newPageCmd(deps Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "page",
		Short: "Create, update and delete Confluence pages from Markdown",
	}
	cmd.AddCommand(
		newPageCreateCmd(deps),
		newPageUpdateCmd(deps),
		newPageDeleteCmd(deps),
	)
	return cmd
}

func newPageCreateCmd(deps Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "create <file.md> <url>",
		Short: "Create a page under <url> (updates it if a page with the same title already exists)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			parent, err := confluence.ParseRef(args[1])
			if err != nil {
				return err
			}
			content, err := app.RenderFile(args[0])
			if err != nil {
				return err
			}
			service, host, err := deps.Service(parent.Host)
			if err != nil {
				return err
			}
			result, err := service.Create(cmd.Context(), app.CreateInput{
				Content: content,
				Parent:  parent,
				Opts:    deps.Options(),
			})
			if err != nil {
				return err
			}
			printResult(result, host)
			return nil
		},
	}
}

func newPageUpdateCmd(deps Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "update <file.md> <url>",
		Short: "Update the page at <url> in place (creates it in the URL's space if the id is gone)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			target, err := confluence.ParseRef(args[1])
			if err != nil {
				return err
			}
			content, err := app.RenderFile(args[0])
			if err != nil {
				return err
			}
			service, host, err := deps.Service(target.Host)
			if err != nil {
				return err
			}
			result, err := service.Update(cmd.Context(), app.UpdateInput{
				Content: content,
				Target:  target,
				Opts:    deps.Options(),
			})
			if err != nil {
				return err
			}
			printResult(result, host)
			return nil
		},
	}
}

func newPageDeleteCmd(deps Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <url>",
		Short: "Move a page to the Confluence trash (reversible)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target, err := confluence.ParseRef(args[0])
			if err != nil {
				return err
			}
			service, host, err := deps.Service(target.Host)
			if err != nil {
				return err
			}
			result, err := service.Delete(cmd.Context(), app.DeleteInput{
				Target: target,
				Opts:   deps.Options(),
			})
			if err != nil {
				return err
			}
			printResult(result, host)
			return nil
		},
	}
}
