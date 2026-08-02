package app

import (
	"context"
	"fmt"

	"acli-plus/internal/config"
	confluence "acli-plus/internal/domain/confluence"
)

// GatewayFactory builds a gateway for a host and credentials. It is injected so
// setup can verify credentials before storing them (and tests can stub it).
type GatewayFactory func(host, email, token string) confluence.Gateway

// SetupService verifies and stores per-host credentials.
type SetupService struct {
	store   *config.CredentialStore
	newGate GatewayFactory
}

// NewSetupService wires the service to the credential store and a gateway factory.
func NewSetupService(store *config.CredentialStore, factory GatewayFactory) *SetupService {
	return &SetupService{store: store, newGate: factory}
}

// SetupInput is the data collected interactively by the setup command.
type SetupInput struct {
	Host  string
	Email string
	Token string
}

// Run verifies the credentials against the site, then stores them keyed by host.
func (s *SetupService) Run(ctx context.Context, in SetupInput) error {
	gateway := s.newGate(in.Host, in.Email, in.Token)
	if err := gateway.VerifyAuth(ctx); err != nil {
		return fmt.Errorf("verifying credentials for %s: %w", in.Host, err)
	}
	return s.store.Save(in.Host, config.Credential{Email: in.Email, Token: in.Token})
}
