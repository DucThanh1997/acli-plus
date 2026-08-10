package app

import (
	"context"
	"fmt"

	jira "acli-plus/internal/domain/jira"
)

// fakeJira is a controllable jira.Gateway for use-case tests. Every method has
// a hook; the ones a given test does not set return harmless zero values, and
// the recorded slices let a test assert what reached the gateway.
type fakeJira struct {
	currentUserFn func() (jira.User, error)
	findUsersFn   func(query string) ([]jira.User, error)
	fieldsFn      func() ([]jira.Field, error)
	getFn         func(key string, fields []string) (jira.WorkItem, error)
	searchFn      func(in jira.SearchInput) (jira.SearchPage, error)
	transitionsFn func(key string) ([]jira.Transition, error)
	linkTypesFn   func() ([]jira.LinkType, error)
	commentFn     func(key, id string) (jira.Comment, jira.Document, error)

	created     []jira.NewWorkItemInput
	updated     []jira.EditWorkItemInput
	deleted     []string
	assigned    [][2]string
	transitions [][2]string
	archived    []string
	unarchived  []string
	comments    []jira.Document
	links       []jira.NewLinkInput
}

func (f *fakeJira) VerifyAuth(context.Context) error { return nil }

func (f *fakeJira) CurrentUser(context.Context) (jira.User, error) {
	if f.currentUserFn != nil {
		return f.currentUserFn()
	}
	return jira.User{AccountID: "me-account-id", DisplayName: "Me", Email: "me@acme.com"}, nil
}

func (f *fakeJira) FindUsers(_ context.Context, query string) ([]jira.User, error) {
	if f.findUsersFn != nil {
		return f.findUsersFn(query)
	}
	return nil, nil
}

func (f *fakeJira) GetWorkItem(_ context.Context, key string, fields []string) (jira.WorkItem, error) {
	if f.getFn != nil {
		return f.getFn(key, fields)
	}
	return jira.WorkItem{Key: key}, nil
}

func (f *fakeJira) CreateWorkItem(_ context.Context, in jira.NewWorkItemInput) (jira.WorkItem, error) {
	f.created = append(f.created, in)
	return jira.WorkItem{ID: "10001", Key: "TEAM-100"}, nil
}

func (f *fakeJira) CreateWorkItemsBulk(_ context.Context, inputs []jira.NewWorkItemInput) ([]jira.WorkItem, error) {
	items := make([]jira.WorkItem, 0, len(inputs))
	for i, in := range inputs {
		f.created = append(f.created, in)
		items = append(items, jira.WorkItem{Key: fmt.Sprintf("TEAM-%d", 200+i)})
	}
	return items, nil
}

func (f *fakeJira) UpdateWorkItem(_ context.Context, in jira.EditWorkItemInput) error {
	f.updated = append(f.updated, in)
	return nil
}

func (f *fakeJira) DeleteWorkItem(_ context.Context, key string, _ bool) error {
	f.deleted = append(f.deleted, key)
	return nil
}

func (f *fakeJira) Search(_ context.Context, in jira.SearchInput) (jira.SearchPage, error) {
	if f.searchFn != nil {
		return f.searchFn(in)
	}
	return jira.SearchPage{}, nil
}

func (f *fakeJira) ArchiveWorkItems(_ context.Context, keys []string) error {
	f.archived = append(f.archived, keys...)
	return nil
}

func (f *fakeJira) UnarchiveWorkItems(_ context.Context, keys []string) error {
	f.unarchived = append(f.unarchived, keys...)
	return nil
}

func (f *fakeJira) AssignWorkItem(_ context.Context, key, accountID string) error {
	f.assigned = append(f.assigned, [2]string{key, accountID})
	return nil
}

func (f *fakeJira) ListTransitions(_ context.Context, key string) ([]jira.Transition, error) {
	if f.transitionsFn != nil {
		return f.transitionsFn(key)
	}
	return nil, nil
}

func (f *fakeJira) TransitionWorkItem(_ context.Context, key, transitionID string, _ jira.FieldValues) error {
	f.transitions = append(f.transitions, [2]string{key, transitionID})
	return nil
}

func (f *fakeJira) RemoveWatcher(context.Context, string, string) error { return nil }

func (f *fakeJira) ListWatchers(context.Context, string) ([]jira.User, error) { return nil, nil }

func (f *fakeJira) ListComments(context.Context, string) ([]jira.Comment, error) { return nil, nil }

func (f *fakeJira) GetComment(_ context.Context, key, id string) (jira.Comment, jira.Document, error) {
	if f.commentFn != nil {
		return f.commentFn(key, id)
	}
	return jira.Comment{ID: id}, jira.Document(`{"type":"doc","version":1,"content":[]}`), nil
}

