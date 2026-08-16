package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	jira "acli-plus/internal/domain/jira"
)

func TestResolveAccountID(t *testing.T) {
	t.Run("@me is the authenticated account", func(t *testing.T) {
		service := newTestService(&fakeJira{})
		got, err := service.ResolveAccountID(context.Background(), "@me")
		if err != nil {
			t.Fatal(err)
		}
		if got != "me-account-id" {
			t.Errorf("account id = %q, want me-account-id", got)
		}
	})

	t.Run("empty means unassigned", func(t *testing.T) {
		service := newTestService(&fakeJira{})
		for _, value := range []string{"", "none", "unassigned"} {
			got, err := service.ResolveAccountID(context.Background(), value)
			if err != nil {
				t.Fatal(err)
			}
			if got != "" {
				t.Errorf("ResolveAccountID(%q) = %q, want empty", value, got)
			}
		}
	})

	t.Run("an account id passes through without a lookup", func(t *testing.T) {
		gw := &fakeJira{findUsersFn: func(string) ([]jira.User, error) {
			t.Error("an account id should not trigger a user search")
			return nil, nil
		}}
		got, err := newTestService(gw).ResolveAccountID(context.Background(), "5b10a2844c20165700ede21g")
		if err != nil {
			t.Fatal(err)
		}
		if got != "5b10a2844c20165700ede21g" {
			t.Errorf("account id = %q", got)
		}
	})

	t.Run("email resolves through search", func(t *testing.T) {
		gw := &fakeJira{findUsersFn: func(string) ([]jira.User, error) {
			return []jira.User{{AccountID: "ann-id", Email: "ann@acme.com", DisplayName: "Ann"}}, nil
		}}
		got, err := newTestService(gw).ResolveAccountID(context.Background(), "ann@acme.com")
		if err != nil {
			t.Fatal(err)
		}
		if got != "ann-id" {
			t.Errorf("account id = %q, want ann-id", got)
		}
	})

	t.Run("an exact email wins over other fuzzy matches", func(t *testing.T) {
		gw := &fakeJira{findUsersFn: func(string) ([]jira.User, error) {
			return []jira.User{
				{AccountID: "other-id", Email: "ann.lee@acme.com", DisplayName: "Ann Lee"},
				{AccountID: "ann-id", Email: "ann@acme.com", DisplayName: "Ann"},
			}, nil
		}}
		got, err := newTestService(gw).ResolveAccountID(context.Background(), "ann@acme.com")
		if err != nil {
			t.Fatal(err)
		}
		if got != "ann-id" {
			t.Errorf("account id = %q, want the exact email match ann-id", got)
		}
	})

	t.Run("several inexact matches are an error, not a guess", func(t *testing.T) {
		gw := &fakeJira{findUsersFn: func(string) ([]jira.User, error) {
			return []jira.User{
				{AccountID: "a", DisplayName: "Ann Lee"},
				{AccountID: "b", DisplayName: "Ann Ray"},
			}, nil
		}}
		_, err := newTestService(gw).ResolveAccountID(context.Background(), "Ann")
		if err == nil {
			t.Fatal("want an ambiguity error")
		}
		if !strings.Contains(err.Error(), "matches 2 accounts") {
			t.Errorf("error = %v, want it to name the ambiguity", err)
		}
	})

	t.Run("no match", func(t *testing.T) {
		_, err := newTestService(&fakeJira{}).ResolveAccountID(context.Background(), "ghost@acme.com")
		if !errors.Is(err, jira.ErrUserNotFound) {
			t.Errorf("error = %v, want ErrUserNotFound", err)
		}
	})
}

