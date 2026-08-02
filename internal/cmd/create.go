package cmd

import (
	"github.com/spf13/cobra"

	"acli-plus/internal/app"
	confluence "acli-plus/internal/domain/confluence"
)

func newCreateCmd() *cobra.Command {
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
			service, host, err := buildPageService(parent.Host)
			if err != nil {
				return err
			}
			result, err := service.Create(cmd.Context(), app.CreateInput{
				Content: content,
				Parent:  parent,
				Opts:    writeOptions(),
			})
			if err != nil {
				return err
			}
			printResult(result, host)
			return nil
		},
	}
}
