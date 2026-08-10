package jirarest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	jira "acli-plus/internal/domain/jira"
)

// newTestClient starts a stub Jira and returns a client pointed at it.
func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	client := New("acme.atlassian.net", "me@acme.com", "token")
	client.baseURL = server.URL
	return client
}

// recorded captures what the client sent, for tests that assert on the request.
type recorded struct {
	method string
	path   string
	query  string
	auth   string
	body   map[string]any
}

func record(t *testing.T, req *http.Request, into *recorded) {
	t.Helper()
	into.method = req.Method
	into.path = req.URL.Path
	into.query = req.URL.RawQuery
	into.auth = req.Header.Get("Authorization")

	payload, err := io.ReadAll(req.Body)
	if err != nil || len(payload) == 0 {
		return
	}
	if err := json.Unmarshal(payload, &into.body); err != nil {
		t.Errorf("request body is not JSON: %s", payload)
	}
}

func TestSearchUsesTheJQLEndpoint(t *testing.T) {
	var got recorded
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		record(t, r, &got)
		fmt.Fprint(w, `{"issues":[{"id":"1","key":"TEAM-1","fields":{"summary":"One"}}],"nextPageToken":"tok2"}`)
	})

	page, err := client.Search(context.Background(), jira.SearchInput{
		JQL: "project = TEAM", Fields: []string{"summary"}, MaxResults: 25, PageToken: "tok1",
	})
	if err != nil {
		t.Fatal(err)
	}

	// The old GET /rest/api/3/search was removed and answers 410, so the POST
	// path here is the only supported search.
	if got.method != http.MethodPost || got.path != "/rest/api/3/search/jql" {
		t.Errorf("request = %s %s, want POST /rest/api/3/search/jql", got.method, got.path)
	}
	if !strings.HasPrefix(got.auth, "Basic ") {
		t.Errorf("authorization = %q, want Basic auth", got.auth)
	}
	if got.body["jql"] != "project = TEAM" {
		t.Errorf("body jql = %v", got.body["jql"])
	}
	if got.body["nextPageToken"] != "tok1" {
		t.Errorf("body nextPageToken = %v, want tok1", got.body["nextPageToken"])
	}
	if page.NextPageToken != "tok2" {
		t.Errorf("next page token = %q, want tok2", page.NextPageToken)
	}
	if len(page.Items) != 1 || page.Items[0].Key != "TEAM-1" {
		t.Errorf("items = %+v", page.Items)
	}
}

// TestSearchSendsDefaultFields covers the trap in the replacement endpoint: it
// returns only ids unless fields are named, so an empty request must not go out
// with an empty field list.
func TestSearchSendsDefaultFields(t *testing.T) {
	var got recorded
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		record(t, r, &got)
		fmt.Fprint(w, `{"issues":[]}`)
	})

	if _, err := client.Search(context.Background(), jira.SearchInput{JQL: "project = TEAM"}); err != nil {
		t.Fatal(err)
	}
	fields, ok := got.body["fields"].([]any)
	if !ok || len(fields) == 0 {
		t.Fatalf("body fields = %v, want the default set", got.body["fields"])
	}
	if !strings.Contains(fmt.Sprint(fields), "summary") {
		t.Errorf("default fields = %v, want summary among them", fields)
	}
}

func TestGetWorkItemDecodesFields(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/issue/TEAM-1" {
			t.Errorf("path = %s", r.URL.Path)
		}
		fmt.Fprint(w, `{
			"id": "10001", "key": "TEAM-1",
			"fields": {
				"summary": "Fix login",
				"description": {"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"details"}]}]},
				"status": {"name": "In Progress", "statusCategory": {"name": "In Progress"}},
				"issuetype": {"name": "Bug"},
				"project": {"key": "TEAM"},
				"priority": {"name": "High"},
				"assignee": {"accountId": "ann-id", "displayName": "Ann", "emailAddress": "ann@acme.com"},
				"labels": ["backend", "urgent"],
				"components": [{"name": "api"}],
				"parent": {"key": "TEAM-9"},
				"created": "2026-01-02T15:04:05.000+0700",
				"duedate": "2026-03-01"
			}
		}`)
	})

	item, err := client.GetWorkItem(context.Background(), "TEAM-1", nil)
	if err != nil {
		t.Fatal(err)
	}

	for _, check := range []struct{ name, got, want string }{
		{"key", item.Key, "TEAM-1"},
		{"summary", item.Summary, "Fix login"},
		{"status", item.Status, "In Progress"},
		{"type", item.Type, "Bug"},
		{"project", item.ProjectKey, "TEAM"},
		{"priority", item.Priority, "High"},
		{"assignee", item.Assignee.Name(), "Ann"},
		{"parent", item.ParentKey, "TEAM-9"},
		{"due date", item.DueDate, "2026-03-01"},
		{"description", item.Description, "details"},
	} {
		if check.got != check.want {
			t.Errorf("%s = %q, want %q", check.name, check.got, check.want)
		}
	}
	if item.Created.Year() != 2026 || item.Created.Month() != 1 {
		t.Errorf("created = %v, want a parsed 2026-01 timestamp", item.Created)
	}
	if len(item.Labels) != 2 || len(item.Components) != 1 {
		t.Errorf("labels = %v, components = %v", item.Labels, item.Components)
	}
	if len(item.Raw) == 0 {
		t.Error("raw payload should be kept for --json")
	}
}

