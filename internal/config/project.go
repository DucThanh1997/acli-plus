package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Project is the optional, non-secret per-project config (acli-plus.yaml).
// It never holds a token.
type Project struct {
	Site   string `yaml:"site"`
	Space  string `yaml:"space"`
	Parent string `yaml:"parent"`
	// JiraProject is the default project key for 'jira workitem create' and
	// 'jira board search', so a repo pinned to one Jira project need not repeat
	// --project on every command.
	JiraProject string `yaml:"jira_project"`
	// JiraBoard is the default board id or name for the board and sprint commands.
	JiraBoard string `yaml:"jira_board"`
}

const projectFileName = "acli-plus.yaml"

// LoadProject reads acli-plus.yaml from dir. Absence is not an error.
func LoadProject(dir string) (Project, error) {
	path := filepath.Join(dir, projectFileName)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Project{}, nil
	}
	if err != nil {
		return Project{}, fmt.Errorf("reading %s: %w", projectFileName, err)
	}
	var project Project
	if err := yaml.Unmarshal(data, &project); err != nil {
		return Project{}, fmt.Errorf("parsing %s: %w", projectFileName, err)
	}
	return project, nil
}