func TestResolveField(t *testing.T) {
	service := newTestService(&fakeJira{})

	t.Run("by id", func(t *testing.T) {
		field, err := service.ResolveField(context.Background(), "customfield_10016")
		if err != nil {
			t.Fatal(err)
		}
		if field.Name != "Story Points" {
			t.Errorf("field = %+v", field)
		}
	})

	t.Run("by display name, case-insensitive", func(t *testing.T) {
		field, err := service.ResolveField(context.Background(), "story points")
		if err != nil {
			t.Fatal(err)
		}
		if field.ID != "customfield_10016" {
			t.Errorf("field id = %q, want customfield_10016", field.ID)
		}
	})

	t.Run("unknown name", func(t *testing.T) {
		_, err := service.ResolveField(context.Background(), "Nope")
		if !errors.Is(err, jira.ErrFieldNotFound) {
			t.Errorf("error = %v, want ErrFieldNotFound", err)
		}
	})

	t.Run("a duplicated name lists the ids to choose from", func(t *testing.T) {
		_, err := service.ResolveField(context.Background(), "Ambiguous")
		if err == nil {
			t.Fatal("want an ambiguity error")
		}
		for _, id := range []string{"duplicate", "duplicate_two"} {
			if !strings.Contains(err.Error(), id) {
				t.Errorf("error = %v, want it to list %s", err, id)
			}
		}
	})
}

// TestFieldCatalogFetchedOnce guards the cache: a command that touches several
// fields must not re-download the site's whole field list each time.
func TestFieldCatalogFetchedOnce(t *testing.T) {
	calls := 0
	gw := &fakeJira{fieldsFn: func() ([]jira.Field, error) {
		calls++
		return defaultTestFields, nil
	}}
	service := newTestService(gw)

	for _, name := range []string{"Story Points", "Labels", "Severity"} {
		if _, err := service.ResolveField(context.Background(), name); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 1 {
		t.Errorf("field catalog fetched %d times, want 1", calls)
	}
}

func TestBuildFields(t *testing.T) {
	tests := []struct {
		name       string
		assignment string
		wantID     string
		want       any
	}{
		{name: "string", assignment: "Summary=hello", wantID: "summary", want: "hello"},
		{name: "number", assignment: "Story Points=5", wantID: "customfield_10016", want: float64(5)},
		{
			name:       "string array splits on commas",
			assignment: "Labels=a, b ,c",
			wantID:     "labels",
			want:       []string{"a", "b", "c"},
		},
		{
			name:       "option wraps in value",
			assignment: "Severity=High",
			wantID:     "customfield_10030",
			want:       map[string]string{"value": "High"},
		},
		{
			name:       "option array",
			assignment: "Platforms=ios,android",
			wantID:     "customfield_10050",
			want:       []map[string]string{{"value": "ios"}, {"value": "android"}},
		},
		{
			name:       "raw JSON passes through",
			assignment: `Severity={"value":"Critical"}`,
			wantID:     "customfield_10030",
			want:       map[string]any{"value": "Critical"},
		},
		{name: "empty clears the field", assignment: "Summary=", wantID: "summary", want: nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assignment, err := ParseFieldAssignment(tc.assignment)
			if err != nil {
				t.Fatal(err)
			}
			fields, err := newTestService(&fakeJira{}).BuildFields(context.Background(), []FieldAssignment{assignment})
			if err != nil {
				t.Fatal(err)
			}
			got, ok := fields[tc.wantID]
			if !ok {
				t.Fatalf("fields = %v, want a %s entry", fields, tc.wantID)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("value = %#v, want %#v", got, tc.want)
			}
		})
	}

	t.Run("user fields resolve to an account id", func(t *testing.T) {
		assignment, _ := ParseFieldAssignment("Team Lead=@me")
		fields, err := newTestService(&fakeJira{}).BuildFields(context.Background(), []FieldAssignment{assignment})
		if err != nil {
			t.Fatal(err)
		}
		want := map[string]string{"accountId": "me-account-id"}
		if !reflect.DeepEqual(fields["customfield_10020"], want) {
			t.Errorf("value = %#v, want %#v", fields["customfield_10020"], want)
		}
	})

	t.Run("a non-numeric value for a number field is rejected", func(t *testing.T) {
		assignment, _ := ParseFieldAssignment("Story Points=lots")
		_, err := newTestService(&fakeJira{}).BuildFields(context.Background(), []FieldAssignment{assignment})
		if err == nil || !strings.Contains(err.Error(), "expects a number") {
			t.Errorf("error = %v, want a number-format error", err)
		}
	})
}

