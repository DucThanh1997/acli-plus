// Package jira wires the Cobra command tree for Jira Cloud. It mirrors the
// command and flag names of Atlassian's own acli so the two are interchangeable,
// and depends on the application layer only through Deps.
package jira

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"acli-plus/internal/app"
	jiradomain "acli-plus/internal/domain/jira"
)

// Deps is what the root command supplies. Service resolves credentials for the
// site (hostHint comes from a work item URL when the user pasted one), Options
// carries the global --dry-run/--yes flags, and Defaults reads acli-plus.yaml.
type Deps struct {
	Service  func(hostHint string) (*app.JiraService, error)
	Options  func() app.WriteOptions
	Defaults func() Defaults
}

// Defaults are the optional per-project settings from acli-plus.yaml that save
// repeating the same flag in every command.
type Defaults struct {
	Project string
	Board   string
}

// defaults reads the project defaults, tolerating a Deps built without them
// (which is what tests do).
func (d Deps) defaults() Defaults {
	if d.Defaults == nil {
		return Defaults{}
	}
	return d.Defaults()
}

// NewCommand builds the "acli-plus jira" command tree.
func NewCommand(deps Deps) *cobra.Command {
	root := &cobra.Command{
		Use:   "jira",
		Short: "Work with Jira Cloud: work items, projects, boards, sprints, filters, dashboards",
		Long: "Work with Jira Cloud.\n\n" +
			"Commands and flags mirror Atlassian's acli, and authentication is the\n" +
			"credential you already registered with 'acli-plus setup' — Jira and\n" +
			"Confluence share one site, one account, and one API token.",
	}
	root.AddCommand(
		newWorkItemCmd(deps),
		newProjectCmd(deps),
		newFieldCmd(deps),
		newBoardCmd(deps),
		newSprintCmd(deps),
		newFilterCmd(deps),
		newDashboardCmd(deps),
	)
	return root
}

// targetFlags selects which work items a command acts on. Keys may be given as
// positional arguments, as --key (comma-separated or repeated), or implied by
// a --jql query.
type targetFlags struct {
	keys []string
	jql  string
}

func (t *targetFlags) register(cmd *cobra.Command, withJQL bool) {
	cmd.Flags().StringSliceVar(&t.keys, "key", nil, "work item key(s), comma-separated (e.g. TEAM-1,TEAM-2)")
	if withJQL {
		cmd.Flags().StringVar(&t.jql, "jql", "", "select work items with a JQL query instead of keys")
	}
}

// resolve parses every key it was given, returning the targets plus the host
// implied by any URL among them.
func (t *targetFlags) resolve(args []string) (app.Targets, string, error) {
	refs := make([]jiradomain.Ref, 0, len(args)+len(t.keys))
	for _, value := range append(append([]string{}, args...), t.keys...) {
		parsed, err := jiradomain.ParseRefs(value)
		if err != nil {
			return app.Targets{}, "", err
		}
		refs = append(refs, parsed...)
	}
	if len(refs) == 0 && strings.TrimSpace(t.jql) == "" {
		return app.Targets{}, "", fmt.Errorf("no work items selected: pass a key, --key, or --jql")
	}
	return app.Targets{Keys: jiradomain.Keys(refs), JQL: t.jql}, jiradomain.HostOf(refs), nil
}

// singleKey parses exactly one work item reference.
func singleKey(value string) (string, string, error) {
	ref, err := jiradomain.ParseRef(value)
	if err != nil {
		return "", "", err
	}
	return ref.Key, ref.Host, nil
}

// attributeFlags are the work item field flags shared by create, create-bulk,
// clone, edit and transition.
type attributeFlags struct {
	project     string
	typeName    string
	summary     string
	parent      string
	priority    string
	assignee    string
	reporter    string
	due         string
	labels      []string
	components  []string
	fixVersions []string
	fields      []string

	description     string
	descriptionFile string
	fromFile        string
	editor          bool
}

