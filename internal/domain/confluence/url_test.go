package confluence

import (
	"errors"
	"testing"
)

func TestParseRef(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantHost string
		wantKey  string
		wantID   string
		wantErr  error
	}{
		{
			name:     "full page URL",
			input:    "https://acme.atlassian.net/wiki/spaces/DEV/pages/98765/Some+Title",
			wantHost: "acme.atlassian.net", wantKey: "DEV", wantID: "98765",
		},
		{
			name:     "page URL without title slug",
			input:    "https://acme.atlassian.net/wiki/spaces/DEV/pages/98765",
			wantHost: "acme.atlassian.net", wantKey: "DEV", wantID: "98765",
		},
		{
			name:     "space URL",
			input:    "https://acme.atlassian.net/wiki/spaces/DEV",
			wantHost: "acme.atlassian.net", wantKey: "DEV", wantID: "",
		},
		{
			name:     "space overview URL",
			input:    "https://acme.atlassian.net/wiki/spaces/DEV/overview",
			wantHost: "acme.atlassian.net", wantKey: "DEV", wantID: "",
		},
		{
			name:     "personal space key",
			input:    "https://acme.atlassian.net/wiki/spaces/~712020abc/pages/1/Home",
			wantHost: "acme.atlassian.net", wantKey: "~712020abc", wantID: "1",
		},
		{
			name:   "bare numeric page id",
			input:  "98765",
			wantID: "98765",
		},
		{
			name:    "short link is rejected",
			input:   "https://acme.atlassian.net/wiki/x/ABC123",
			wantErr: ErrShortLink,
		},
		{
			name:    "unrecognized URL",
			input:   "https://acme.atlassian.net/wiki/foo",
			wantErr: ErrInvalidRef,
		},
		{
			name:    "empty input",
			input:   "   ",
			wantErr: ErrInvalidRef,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseRef(tc.input)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("expected error %v, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Host != tc.wantHost || got.SpaceKey != tc.wantKey || got.PageID != tc.wantID {
				t.Errorf("got %+v, want host=%q key=%q id=%q", got, tc.wantHost, tc.wantKey, tc.wantID)
			}
		})
	}
}
