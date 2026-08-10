package app

import (
	"context"
	"fmt"
	"strings"

	"acli-plus/internal/config"
	confluence "acli-plus/internal/domain/confluence"
	jira "acli-plus/internal/domain/jira"
)

// GatewayFactory builds a Confluence gateway for a host and credentials. It is
// injected so setup can verify credentials before storing them (and tests can
// stub it).
type GatewayFactory func(host, email, token string) confluence.Gateway

// JiraGatewayFactory is the same idea for Jira.
type JiraGatewayFactory func(host, email, token string) jira.Gateway

// ProductVerifier checks the credentials against one Atlassian product.
type ProductVerifier struct {
	Name   string
	Verify func(ctx context.Context, host, email, token string) error
}

// ConfluenceVerifier adapts a Confluence gateway factory to a verifier.
func ConfluenceVerifier(factory GatewayFactory) ProductVerifier {
	return ProductVerifier{
		Name: "Confluence",
		Verify: func(ctx context.Context, host, email, token string) error {
			return factory(host, email, token).VerifyAuth(ctx)
		},
	}
}

// JiraVerifier adapts a Jira gateway factory to a verifier.
func JiraVerifier(factory JiraGatewayFactory) ProductVerifier {
	return ProductVerifier{
		Name: "Jira",
		Verify: func(ctx context.Context, host, email, token string) error {
			return factory(host, email, token).VerifyAuth(ctx)
		},
	}
}

// SetupService verifies and stores per-host credentials.
type SetupService struct {
	store     *config.CredentialStore
	verifiers []ProductVerifier
}

// NewSetupService wires the service to the credential store and the products to
// check the credentials against.
func NewSetupService(store *config.CredentialStore, verifiers ...ProductVerifier) *SetupService {
	return &SetupService{store: store, verifiers: verifiers}
}

// SetupInput is the data collected interactively by the setup command.
type SetupInput struct {
	Host  string
	Email string
	Token string
}

// SetupOutcome reports which products accepted the credentials, so setup can
// tell the user what they just gained access to.
type SetupOutcome struct {
	Reachable []string
}

// Run verifies the credentials, then stores them keyed by host. One Atlassian
// site serves both products from the same account and API token, but a site may
// license only one of them — so the credentials are accepted when *any* product
// answers, and setup reports which ones did.
func (s *SetupService) Run(ctx context.Context, in SetupInput) (SetupOutcome, error) {
	var outcome SetupOutcome
	failures := make([]string, 0, len(s.verifiers))

	for _, verifier := range s.verifiers {
		if err := verifier.Verify(ctx, in.Host, in.Email, in.Token); err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", verifier.Name, err))
			continue
		}
		outcome.Reachable = append(outcome.Reachable, verifier.Name)
	}

	if len(outcome.Reachable) == 0 {
		return outcome, fmt.Errorf("verifying credentials for %s: %s",
			in.Host, strings.Join(failures, "; "))
	}
	if err := s.store.Save(in.Host, config.Credential{Email: in.Email, Token: in.Token}); err != nil {
		return outcome, err
	}
	return outcome, nil
}
