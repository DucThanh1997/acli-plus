// Package confluence wires the Cobra command tree for Confluence. It is shaped
// like the jira package — a product namespace, then a noun, then a verb — so
// both halves of the CLI read the same way, and depends on the application layer
// only through Deps.
package confluence

import (
	"github.com/spf13/cobra"

	"acli-plus/internal/app"
)

// Deps is what the root command supplies. Service resolves credentials for the
// site (hostHint comes from the page URL) and returns the resolved host for
// building output links; Options carries the global --dry-run/--yes flags.
type Deps struct {
	Service func(hostHint string) (*app.PageService, string, error)
	Options func() app.WriteOptions
}

// NewCommand builds the "acli-plus confluence" command tree.
func NewCommand(deps Deps) *cobra.Command {
	root := &cobra.Command{
		Use:   "confluence",
		Short: "Work with Confluence: publish Markdown files as pages",
		Long: "Work with Confluence.\n\n" +
			"Authentication is the credential you already registered with\n" +
			"'acli-plus setup' — Jira and Confluence share one site, one account,\n" +
			"and one API token.",
	}
	root.AddCommand(newPageCmd(deps))
	return root
}