func TestParseFieldAssignment(t *testing.T) {
	t.Run("value may contain equals signs", func(t *testing.T) {
		got, err := ParseFieldAssignment("JQL=project = TEAM")
		if err != nil {
			t.Fatal(err)
		}
		if got.Name != "JQL" || got.Value != "project = TEAM" {
			t.Errorf("assignment = %+v", got)
		}
	})

	t.Run("missing equals is an error", func(t *testing.T) {
		if _, err := ParseFieldAssignment("Summary"); err == nil {
			t.Error("want an error for a value without =")
		}
	})
}

func TestCreateWorkItemRequiresCoreFields(t *testing.T) {
	service := newTestService(&fakeJira{})
	_, err := service.CreateWorkItem(context.Background(),
		WorkItemAttributes{Project: "TEAM", Summary: "no type"}, WriteOptions{})
	if err == nil || !strings.Contains(err.Error(), "--type") {
		t.Errorf("error = %v, want it to name the missing --type", err)
	}
}

func TestCreateWorkItemBuildsFields(t *testing.T) {
	gw := &fakeJira{}
	attrs := WorkItemAttributes{
		Project:     "TEAM",
		Type:        "Task",
		Summary:     "Fix login",
		Assignee:    "@me",
		Labels:      []string{"backend"},
		Priority:    "High",
		Due:         "2026-09-01",
		Parent:      "team-9",
		Description: DescriptionSource{Text: "Some **markdown**"},
	}
	result, err := newTestService(gw).CreateWorkItem(context.Background(), attrs, WriteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(gw.created) != 1 {
		t.Fatalf("created %d work items, want 1", len(gw.created))
	}

	fields := gw.created[0].Fields
	assertField(t, fields, "project", map[string]string{"key": "TEAM"})
	assertField(t, fields, "issuetype", map[string]string{"name": "Task"})
	assertField(t, fields, "summary", "Fix login")
	assertField(t, fields, "priority", map[string]string{"name": "High"})
	assertField(t, fields, "duedate", "2026-09-01")
	assertField(t, fields, "labels", []string{"backend"})
	assertField(t, fields, "assignee", map[string]any{"accountId": "me-account-id"})
	// Parent keys are upper-cased so a lower-case key still resolves.
	assertField(t, fields, "parent", map[string]string{"key": "TEAM-9"})

	description, ok := fields["description"].(jira.Document)
	if !ok {
		t.Fatalf("description = %#v, want a jira.Document", fields["description"])
	}
	if !strings.Contains(string(description), `"type":"doc"`) {
		t.Errorf("description is not ADF: %s", description)
	}
	if result.Keys[0] != "TEAM-100" {
		t.Errorf("keys = %v", result.Keys)
	}
}

// TestDryRunSkipsTheGateway is the contract behind --dry-run: nothing reaches
// the API, and the result says what would have happened.
func TestDryRunSkipsTheGateway(t *testing.T) {
	gw := &fakeJira{}
	service := newTestService(gw)
	opts := WriteOptions{DryRun: true}

	attrs := WorkItemAttributes{Project: "TEAM", Type: "Task", Summary: "Nope"}
	result, err := service.CreateWorkItem(context.Background(), attrs, opts)
	if err != nil {
		t.Fatal(err)
	}
	if !result.DryRun || len(gw.created) != 0 {
		t.Errorf("dry run created %d items, result = %+v", len(gw.created), result)
	}

	if _, err := service.DeleteWorkItems(context.Background(), Targets{Keys: []string{"TEAM-1"}}, false, opts); err != nil {
		t.Fatal(err)
	}
	if len(gw.deleted) != 0 {
		t.Errorf("dry run deleted %v", gw.deleted)
	}

	if _, err := service.AssignWorkItems(context.Background(), Targets{Keys: []string{"TEAM-1"}}, "@me", opts); err != nil {
		t.Fatal(err)
	}
	if len(gw.assigned) != 0 {
		t.Errorf("dry run assigned %v", gw.assigned)
	}
}

func TestDeleteWorkItemsNeedsConfirmation(t *testing.T) {
	t.Run("declining leaves everything alone", func(t *testing.T) {
		gw := &fakeJira{}
		opts := WriteOptions{Confirm: alwaysNo}
		result, err := newTestService(gw).DeleteWorkItems(context.Background(), Targets{Keys: []string{"TEAM-1"}}, false, opts)
		if err != nil {
			t.Fatal(err)
		}
		if !result.Aborted || len(gw.deleted) != 0 {
			t.Errorf("result = %+v, deleted = %v", result, gw.deleted)
		}
	})

	t.Run("--yes skips the prompt", func(t *testing.T) {
		gw := &fakeJira{}
		opts := WriteOptions{SkipConfirm: true, Confirm: alwaysNo}
		if _, err := newTestService(gw).DeleteWorkItems(context.Background(), Targets{Keys: []string{"TEAM-1"}}, false, opts); err != nil {
			t.Fatal(err)
		}
		if len(gw.deleted) != 1 {
			t.Errorf("deleted = %v, want TEAM-1", gw.deleted)
		}
	})
}

func TestSearchWorkItemsPaging(t *testing.T) {
	// pages simulates a three-page result set keyed by the incoming token.
	pages := map[string]jira.SearchPage{
		"":   {Items: []jira.WorkItem{{Key: "TEAM-1"}}, NextPageToken: "t2"},
		"t2": {Items: []jira.WorkItem{{Key: "TEAM-2"}}, NextPageToken: "t3"},
		"t3": {Items: []jira.WorkItem{{Key: "TEAM-3"}}},
	}
	newGateway := func(seen *[]string) *fakeJira {
		return &fakeJira{searchFn: func(in jira.SearchInput) (jira.SearchPage, error) {
			*seen = append(*seen, in.PageToken)
			return pages[in.PageToken], nil
		}}
	}

	t.Run("without --paginate only the first page is fetched", func(t *testing.T) {
		var seen []string
		items, err := newTestService(newGateway(&seen)).SearchWorkItems(
			context.Background(), SearchRequest{JQL: "project = TEAM"})
		if err != nil {
			t.Fatal(err)
		}
		if len(items) != 1 || len(seen) != 1 {
			t.Errorf("got %d items over %d requests, want 1 and 1", len(items), len(seen))
		}
	})

	t.Run("--paginate follows the tokens to the end", func(t *testing.T) {
		var seen []string
		items, err := newTestService(newGateway(&seen)).SearchWorkItems(
			context.Background(), SearchRequest{JQL: "project = TEAM", Paginate: true})
		if err != nil {
			t.Fatal(err)
		}
		if len(items) != 3 {
			t.Fatalf("got %d items, want 3", len(items))
		}
		if !reflect.DeepEqual(seen, []string{"", "t2", "t3"}) {
			t.Errorf("tokens seen = %v, want [\"\" t2 t3]", seen)
		}
	})

	t.Run("--limit truncates and stops early", func(t *testing.T) {
		var seen []string
		items, err := newTestService(newGateway(&seen)).SearchWorkItems(
			context.Background(), SearchRequest{JQL: "project = TEAM", Paginate: true, Limit: 2})
		if err != nil {
			t.Fatal(err)
		}
		if len(items) != 2 {
			t.Errorf("got %d items, want 2", len(items))
		}
		if len(seen) != 2 {
			t.Errorf("made %d requests, want 2", len(seen))
		}
	})

	t.Run("an empty query is rejected before any call", func(t *testing.T) {
		if _, err := newTestService(&fakeJira{}).SearchWorkItems(context.Background(), SearchRequest{}); err == nil {
			t.Error("want an error for an empty JQL query")
		}
	})
}

func TestResolveTargets(t *testing.T) {
	t.Run("JQL results are added to explicit keys", func(t *testing.T) {
		gw := &fakeJira{searchFn: func(jira.SearchInput) (jira.SearchPage, error) {
			return jira.SearchPage{Items: []jira.WorkItem{{Key: "TEAM-7"}, {Key: "TEAM-8"}}}, nil
		}}
		keys, err := newTestService(gw).Resolve(context.Background(),
			Targets{Keys: []string{"TEAM-1"}, JQL: "project = TEAM"})
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"TEAM-1", "TEAM-7", "TEAM-8"}
		if !reflect.DeepEqual(keys, want) {
			t.Errorf("keys = %v, want %v", keys, want)
		}
	})

	t.Run("selecting nothing is an error", func(t *testing.T) {
		_, err := newTestService(&fakeJira{}).Resolve(context.Background(), Targets{})
		if err == nil {
			t.Fatal("want an error when neither keys nor JQL are given")
		}
		if !strings.Contains(err.Error(), "pass --key or --jql") {
			t.Errorf("error = %v, want it to say what to pass", err)
		}
	})

	// A query that ran and matched nothing is a different problem from not
	// selecting anything; telling the user to "pass --jql" when they just did
	// sends them looking in the wrong place.
	t.Run("a JQL query that matches nothing says so", func(t *testing.T) {
		gw := &fakeJira{searchFn: func(jira.SearchInput) (jira.SearchPage, error) {
			return jira.SearchPage{}, nil
		}}
		_, err := newTestService(gw).Resolve(context.Background(), Targets{JQL: "project = EMPTY"})
		if err == nil {
			t.Fatal("want an error when the query matches nothing")
		}
		if !strings.Contains(err.Error(), "matched no work items") {
			t.Errorf("error = %v, want it to say the query matched nothing", err)
		}
		if !strings.Contains(err.Error(), "project = EMPTY") {
			t.Errorf("error = %v, want it to echo the query", err)
		}
	})
}

