package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// version is the CLI version, overridable at build time via -ldflags.
var version = "0.1.0-dev"

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the acli-plus version",
		Args:  cobra.NoArgs,
		Run: func(_ *cobra.Command, _ []string) {
			fmt.Println("acli-plus", version)
		},
	}
}
