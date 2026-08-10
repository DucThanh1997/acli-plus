package jira

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// acliCommands is every command Atlassian's acli exposes under "acli jira".
// The tree here is meant to be a drop-in replacement, so this list is the
// parity contract: a missing entry means a user's muscle memory breaks.
var acliCommands = []string{
	"workitem archive",
	"workitem assign",
	"workitem attachment-delete",
	"workitem attachment-list",
	"workitem clone",
	"workitem comment-create",
	"workitem comment-delete",
	"workitem comment-list",
	"workitem comment-update",
	"workitem comment-visibility",
	"workitem create",
	"workitem create-bulk",
	"workitem delete",
	"workitem edit",
	"workitem link",
	"workitem search",
	"workitem transition",
	"workitem unarchive",
	"workitem view",
	"workitem watcher-remove",

	"project archive",
	"project create",
	"project delete",
	"project list",
	"project restore",
	"project update",
	"project view",

	"board list-sprints",
	"board search",

	"sprint list-workitems",

	"filter add-favourite",
	"filter change-owner",
	"filter list",
	"filter search",

	"dashboard search",

	"field cancel-delete",
	"field create",
	"field delete",
}

// extraCommands are the additions to acli's surface. Each one exists because a
// listed command is hard to use without it: you need a field id before you can
// delete a field, and a watcher's account before you can remove one.
var extraCommands = []string{
	"field list",
	"workitem watcher-list",
}

func TestCommandTreeMatchesACLI(t *testing.T) {
	root := NewCommand(Deps{})

	for _, path := range append(append([]string{}, acliCommands...), extraCommands...) {
		t.Run(path, func(t *testing.T) {
			if findCommand(root, strings.Fields(path)) == nil {
				t.Errorf("acli-plus jira %s is missing", path)
			}
		})
	}
}

// TestEveryCommandRuns checks that no command was added without a handler,
// which Cobra would otherwise report only at runtime as a usage dump.
func TestEveryCommandRuns(t *testing.T) {
	root := NewCommand(Deps{})
	var walk func(cmd *cobra.Command)
	walk = func(cmd *cobra.Command) {
		children := cmd.Commands()
		if len(children) == 0 && cmd.RunE == nil && cmd.Run == nil {
			t.Errorf("%q is a leaf command with no handler", cmd.CommandPath())
		}
		for _, child := range children {
			walk(child)
		}
	}
	walk(root)
}

func TestSingleKey(t *testing.T) {
	key, host, err := singleKey("https://acme.atlassian.net/browse/TEAM-5")
	if err != nil {
		t.Fatal(err)
	}
	if key != "TEAM-5" || host != "acme.atlassian.net" {
		t.Errorf("key = %q, host = %q", key, host)
	}
	if _, _, err := singleKey("not a key"); err == nil {
		t.Error("want an error for a value that is neither a key nor a URL")
	}
}

func TestTargetFlagsResolve(t *testing.T) {
	t.Run("positional args and --key are combined", func(t *testing.T) {
		flags := targetFlags{keys: []string{"TEAM-2,TEAM-3"}}
		targets, _, err := flags.resolve([]string{"TEAM-1"})
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"TEAM-1", "TEAM-2", "TEAM-3"}
		if strings.Join(targets.Keys, ",") != strings.Join(want, ",") {
			t.Errorf("keys = %v, want %v", targets.Keys, want)
		}
	})

	t.Run("a URL selects the site", func(t *testing.T) {
		var flags targetFlags
		_, host, err := flags.resolve([]string{"https://acme.atlassian.net/browse/TEAM-1"})
		if err != nil {
			t.Fatal(err)
		}
		if host != "acme.atlassian.net" {
			t.Errorf("host = %q", host)
		}
	})

	t.Run("--jql alone is enough", func(t *testing.T) {
		flags := targetFlags{jql: "project = TEAM"}
		targets, _, err := flags.resolve(nil)
		if err != nil {
			t.Fatal(err)
		}
		if targets.JQL == "" {
			t.Error("JQL should reach the service")
		}
	})

	t.Run("selecting nothing is an error", func(t *testing.T) {
		var flags targetFlags
		if _, _, err := flags.resolve(nil); err == nil {
			t.Error("want an error when nothing is selected")
		}
	})
}

func TestSplitEditorBuffer(t *testing.T) {
	t.Run("first line is the summary, the rest the description", func(t *testing.T) {
		summary, body, err := splitEditorBuffer("# a comment\nFix login\n\nSome **details**\n")
		if err != nil {
			t.Fatal(err)
		}
		if summary != "Fix login" {
			t.Errorf("summary = %q", summary)
		}
		if body != "Some **details**" {
			t.Errorf("body = %q", body)
		}
	})

	t.Run("an empty buffer aborts", func(t *testing.T) {
		if _, _, err := splitEditorBuffer("# only comments\n\n"); err == nil {
			t.Error("want an error when the editor was left empty")
		}
	})
}

// findCommand walks the tree by command name.
func findCommand(cmd *cobra.Command, path []string) *cobra.Command {
	if len(path) == 0 {
		return cmd
	}
	for _, child := range cmd.Commands() {
		if child.Name() == path[0] {
			return findCommand(child, path[1:])
		}
	}
	return nil
}