func (f *fakeJira) CreateComment(_ context.Context, _ string, body jira.Document, _ jira.CommentVisibility) (jira.Comment, error) {
	f.comments = append(f.comments, body)
	return jira.Comment{ID: "20001"}, nil
}

func (f *fakeJira) UpdateComment(_ context.Context, _, id string, body jira.Document, _ jira.CommentVisibility) (jira.Comment, error) {
	f.comments = append(f.comments, body)
	return jira.Comment{ID: id}, nil
}

func (f *fakeJira) DeleteComment(context.Context, string, string) error { return nil }

func (f *fakeJira) ListAttachments(context.Context, string) ([]jira.Attachment, error) {
	return nil, nil
}

func (f *fakeJira) DeleteAttachment(context.Context, string) error { return nil }

func (f *fakeJira) ListLinkTypes(context.Context) ([]jira.LinkType, error) {
	if f.linkTypesFn != nil {
		return f.linkTypesFn()
	}
	return []jira.LinkType{{Name: "Blocks", Inward: "is blocked by", Outward: "blocks"}}, nil
}

func (f *fakeJira) CreateLink(_ context.Context, in jira.NewLinkInput) error {
	f.links = append(f.links, in)
	return nil
}

func (f *fakeJira) ListProjects(context.Context, jira.ProjectQuery) ([]jira.Project, error) {
	return nil, nil
}

func (f *fakeJira) GetProject(_ context.Context, keyOrID string) (jira.Project, error) {
	return jira.Project{Key: keyOrID}, nil
}

func (f *fakeJira) CreateProject(_ context.Context, in jira.NewProjectInput) (jira.Project, error) {
	return jira.Project{Key: in.Key, Name: in.Name}, nil
}

func (f *fakeJira) UpdateProject(_ context.Context, in jira.UpdateProjectInput) (jira.Project, error) {
	return jira.Project{Key: in.KeyOrID}, nil
}

func (f *fakeJira) DeleteProject(context.Context, string, bool) error { return nil }

func (f *fakeJira) ArchiveProject(context.Context, string) error { return nil }

func (f *fakeJira) RestoreProject(context.Context, string) error { return nil }

func (f *fakeJira) ListFields(context.Context) ([]jira.Field, error) {
	if f.fieldsFn != nil {
		return f.fieldsFn()
	}
	return defaultTestFields, nil
}

func (f *fakeJira) CreateField(_ context.Context, in jira.NewFieldInput) (jira.Field, error) {
	return jira.Field{ID: "customfield_99999", Name: in.Name}, nil
}

func (f *fakeJira) DeleteField(context.Context, string) error { return nil }

func (f *fakeJira) RestoreField(context.Context, string) error { return nil }

func (f *fakeJira) SearchBoards(context.Context, jira.BoardQuery) ([]jira.Board, error) {
	return nil, nil
}

func (f *fakeJira) ListSprints(context.Context, int, string) ([]jira.Sprint, error) { return nil, nil }

func (f *fakeJira) ListSprintWorkItems(context.Context, int, string, []string) ([]jira.WorkItem, error) {
	return nil, nil
}

func (f *fakeJira) SearchFilters(context.Context, jira.FilterQuery) ([]jira.Filter, error) {
	return nil, nil
}

func (f *fakeJira) ListMyFilters(context.Context, bool) ([]jira.Filter, error) { return nil, nil }

func (f *fakeJira) AddFilterFavourite(context.Context, string) error { return nil }

func (f *fakeJira) ChangeFilterOwner(context.Context, string, string) error { return nil }

func (f *fakeJira) SearchDashboards(context.Context, jira.DashboardQuery) ([]jira.Dashboard, error) {
	return nil, nil
}

// defaultTestFields is a small stand-in for a real site's field catalog,
// covering one field of each shape the value coercion has to handle.
var defaultTestFields = []jira.Field{
	{ID: "summary", Name: "Summary", SchemaType: "string"},
	{ID: "labels", Name: "Labels", SchemaType: "array", ItemType: "string"},
	{ID: "customfield_10016", Name: "Story Points", Custom: true, SchemaType: "number"},
	{ID: "customfield_10020", Name: "Team Lead", Custom: true, SchemaType: "user"},
	{ID: "customfield_10030", Name: "Severity", Custom: true, SchemaType: "option"},
	{ID: "customfield_10040", Name: "Reviewers", Custom: true, SchemaType: "array", ItemType: "user"},
	{ID: "customfield_10050", Name: "Platforms", Custom: true, SchemaType: "array", ItemType: "option"},
	{ID: "duplicate", Name: "Ambiguous", Custom: true, SchemaType: "string"},
	{ID: "duplicate_two", Name: "Ambiguous", Custom: true, SchemaType: "string"},
}

// newTestService wires a service around a fake gateway.
func newTestService(gw *fakeJira) *JiraService {
	return NewJiraService(gw, "acme.atlassian.net")
}
