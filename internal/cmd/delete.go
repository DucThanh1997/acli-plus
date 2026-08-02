package cmd

import (
	"github.com/spf13/cobra"

	"acli-plus/internal/app"
	confluence "acli-plus/internal/domain/confluence"
)

func newDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <url>",
		Short: "Move a page to the Confluence trash (reversible)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target, err := confluence.ParseRef(args[0])
			if err != nil {
				return err
			}
			service, host, err := buildPageService(target.Host)
			if err != nil {
				return err
			}
			result, err := service.Delete(cmd.Context(), app.DeleteInput{
				Target: target,
				Opts:   writeOptions(),
			})
			if err != nil {
				return err
			}
			printResult(result, host)
			return nil
		},
	}
}