// register adds the field flags. core is set on commands that create a work
// item, which are the only ones that advertise --project and --type.
func (a *attributeFlags) register(cmd *cobra.Command, core bool) {
	flags := cmd.Flags()
	flags.StringVarP(&a.summary, "summary", "s", "", "work item summary")
	flags.StringVarP(&a.description, "description", "d", "", "description as plain text, Markdown, or ADF JSON")
	flags.StringVar(&a.descriptionFile, "description-file", "", "read the description from a Markdown or ADF file")
	flags.StringVarP(&a.fromFile, "from-file", "f", "", "read summary and description from a Markdown file (title or H1 is the summary)")
	flags.BoolVarP(&a.editor, "editor", "e", false, "open $EDITOR for the summary (first line) and description")
	flags.StringVarP(&a.assignee, "assignee", "a", "", "assignee by email, display name, or account id; @me for yourself")
	flags.StringVar(&a.reporter, "reporter", "", "reporter by email, display name, or account id")
	flags.StringSliceVarP(&a.labels, "label", "l", nil, "labels, comma-separated")
	flags.StringSliceVar(&a.components, "component", nil, "components, comma-separated")
	flags.StringSliceVar(&a.fixVersions, "fix-version", nil, "fix versions, comma-separated")
	flags.StringVar(&a.priority, "priority", "", "priority name (e.g. High)")
	flags.StringVar(&a.due, "due", "", "due date as YYYY-MM-DD")
	flags.StringVar(&a.parent, "parent", "", "parent work item key (for subtasks and items under an epic)")
	flags.StringArrayVar(&a.fields, "field", nil, "any other field as name=value; repeatable (e.g. --field \"Story Points=5\")")

	if core {
		flags.StringVarP(&a.project, "project", "p", "", "project key")
		flags.StringVarP(&a.typeName, "type", "t", "", "work item type (Epic, Story, Task, Bug, ...)")
	}
}

// attributes turns the flags into the application-layer input, running the
// editor when asked.
func (a *attributeFlags) attributes() (app.WorkItemAttributes, error) {
	attrs := app.WorkItemAttributes{
		Project:     a.project,
		Type:        a.typeName,
		Summary:     a.summary,
		Parent:      a.parent,
		Priority:    a.priority,
		Assignee:    a.assignee,
		Reporter:    a.reporter,
		Due:         a.due,
		Labels:      a.labels,
		Components:  a.components,
		FixVersions: a.fixVersions,
	}

	switch {
	case a.fromFile != "":
		attrs.Description = app.DescriptionSource{File: a.fromFile}
	case a.descriptionFile != "":
		attrs.Description = app.DescriptionSource{File: a.descriptionFile}
	case a.description != "":
		attrs.Description = app.DescriptionSource{Text: a.description}
	}

	if a.editor {
		summary, body, err := runEditor(a.summary, a.description)
		if err != nil {
			return app.WorkItemAttributes{}, err
		}
		if summary != "" {
			attrs.Summary = summary
		}
		attrs.Description = app.DescriptionSource{Text: body}
	}

	for _, raw := range a.fields {
		assignment, err := app.ParseFieldAssignment(raw)
		if err != nil {
			return app.WorkItemAttributes{}, err
		}
		attrs.Fields = append(attrs.Fields, assignment)
	}
	return attrs, nil
}

// editorTemplate explains the first-line-is-the-summary convention inside the
// buffer, and its lines are stripped before the text is used.
const editorTemplate = `
# Lines starting with # are ignored.
# The first non-empty line is the summary; everything after it is the
# description, which may use Markdown.
`

// runEditor opens $EDITOR (or vi) on a scratch file and splits what comes back
// into a summary and a description.
func runEditor(summary, description string) (string, string, error) {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}

	file, err := os.CreateTemp("", "acli-plus-*.md")
	if err != nil {
		return "", "", fmt.Errorf("creating scratch file: %w", err)
	}
	path := file.Name()
	defer os.Remove(path)

	initial := strings.TrimSpace(summary + "\n\n" + description)
	if _, err := file.WriteString(initial + "\n" + editorTemplate); err != nil {
		file.Close()
		return "", "", fmt.Errorf("writing scratch file: %w", err)
	}
	file.Close()

	parts := strings.Fields(editor)
	command := exec.Command(parts[0], append(parts[1:], path)...) //nolint:gosec // the editor comes from the user's own $EDITOR
	command.Stdin, command.Stdout, command.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := command.Run(); err != nil {
		return "", "", fmt.Errorf("running %s: %w", filepath.Base(parts[0]), err)
	}

	edited, err := os.ReadFile(path)
	if err != nil {
		return "", "", fmt.Errorf("reading scratch file: %w", err)
	}
	return splitEditorBuffer(string(edited))
}

// splitEditorBuffer drops comment lines, then takes the first non-empty line as
// the summary and the rest as the description.
func splitEditorBuffer(buffer string) (string, string, error) {
	var kept []string
	for _, line := range strings.Split(buffer, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		kept = append(kept, line)
	}

	for i, line := range kept {
		if strings.TrimSpace(line) == "" {
			continue
		}
		body := strings.TrimSpace(strings.Join(kept[i+1:], "\n"))
		return strings.TrimSpace(line), body, nil
	}
	return "", "", fmt.Errorf("nothing was written in the editor; aborting")
}
