package config

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// ErrNoCredentials indicates the resolved host has no stored credentials.
var ErrNoCredentials = errors.New("no credentials for host; run 'acli-plus setup'")

// ErrNoSite indicates no host could be determined from any source.
var ErrNoSite = errors.New("no Atlassian site specified and none registered: run 'acli-plus setup', or pass a URL, --site, ACLI_PLUS_SITE, or set 'site' in acli-plus.yaml")

// ErrAmbiguousSite indicates several sites are registered and nothing said
// which one to use.
var ErrAmbiguousSite = errors.New("several sites are registered; pass --site to choose one")

// Resolved carries everything needed to build a Confluence gateway.
type Resolved struct {
	Host  string
	Email string
	Token string
}

// ResolveHost picks the target host with precedence:
// explicit URL host > --site flag > env var > project config. It returns "" if
// none is available. Any full URL or bare host is reduced to its host component.
func ResolveHost(urlHost, siteFlag, envSite string, project Project) string {
	for _, candidate := range []string{urlHost, siteFlag, envSite, project.Site} {
		if host := normalizeHost(candidate); host != "" {
			return host
		}
	}
	return ""
}

// Resolve determines the host and looks up its stored credentials. When
// nothing names a site it falls back to the only registered one: most Jira
// commands carry no URL to take a host from, and a machine set up against a
// single site should not have to repeat --site on every call.
func Resolve(store *CredentialStore, urlHost, siteFlag, envSite string, project Project) (Resolved, error) {
	host := ResolveHost(urlHost, siteFlag, envSite, project)
	if host == "" {
		only, err := onlyRegisteredHost(store)
		if err != nil {
			return Resolved{}, err
		}
		host = only
	}
	cred, ok, err := store.Get(host)
	if err != nil {
		return Resolved{}, err
	}
	if !ok {
		return Resolved{}, fmt.Errorf("%w (host %s)", ErrNoCredentials, host)
	}
	return Resolved{Host: host, Email: cred.Email, Token: cred.Token}, nil
}

// onlyRegisteredHost returns the sole registered host. Falling back to it is
// only safe when there is exactly one; with several, guessing could write to
// the wrong site, so the caller is asked to choose.
func onlyRegisteredHost(store *CredentialStore) (string, error) {
	hosts, err := store.Hosts()
	if err != nil {
		return "", err
	}
	switch len(hosts) {
	case 1:
		return hosts[0], nil
	case 0:
		return "", ErrNoSite
	default:
		return "", fmt.Errorf("%w (registered: %s)", ErrAmbiguousSite, strings.Join(hosts, ", "))
	}
}

func normalizeHost(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if !strings.Contains(value, "://") {
		value = "https://" + value
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" {
		return ""
	}
	return parsed.Host
}