func TestGetWorkItemFieldsQuery(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("fields"); got != "summary,status" {
			t.Errorf("fields query = %q, want summary,status", got)
		}
		fmt.Fprint(w, `{"key":"TEAM-1","fields":{}}`)
	})
	if _, err := client.GetWorkItem(context.Background(), "TEAM-1", []string{"summary", "status"}); err != nil {
		t.Fatal(err)
	}
}

func TestErrorMapping(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		body     string
		wantErr  error
		contains string
	}{
		{
			name:    "401 is an auth error",
			status:  http.StatusUnauthorized,
			body:    `{"errorMessages":["Client must be authenticated"]}`,
			wantErr: jira.ErrAuth,
		},
		{
			name:    "403 is an auth error",
			status:  http.StatusForbidden,
			body:    `{"errorMessages":["Forbidden"]}`,
			wantErr: jira.ErrAuth,
		},
		{
			name:    "404 names the work item",
			status:  http.StatusNotFound,
			body:    `{"errorMessages":["Issue does not exist"]}`,
			wantErr: jira.ErrWorkItemNotFound,
		},
		{
			name:     "field errors are surfaced",
			status:   http.StatusBadRequest,
			body:     `{"errorMessages":[],"errors":{"summary":"Summary is required"}}`,
			contains: "summary: Summary is required",
		},
		{
			name:     "general messages are surfaced",
			status:   http.StatusBadRequest,
			body:     `{"errorMessages":["Field 'nope' cannot be set"]}`,
			contains: "Field 'nope' cannot be set",
		},
		{
			name:     "a non-JSON body still reaches the user",
			status:   http.StatusBadGateway,
			body:     `<html>gateway timeout</html>`,
			contains: "gateway timeout",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			})

			_, err := client.GetWorkItem(context.Background(), "TEAM-1", nil)
			if err == nil {
				t.Fatal("want an error")
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Errorf("error = %v, want it to wrap %v", err, tc.wantErr)
			}
			if tc.contains != "" && !strings.Contains(err.Error(), tc.contains) {
				t.Errorf("error = %v, want it to contain %q", err, tc.contains)
			}
		})
	}
}

// TestArchiveWithoutPremium covers the plan-gated endpoint: a site without
// Premium answers 404 for the path itself, which would otherwise read as
// "work item not found".
func TestArchiveWithoutPremium(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"errorMessages":["The requested resource is not available"]}`)
	})

	err := client.ArchiveWorkItems(context.Background(), []string{"TEAM-1"})
	if !errors.Is(err, jira.ErrNotLicensed) {
		t.Fatalf("error = %v, want ErrNotLicensed", err)
	}
	if !strings.Contains(err.Error(), "Premium") {
		t.Errorf("error = %v, want it to mention the plan requirement", err)
	}
}

func TestArchiveReportsPerItemFailures(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"numberOfIssuesUpdated":0,"errors":{"issuesNotFound":["TEAM-99"]}}`)
	})

	err := client.ArchiveWorkItems(context.Background(), []string{"TEAM-99"})
	if !errors.Is(err, jira.ErrWorkItemNotFound) {
		t.Fatalf("error = %v, want ErrWorkItemNotFound", err)
	}
}

func TestAssignWorkItemClearsWithNull(t *testing.T) {
	var got recorded
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		record(t, r, &got)
		w.WriteHeader(http.StatusNoContent)
	})

	if err := client.AssignWorkItem(context.Background(), "TEAM-1", ""); err != nil {
		t.Fatal(err)
	}
	if value, ok := got.body["accountId"]; !ok || value != nil {
		t.Errorf("body = %v, want accountId null to unassign", got.body)
	}
}

// TestUpdateCommentAlwaysSendsVisibility is what makes "comment-visibility
// --public" able to lift a restriction: the key has to be present and null.
func TestUpdateCommentAlwaysSendsVisibility(t *testing.T) {
	var got recorded
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		record(t, r, &got)
		fmt.Fprint(w, `{"id":"20001"}`)
	})

	body := jira.Document(`{"type":"doc","version":1,"content":[]}`)
	if _, err := client.UpdateComment(context.Background(), "TEAM-1", "20001", body, jira.CommentVisibility{}); err != nil {
		t.Fatal(err)
	}
	value, present := got.body["visibility"]
	if !present || value != nil {
		t.Errorf("body = %v, want an explicit null visibility", got.body)
	}
}

