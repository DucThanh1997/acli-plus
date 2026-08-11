package config

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

// newTestStore points a credential store at a throwaway directory.
func newTestStore(t *testing.T) *CredentialStore {
	t.Helper()
	return &CredentialStore{path: filepath.Join(t.TempDir(), "credentials.yaml")}
}

func TestCredentialStoreRoundTrip(t *testing.T) {
	store := newTestStore(t)

	if _, ok, err := store.Get("acme.atlassian.net"); err != nil || ok {
		t.Fatalf("a missing store should read as empty, got ok=%v err=%v", ok, err)
	}

	cred := Credential{Email: "me@acme.com", Token: "secret"}
	if err := store.Save("acme.atlassian.net", cred); err != nil {
		t.Fatal(err)
	}
	got, ok, err := store.Get("acme.atlassian.net")
	if err != nil || !ok {
		t.Fatalf("Get after Save: ok=%v err=%v", ok, err)
	}
	if got != cred {
		t.Errorf("credential = %+v, want %+v", got, cred)
	}

	// Saving the same host again must update it rather than duplicate it.
	if err := store.Save("acme.atlassian.net", Credential{Email: "new@acme.com", Token: "t2"}); err != nil {
		t.Fatal(err)
	}
	hosts, err := store.Hosts()
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) != 1 {
		t.Errorf("hosts = %v, want one entry", hosts)
	}
}

func TestCredentialStoreHostsAreSorted(t *testing.T) {
	store := newTestStore(t)
	for _, host := range []string{"zulu.atlassian.net", "alpha.atlassian.net", "mike.atlassian.net"} {
		if err := store.Save(host, Credential{Email: "me@acme.com", Token: "t"}); err != nil {
			t.Fatal(err)
		}
	}
	hosts, err := store.Hosts()
	if err != nil {
		t.Fatal(err)
	}
	want := "alpha.atlassian.net,mike.atlassian.net,zulu.atlassian.net"
	if strings.Join(hosts, ",") != want {
		t.Errorf("hosts = %v, want %s", hosts, want)
	}
}

// TestResolveFallsBackToTheOnlyHost covers the case that makes the Jira
// commands usable: they carry no URL, so with one registered site nothing
// should have to name it.
func TestResolveFallsBackToTheOnlyHost(t *testing.T) {
	store := newTestStore(t)
	if err := store.Save("acme.atlassian.net", Credential{Email: "me@acme.com", Token: "t"}); err != nil {
		t.Fatal(err)
	}

	resolved, err := Resolve(store, "", "", "", Project{})
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Host != "acme.atlassian.net" || resolved.Email != "me@acme.com" {
		t.Errorf("resolved = %+v", resolved)
	}
}

func TestResolvePrecedenceBeatsTheFallback(t *testing.T) {
	store := newTestStore(t)
	for _, host := range []string{"acme.atlassian.net", "other.atlassian.net"} {
		if err := store.Save(host, Credential{Email: "me@acme.com", Token: "t"}); err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		name    string
		urlHost string
		site    string
		env     string
		project Project
		want    string
	}{
		{name: "url host", urlHost: "other.atlassian.net", want: "other.atlassian.net"},
		{name: "--site", site: "acme.atlassian.net", want: "acme.atlassian.net"},
		{name: "env", env: "other.atlassian.net", want: "other.atlassian.net"},
		{name: "project file", project: Project{Site: "acme.atlassian.net"}, want: "acme.atlassian.net"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resolved, err := Resolve(store, tc.urlHost, tc.site, tc.env, tc.project)
			if err != nil {
				t.Fatal(err)
			}
			if resolved.Host != tc.want {
				t.Errorf("host = %q, want %q", resolved.Host, tc.want)
			}
		})
	}
}

func TestResolveWithSeveralHostsAsksWhichOne(t *testing.T) {
	store := newTestStore(t)
	for _, host := range []string{"acme.atlassian.net", "other.atlassian.net"} {
		if err := store.Save(host, Credential{Email: "me@acme.com", Token: "t"}); err != nil {
			t.Fatal(err)
		}
	}

	_, err := Resolve(store, "", "", "", Project{})
	if !errors.Is(err, ErrAmbiguousSite) {
		t.Fatalf("error = %v, want ErrAmbiguousSite", err)
	}
	// The message has to name the candidates, or the user cannot act on it.
	for _, host := range []string{"acme.atlassian.net", "other.atlassian.net"} {
		if !strings.Contains(err.Error(), host) {
			t.Errorf("error = %v, want it to list %s", err, host)
		}
	}
}

func TestResolveWithNoHostsPointsAtSetup(t *testing.T) {
	_, err := Resolve(newTestStore(t), "", "", "", Project{})
	if !errors.Is(err, ErrNoSite) {
		t.Fatalf("error = %v, want ErrNoSite", err)
	}
	if !strings.Contains(err.Error(), "acli-plus setup") {
		t.Errorf("error = %v, want it to point at setup", err)
	}
}

func TestResolveUnregisteredHost(t *testing.T) {
	store := newTestStore(t)
	if err := store.Save("acme.atlassian.net", Credential{Email: "me@acme.com", Token: "t"}); err != nil {
		t.Fatal(err)
	}

	_, err := Resolve(store, "", "nope.atlassian.net", "", Project{})
	if !errors.Is(err, ErrNoCredentials) {
		t.Errorf("error = %v, want ErrNoCredentials", err)
	}
}
