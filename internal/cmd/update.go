package cmd

import (
	"github.com/spf13/cobra"

	"acli-plus/internal/app"
	confluence "acli-plus/internal/domain/confluence"
)

func newUpdateCmd() *cobra.Command {
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
			service, host, err := buildPageService(target.Host)
			if err != nil {
				return err
			}
			result, err := service.Update(cmd.Context(), app.UpdateInput{
				Content: content,
				Target:  target,
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