func TestBulkCreateReportsPartialFailure(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{
			"issues": [{"id":"1","key":"TEAM-1"}],
			"errors": [{"failedElementNumber":1,"elementErrors":{"errorMessages":["boom"],"errors":{"summary":"required"}}}]
		}`)
	})

	items, err := client.CreateWorkItemsBulk(context.Background(), []jira.NewWorkItemInput{
		{Fields: jira.FieldValues{"summary": "ok"}},
		{Fields: jira.FieldValues{}},
	})
	if err == nil {
		t.Fatal("want an error when part of the batch fails")
	}
	// The ones that did get created must still be reported.
	if len(items) != 1 || items[0].Key != "TEAM-1" {
		t.Errorf("items = %+v, want the created work item to survive the error", items)
	}
	for _, want := range []string{"item 2", "boom", "summary: required"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %v, want it to contain %q", err, want)
		}
	}
}

func TestPagedValuesWalksPages(t *testing.T) {
	var startAts []string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		startAts = append(startAts, r.URL.Query().Get("startAt"))
		switch r.URL.Query().Get("startAt") {
		case "0":
			fmt.Fprint(w, `{"values":[{"key":"A"},{"key":"B"}],"isLast":false,"total":3}`)
		default:
			fmt.Fprint(w, `{"values":[{"key":"C"}],"isLast":true,"total":3}`)
		}
	})

	projects, err := client.ListProjects(context.Background(), jira.ProjectQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 3 {
		t.Errorf("got %d projects over %v, want 3", len(projects), startAts)
	}
	if len(startAts) != 2 || startAts[0] != "0" || startAts[1] != "2" {
		t.Errorf("startAt values = %v, want [0 2]", startAts)
	}
}

func TestPagedValuesHonorsLimit(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"values":[{"key":"A"},{"key":"B"},{"key":"C"}],"isLast":false,"total":99}`)
	})

	projects, err := client.ListProjects(context.Background(), jira.ProjectQuery{MaxResults: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 2 {
		t.Errorf("got %d projects, want the limit of 2", len(projects))
	}
}

// TestBoardsUseTheAgileAPI guards the one place where Jira splits its API:
// boards and sprints do not live under /rest/api/3.
func TestBoardsUseTheAgileAPI(t *testing.T) {
	var path string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		fmt.Fprint(w, `{"values":[{"id":42,"name":"TEAM board","type":"scrum","location":{"projectKey":"TEAM"}}],"isLast":true}`)
	})

	boards, err := client.SearchBoards(context.Background(), jira.BoardQuery{ProjectKey: "TEAM"})
	if err != nil {
		t.Fatal(err)
	}
	if path != "/rest/agile/1.0/board" {
		t.Errorf("path = %s, want /rest/agile/1.0/board", path)
	}
	if len(boards) != 1 || boards[0].ID != 42 || boards[0].ProjectKey != "TEAM" {
		t.Errorf("boards = %+v", boards)
	}
}

func TestListAttachmentsReadsTheField(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("fields"); got != "attachment" {
			t.Errorf("fields query = %q, want attachment", got)
		}
		fmt.Fprint(w, `{"fields":{"attachment":[{"id":10001,"filename":"log.txt","size":2048,"mimeType":"text/plain"}]}}`)
	})

	attachments, err := client.ListAttachments(context.Background(), "TEAM-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(attachments) != 1 {
		t.Fatalf("got %d attachments, want 1", len(attachments))
	}
	// The id arrives as a number here and as a string elsewhere in the API.
	if attachments[0].ID != "10001" || attachments[0].Filename != "log.txt" {
		t.Errorf("attachment = %+v", attachments[0])
	}
}

func TestJiraTimeLayouts(t *testing.T) {
	tests := []struct {
		name  string
		value string
		year  int
	}{
		{name: "platform millisecond stamp", value: `"2026-01-02T15:04:05.000+0700"`, year: 2026},
		{name: "rfc3339 from the agile api", value: `"2025-06-01T10:00:00Z"`, year: 2025},
		{name: "date only", value: `"2024-12-31"`, year: 2024},
		{name: "null is no timestamp", value: `null`, year: 1},
		{name: "empty is no timestamp", value: `""`, year: 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var parsed jiraTime
			if err := json.Unmarshal([]byte(tc.value), &parsed); err != nil {
				t.Fatal(err)
			}
			if parsed.Year() != tc.year {
				t.Errorf("year = %d, want %d", parsed.Year(), tc.year)
			}
		})
	}
}

func TestDeleteWorkItemPassesSubtaskFlag(t *testing.T) {
	var got recorded
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		record(t, r, &got)
		w.WriteHeader(http.StatusNoContent)
	})

	if err := client.DeleteWorkItem(context.Background(), "TEAM-1", true); err != nil {
		t.Fatal(err)
	}
	if got.method != http.MethodDelete || !strings.Contains(got.query, "deleteSubtasks=true") {
		t.Errorf("request = %s %s?%s", got.method, got.path, got.query)
	}
}
