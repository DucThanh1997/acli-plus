package jira

import (
	"errors"
	"testing"
)

func TestParseRef(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantKey  string
		wantHost string
		wantErr  bool
	}{
		{name: "bare key", input: "TEAM-123", wantKey: "TEAM-123"},
		{name: "lower case key is upper-cased", input: "team-123", wantKey: "TEAM-123"},
		{name: "key with digits and underscore", input: "AB1_C-7", wantKey: "AB1_C-7"},
		{
			name:     "browse url",
			input:    "https://acme.atlassian.net/browse/TEAM-123",
			wantKey:  "TEAM-123",
			wantHost: "acme.atlassian.net",
		},
		{
			name:     "board url with selectedIssue",
			input:    "https://acme.atlassian.net/jira/software/projects/TEAM/boards/1?selectedIssue=TEAM-9",
			wantKey:  "TEAM-9",
			wantHost: "acme.atlassian.net",
		},
		{
			name:     "issues path form",
			input:    "https://acme.atlassian.net/jira/software/projects/TEAM/issues/TEAM-42",
			wantKey:  "TEAM-42",
			wantHost: "acme.atlassian.net",
		},
		{name: "empty", input: "", wantErr: true},
		{name: "not a key", input: "TEAM123", wantErr: true},
		{name: "no number", input: "TEAM-", wantErr: true},
		{name: "url without a key", input: "https://acme.atlassian.net/jira/dashboards", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ref, err := ParseRef(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ParseRef(%q) = %+v, want an error", tc.input, ref)
				}
				if !errors.Is(err, ErrInvalidKey) {
					t.Errorf("error = %v, want it to wrap ErrInvalidKey", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if ref.Key != tc.wantKey {
				t.Errorf("key = %q, want %q", ref.Key, tc.wantKey)
			}
			if ref.Host != tc.wantHost {
				t.Errorf("host = %q, want %q", ref.Host, tc.wantHost)
			}
		})
	}
}

func TestParseRefs(t *testing.T) {
	t.Run("comma-separated list", func(t *testing.T) {
		refs, err := ParseRefs("TEAM-1, TEAM-2 ,TEAM-3")
		if err != nil {
			t.Fatal(err)
		}
		got := Keys(refs)
		want := []string{"TEAM-1", "TEAM-2", "TEAM-3"}
		if len(got) != len(want) {
			t.Fatalf("keys = %v, want %v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("keys = %v, want %v", got, want)
				break
			}
		}
	})

	t.Run("one bad entry fails the whole list", func(t *testing.T) {
		if _, err := ParseRefs("TEAM-1,nope"); err == nil {
			t.Error("want an error for a list holding an invalid key")
		}
	})

	t.Run("empty list", func(t *testing.T) {
		if _, err := ParseRefs(" , "); err == nil {
			t.Error("want an error when no keys are given")
		}
	})
}

func TestHostOf(t *testing.T) {
	t.Run("first url wins", func(t *testing.T) {
		refs := []Ref{{Key: "TEAM-1"}, {Key: "TEAM-2", Host: "acme.atlassian.net"}}
		if got := HostOf(refs); got != "acme.atlassian.net" {
			t.Errorf("HostOf = %q, want acme.atlassian.net", got)
		}
	})

	t.Run("bare keys leave the host to config", func(t *testing.T) {
		if got := HostOf([]Ref{{Key: "TEAM-1"}}); got != "" {
			t.Errorf("HostOf = %q, want empty", got)
		}
	})
}

func TestBrowseURL(t *testing.T) {
	if got := BrowseURL("acme.atlassian.net", "TEAM-1"); got != "https://acme.atlassian.net/browse/TEAM-1" {
		t.Errorf("BrowseURL = %q", got)
	}
	if got := BrowseURL("", "TEAM-1"); got != "" {
		t.Errorf("BrowseURL without a host = %q, want empty", got)
	}
}
