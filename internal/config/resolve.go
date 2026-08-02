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
var ErrNoSite = errors.New("no Confluence site specified: pass a URL, --site, ACLI_PLUS_SITE, or set 'site' in acli-plus.yaml")

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

// Resolve determines the host and looks up its stored credentials.
func Resolve(store *CredentialStore, urlHost, siteFlag, envSite string, project Project) (Resolved, error) {
	host := ResolveHost(urlHost, siteFlag, envSite, project)
	if host == "" {
		return Resolved{}, ErrNoSite
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
