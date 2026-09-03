package cmd

import (
	"strings"
	"testing"
)

// The flags exist so a provisioning script can register a site without a
// terminal. These tests pin the flag surface and the "flag wins over prompt"
// rule; the interactive path is covered by the prompts themselves.
func TestSetupCmdFlags(t *testing.T) {
	cmd := newSetupCmd()
	for _, name := range []string{"site", "email", "token"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("setup is missing the --%s flag", name)
		}
	}
}

func TestSetupLongHelpShowsThePipedForm(t *testing.T) {
	// Passing --token puts the secret in the process list, so the help has to
	// point at the piped form instead of only documenting the flag.
	long := newSetupCmd().Long
	if !strings.Contains(long, "printf") || !strings.Contains(long, "| acli-plus setup") {
		t.Errorf("setup --help does not show the piped-token form:\n%s", long)
	}
}

func TestSetupRejectsAnUnparseableSite(t *testing.T) {
	// "http://" is non-empty — so the prompt is skipped and the flag is genuinely
	// what fails — but carries no host. A blank value would instead fall through
	// to the prompt and read stdin, which is a different code path.
	err := runSetup(t.Context(), setupFlags{site: "http://", email: "a@b.com", token: "x"})
	if err == nil {
		t.Fatal("want an error for a site with no host in it")
	}
	if !strings.Contains(err.Error(), "could not parse a host") {
		t.Errorf("error = %v, want it to name the unparseable host", err)
	}
}

func TestSetupRequiresAToken(t *testing.T) {
	// Every input supplied by flag, so nothing here touches stdin; an empty
	// --token must be refused rather than silently stored.
	err := runSetup(t.Context(), setupFlags{site: "acme.atlassian.net", email: "a@b.com", token: "   "})
	if err == nil || !strings.Contains(err.Error(), "API token is required") {
		t.Errorf("error = %v, want a token-required error", err)
	}
}
