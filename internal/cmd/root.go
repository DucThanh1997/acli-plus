// Package cmd wires the Cobra command tree (the handler layer): it parses args
// and flags, builds the application services, and formats results.
package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"acli-plus/internal/app"
	"acli-plus/internal/config"
	confluence "acli-plus/internal/domain/confluence"
	"acli-plus/internal/gateway/confluencerest"
)

const envSite = "ACLI_PLUS_SITE"

type globalOptions struct {
	dryRun bool
	yes    bool
	force  bool
	site   string
}

var globals globalOptions

// Execute runs the root command.
func Execute() error {
	return newRootCmd().Execute()
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "acli-plus",
		Short:         "Publish Markdown files to Confluence (the Confluence commands acli is missing)",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	flags := root.PersistentFlags()
	flags.BoolVar(&globals.dryRun, "dry-run", false, "show what would happen without writing")
	flags.BoolVar(&globals.yes, "yes", false, "skip confirmation prompts")
	flags.BoolVar(&globals.force, "force", false, "overwrite even if the page was modified outside acli-plus")
	flags.StringVar(&globals.site, "site", "", "Confluence site (host or URL) to use when no URL is given")

	root.AddCommand(
		newSetupCmd(),
		newCreateCmd(),
		newUpdateCmd(),
		newDeleteCmd(),
		newVersionCmd(),
	)
	return root
}

// gatewayFactory builds the concrete Confluence adapter (used as app.GatewayFactory).
func gatewayFactory(host, email, token string) confluence.Gateway {
	return confluencerest.New(host, email, token)
}

// buildPageService resolves credentials for the target host and returns a ready
// PageService plus the resolved host (for building output links).
func buildPageService(urlHost string) (*app.PageService, string, error) {
	store, err := config.NewCredentialStore()
	if err != nil {
		return nil, "", err
	}
	project, err := config.LoadProject(".")
	if err != nil {
		return nil, "", err
	}
	resolved, err := config.Resolve(store, urlHost, globals.site, os.Getenv(envSite), project)
	if err != nil {
		return nil, "", err
	}
	return app.NewPageService(gatewayFactory(resolved.Host, resolved.Email, resolved.Token)), resolved.Host, nil
}

func writeOptions() app.WriteOptions {
	return app.WriteOptions{
		DryRun:      globals.dryRun,
		SkipConfirm: globals.yes || globals.force,
		Confirm:     confirmFromStdin,
	}
}

func confirmFromStdin(prompt string) (bool, error) {
	fmt.Fprintf(os.Stderr, "%s [y/N]: ", prompt)
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && line == "" {
		return false, nil
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes", nil
}

func printResult(res app.Result, host string) {
	for _, warning := range res.Warnings {
		fmt.Fprintln(os.Stderr, "warning:", warning)
	}
	prefix := ""
	if res.DryRun {
		prefix = "[dry-run] "
	}
	switch res.Action {
	case app.ActionCreate:
		fmt.Printf("%screated %q%s\n", prefix, res.Page.Title, pageLink(host, res.Page))
	case app.ActionUpdate:
		fmt.Printf("%supdated %q%s\n", prefix, res.Page.Title, pageLink(host, res.Page))
	case app.ActionDelete:
		fmt.Printf("%sdeleted (moved to trash) %q\n", prefix, res.Page.Title)
	case app.ActionAborted:
		fmt.Println("aborted; no changes made")
	}
}

func pageLink(host string, page confluence.Page) string {
	if host == "" || page.ID == "" {
		return ""
	}
	return fmt.Sprintf(" -> https://%s/wiki/pages/viewpage.action?pageId=%s", host, page.ID)
}