func TestTransitionWorkItems(t *testing.T) {
	transitions := []jira.Transition{
		{ID: "11", Name: "Start progress", ToName: "In Progress"},
		{ID: "31", Name: "Close", ToName: "Done"},
	}
	newGateway := func() *fakeJira {
		return &fakeJira{transitionsFn: func(string) ([]jira.Transition, error) { return transitions, nil }}
	}

	t.Run("matches the destination status", func(t *testing.T) {
		gw := newGateway()
		_, err := newTestService(gw).TransitionWorkItems(context.Background(),
			Targets{Keys: []string{"TEAM-1"}}, "done", WorkItemAttributes{}, WriteOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if len(gw.transitions) != 1 || gw.transitions[0][1] != "31" {
			t.Errorf("transitions = %v, want TEAM-1 via 31", gw.transitions)
		}
	})

	t.Run("falls back to the transition name", func(t *testing.T) {
		gw := newGateway()
		_, err := newTestService(gw).TransitionWorkItems(context.Background(),
			Targets{Keys: []string{"TEAM-1"}}, "Start progress", WorkItemAttributes{}, WriteOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if gw.transitions[0][1] != "11" {
			t.Errorf("transitions = %v, want the 11 transition", gw.transitions)
		}
	})

	t.Run("an unreachable status lists what is available", func(t *testing.T) {
		_, err := newTestService(newGateway()).TransitionWorkItems(context.Background(),
			Targets{Keys: []string{"TEAM-1"}}, "Released", WorkItemAttributes{}, WriteOptions{})
		if !errors.Is(err, jira.ErrTransitionNotFound) {
			t.Fatalf("error = %v, want ErrTransitionNotFound", err)
		}
		if !strings.Contains(err.Error(), "In Progress, Done") {
			t.Errorf("error = %v, want it to list the reachable statuses", err)
		}
	})
}

func TestLinkWorkItems(t *testing.T) {
	t.Run("a directional phrase resolves to the type name", func(t *testing.T) {
		gw := &fakeJira{}
		_, err := newTestService(gw).LinkWorkItems(context.Background(),
			jira.NewLinkInput{Type: "blocks", Inward: "TEAM-1", Outward: "TEAM-2"}, WriteOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if len(gw.links) != 1 || gw.links[0].Type != "Blocks" {
			t.Errorf("links = %+v, want the canonical type name Blocks", gw.links)
		}
	})

	t.Run("an unknown type lists the configured ones", func(t *testing.T) {
		_, err := newTestService(&fakeJira{}).LinkWorkItems(context.Background(),
			jira.NewLinkInput{Type: "Duplicates", Inward: "TEAM-1", Outward: "TEAM-2"}, WriteOptions{})
		if !errors.Is(err, jira.ErrLinkTypeNotFound) {
			t.Fatalf("error = %v, want ErrLinkTypeNotFound", err)
		}
		if !strings.Contains(err.Error(), "Blocks") {
			t.Errorf("error = %v, want it to list Blocks", err)
		}
	})
}

func TestCloneWorkItems(t *testing.T) {
	source := jira.WorkItem{
		Key: "TEAM-1", Summary: "Original", ProjectKey: "TEAM", Type: "Bug",
		Priority: "High", Labels: []string{"regression"},
		Assignee: jira.User{AccountID: "ann-id"},
		Raw:      json.RawMessage(`{"fields":{"description":{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"keep me"}]}]}}}`),
	}
	gw := &fakeJira{getFn: func(string, []string) (jira.WorkItem, error) { return source, nil }}

	result, err := newTestService(gw).CloneWorkItems(context.Background(),
		[]string{"TEAM-1"}, "CLONE -", WorkItemAttributes{}, WriteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(gw.created) != 1 {
		t.Fatalf("created %d items, want 1", len(gw.created))
	}

	fields := gw.created[0].Fields
	assertField(t, fields, "summary", "CLONE - Original")
	assertField(t, fields, "project", map[string]string{"key": "TEAM"})
	assertField(t, fields, "issuetype", map[string]string{"name": "Bug"})
	assertField(t, fields, "assignee", map[string]string{"accountId": "ann-id"})

	// The clone must carry the original ADF, not a flattened re-render.
	description, _ := fields["description"].(jira.Document)
	if !strings.Contains(string(description), "keep me") {
		t.Errorf("description = %s, want the source ADF", description)
	}
	if result.Keys[0] != "TEAM-100" {
		t.Errorf("keys = %v", result.Keys)
	}
}

func TestCloneOverridesWin(t *testing.T) {
	source := jira.WorkItem{Key: "TEAM-1", Summary: "Original", ProjectKey: "TEAM", Type: "Bug"}
	gw := &fakeJira{getFn: func(string, []string) (jira.WorkItem, error) { return source, nil }}

	_, err := newTestService(gw).CloneWorkItems(context.Background(), []string{"TEAM-1"}, "",
		WorkItemAttributes{Project: "OTHER", Assignee: "@me"}, WriteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	assertField(t, gw.created[0].Fields, "project", map[string]string{"key": "OTHER"})
	assertField(t, gw.created[0].Fields, "assignee", map[string]any{"accountId": "me-account-id"})
}

func TestEditWorkItemsRejectsAnEmptyChange(t *testing.T) {
	_, err := newTestService(&fakeJira{}).EditWorkItems(context.Background(),
		Targets{Keys: []string{"TEAM-1"}}, WorkItemAttributes{}, true, WriteOptions{})
	if err == nil || !strings.Contains(err.Error(), "nothing to change") {
		t.Errorf("error = %v, want a nothing-to-change error", err)
	}
}

// TestSetCommentVisibilityKeepsTheBody covers the reason the command reads the
// comment first: Jira's update replaces the whole comment, so the original text
// has to be sent back unchanged.
func TestSetCommentVisibilityKeepsTheBody(t *testing.T) {
	original := jira.Document(`{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"unchanged"}]}]}`)
	gw := &fakeJira{commentFn: func(_, id string) (jira.Comment, jira.Document, error) {
		return jira.Comment{ID: id}, original, nil
	}}

	vis := jira.CommentVisibility{Type: "role", Value: "Administrators"}
	if _, err := newTestService(gw).SetCommentVisibility(context.Background(), "TEAM-1", "20001", vis, WriteOptions{}); err != nil {
		t.Fatal(err)
	}
	if len(gw.comments) != 1 {
		t.Fatalf("updated %d comments, want 1", len(gw.comments))
	}
	if string(gw.comments[0]) != string(original) {
		t.Errorf("body = %s, want it unchanged", gw.comments[0])
	}
}

func TestRenderDescription(t *testing.T) {
	t.Run("plain text becomes ADF", func(t *testing.T) {
		rendered, err := RenderDescription(DescriptionSource{Text: "hello"})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(rendered.Body), `"text":"hello"`) {
			t.Errorf("body = %s", rendered.Body)
		}
	})

	t.Run("inline Markdown is converted", func(t *testing.T) {
		rendered, err := RenderDescription(DescriptionSource{Text: "a **bold** and `code`"})
		if err != nil {
			t.Fatal(err)
		}
		body := string(rendered.Body)
		for _, want := range []string{`"type":"strong"`, `"type":"code"`, `"text":"bold"`} {
			if !strings.Contains(body, want) {
				t.Errorf("body = %s, want it to contain %s", body, want)
			}
		}
		if strings.Contains(body, `**bold**`) {
			t.Errorf("body = %s, want the asterisks consumed", body)
		}
	})

	t.Run("a leading heading stays in the body", func(t *testing.T) {
		// Unlike --from-file, inline text has no summary to lift a heading into,
		// so consuming it would drop what the user wrote.
		rendered, err := RenderDescription(DescriptionSource{Text: "# Heading\n\nbody"})
		if err != nil {
			t.Fatal(err)
		}
		if rendered.Title != "" {
			t.Errorf("title = %q, want empty for an inline source", rendered.Title)
		}
		if !strings.Contains(string(rendered.Body), `"text":"Heading"`) {
			t.Errorf("body = %s, want the heading kept", rendered.Body)
		}
	})

	t.Run("an ADF string is passed through", func(t *testing.T) {
		source := `{"type":"doc","version":1,"content":[]}`
		rendered, err := RenderDescription(DescriptionSource{Text: source})
		if err != nil {
			t.Fatal(err)
		}
		if string(rendered.Body) != source {
			t.Errorf("body = %s, want it unchanged", rendered.Body)
		}
	})

	t.Run("an empty source produces no document", func(t *testing.T) {
		rendered, err := RenderDescription(DescriptionSource{})
		if err != nil {
			t.Fatal(err)
		}
		if !rendered.Body.Empty() {
			t.Errorf("body = %s, want empty", rendered.Body)
		}
	})

	t.Run("a missing file is reported", func(t *testing.T) {
		if _, err := RenderDescription(DescriptionSource{File: "does-not-exist.md"}); err == nil {
			t.Error("want an error for a missing file")
		}
	})
}

// assertField compares one entry of a fields payload.
func assertField(t *testing.T, fields jira.FieldValues, key string, want any) {
	t.Helper()
	got, ok := fields[key]
	if !ok {
		t.Errorf("fields has no %s (got %s)", key, fieldKeys(fields))
		return
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s = %#v, want %#v", key, got, want)
	}
}

func fieldKeys(fields jira.FieldValues) string {
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	return fmt.Sprint(keys)
}
