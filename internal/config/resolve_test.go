package config

import "testing"

func TestResolveHost(t *testing.T) {
	tests := []struct {
		name    string
		urlHost string
		site    string
		env     string
		project Project
		want    string
	}{
		{
			name:    "URL host is authoritative over project config",
			urlHost: "acme.atlassian.net",
			project: Project{Site: "other.atlassian.net"},
			want:    "acme.atlassian.net",
		},
		{
			name:    "falls back to project site (full URL reduced to host)",
			project: Project{Site: "https://proj.atlassian.net/wiki"},
			want:    "proj.atlassian.net",
		},
		{
			name:    "flag beats env and project",
			site:    "flag.atlassian.net",
			env:     "env.atlassian.net",
			project: Project{Site: "proj.atlassian.net"},
			want:    "flag.atlassian.net",
		},
		{
			name:    "env beats project",
			env:     "env.atlassian.net",
			project: Project{Site: "proj.atlassian.net"},
			want:    "env.atlassian.net",
		},
		{
			name: "nothing configured",
			want: "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ResolveHost(tc.urlHost, tc.site, tc.env, tc.project)
			if got != tc.want {
				t.Errorf("ResolveHost = %q, want %q", got, tc.want)
			}
		})
	}
}
