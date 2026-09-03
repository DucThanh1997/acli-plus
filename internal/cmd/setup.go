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

// setupFlags are the non-interactive inputs. Any value left empty is prompted
// for, so the same command serves an interactive first run and a provisioning
// script that already holds the credential.
type setupFlags struct {
	site  string
	email string
	token string
}

func newSetupCmd() *cobra.Command {
	var f setupFlags
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Register an Atlassian site (URL, email, API token) for Confluence and Jira",
		Long: "Register an Atlassian site.\n\n" +
			"One site, one account, and one API token cover both Confluence and Jira,\n" +
			"so this is the only credential either set of commands needs. The token is\n" +
			"checked against both products and stored per host.\n\n" +
			"With no flags it prompts for all three. Pass --site/--email/--token to skip\n" +
			"the matching prompt, which is what a provisioning script wants. Prefer\n" +
			"piping the token instead of passing --token, so it never appears in the\n" +
			"process list:\n\n" +
			"  printf '%s' \"$TOKEN\" | acli-plus setup --site acme.atlassian.net --email you@acme.com",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runSetup(cmd.Context(), f)
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&f.site, "site", "", "Atlassian site (host or URL); prompted for when omitted")
	flags.StringVar(&f.email, "email", "", "Atlassian account email; prompted for when omitted")
	flags.StringVar(&f.token, "token", "", "API token; prompted for (or read from piped stdin) when omitted")
	return cmd
}

func runSetup(ctx context.Context, f setupFlags) error {
	reader := bufio.NewReader(os.Stdin)

	siteInput := strings.TrimSpace(f.site)
	if siteInput == "" {
		fmt.Print("Atlassian site URL (e.g. https://your-team.atlassian.net): ")
		line, _ := reader.ReadString('\n')
		siteInput = strings.TrimSpace(line)
	}
	host := config.ResolveHost(siteInput, "", "", config.Project{})
	if host == "" {
		return fmt.Errorf("could not parse a host from %q", siteInput)
	}

	email := strings.TrimSpace(f.email)
	if email == "" {
		fmt.Print("Atlassian account email: ")
		line, _ := reader.ReadString('\n')
		email = strings.TrimSpace(line)
	}
	if email == "" {
		return fmt.Errorf("email is required (pass --email or answer the prompt)")
	}

	token := strings.TrimSpace(f.token)
	if token == "" {
		// No --token: prompt on a terminal, or take the piped line in a script.
		// The prompt is only printed when someone is there to read it, so script
		// output stays clean.
		interactive := term.IsTerminal(int(os.Stdin.Fd()))
		if interactive {
			fmt.Printf("API token (create at %s): ", apiTokenURL)
		}
		secret, err := readSecret(reader)
		if interactive {
			fmt.Println()
		}
		if err != nil {
			// readSecret only errors when it read nothing at all, so this is an
			// empty or closed stdin — what a provisioning script hits when it
			// forgets to pipe the token. The "API token is required" check below
			// says what to do about it; a bare "EOF" would not.
			secret = ""
		}
		token = secret
	}
	if token == "" {
		return fmt.Errorf("API token is required (pass --token, pipe it on stdin, or answer the prompt)")
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
