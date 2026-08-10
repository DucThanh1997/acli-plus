package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"acli-plus/internal/app"
	"acli-plus/internal/config"
)

const apiTokenURL = "https://id.atlassian.com/manage-profile/security/api-tokens"

func newSetupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "setup",
		Short: "Register an Atlassian site (URL, email, API token) for Confluence and Jira",
		Long: "Register an Atlassian site.\n\n" +
			"One site, one account, and one API token cover both Confluence and Jira,\n" +
			"so this is the only credential either set of commands needs. The token is\n" +
			"checked against both products and stored per host.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runSetup(cmd.Context())
		},
	}
}

func runSetup(ctx context.Context) error {
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Atlassian site URL (e.g. https://your-team.atlassian.net): ")
	siteLine, _ := reader.ReadString('\n')
	host := config.ResolveHost(strings.TrimSpace(siteLine), "", "", config.Project{})
	if host == "" {
		return fmt.Errorf("could not parse a host from %q", strings.TrimSpace(siteLine))
	}

	fmt.Print("Atlassian account email: ")
	emailLine, _ := reader.ReadString('\n')
	email := strings.TrimSpace(emailLine)
	if email == "" {
		return fmt.Errorf("email is required")
	}

	fmt.Printf("API token (create at %s): ", apiTokenURL)
	token, err := readSecret(reader)
	fmt.Println()
	if err != nil {
		return err
	}
	if token == "" {
		return fmt.Errorf("API token is required")
	}

	store, err := config.NewCredentialStore()
	if err != nil {
		return err
	}

	service := app.NewSetupService(store,
		app.ConfluenceVerifier(gatewayFactory),
		app.JiraVerifier(jiraGatewayFactory),
	)
	outcome, err := service.Run(ctx, app.SetupInput{Host: host, Email: email, Token: token})
	if err != nil {
		return err
	}

	fmt.Printf("Saved credentials for %s to %s\n", host, store.Path())
	fmt.Printf("Reachable on this site: %s\n", strings.Join(outcome.Reachable, ", "))
	return nil
}

// readSecret reads a secret without echoing when stdin is a terminal, and falls
// back to a plain line read otherwise (e.g. piped input in scripts/tests).
func readSecret(reader *bufio.Reader) (string, error) {
	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		data, err := term.ReadPassword(fd)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(data)), nil
	}
	line, err := reader.ReadString('\n')
	if err != nil && line == "" {
		return "", err
	}
	return strings.TrimSpace(line), nil
}
