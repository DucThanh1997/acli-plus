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
		Short: "Register a Confluence site (URL, email, API token)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runSetup(cmd.Context())
		},
	}
}

func runSetup(ctx context.Context) error {
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Confluence site URL (e.g. https://your-team.atlassian.net): ")
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

	service := app.NewSetupService(store, gatewayFactory)
	if err := service.Run(ctx, app.SetupInput{Host: host, Email: email, Token: token}); err != nil {
		return err
	}

	fmt.Printf("Saved credentials for %s to %s\n", host, store.Path())
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
