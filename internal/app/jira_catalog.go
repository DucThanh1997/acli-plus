package app

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	jira "acli-plus/internal/domain/jira"
)

// defaultProjectTemplates are the templates Jira creates a project from when
// the caller names a project type but no template. Every type needs one, so
// picking a sensible default is what makes --type alone enough.
var defaultProjectTemplates = map[string]string{
	"software":     "com.pyxis.greenhopper.jira:gh-simplified-agility-kanban",
	"business":     "com.atlassian.jira-core-project-templates:jira-core-simplified-project-management",
	"service_desk": "com.atlassian.servicedesk:simplified-it-service-management",
}

// ListProjects returns the projects visible to the caller.
func (s *JiraService) ListProjects(ctx context.Context, q jira.ProjectQuery) ([]jira.Project, error) {
	return s.gw.ListProjects(ctx, q)
}

// ViewProject fetches one project.
func (s *JiraService) ViewProject(ctx context.Context, keyOrID string) (jira.Project, error) {
	return s.gw.GetProject(ctx, keyOrID)
}

// CreateProject creates a project, defaulting the lead to the current user and
// the template to one that matches the project type.
func (s *JiraService) CreateProject(ctx context.Context, in jira.NewProjectInput, lead string, opts WriteOptions) (JiraResult, error) {
	if in.Key == "" || in.Name == "" {
		return JiraResult{}, fmt.Errorf("--key and --name are required to create a project")
	}
	if in.TypeKey == "" {
		in.TypeKey = "software"
	}
	if in.TemplateKey == "" {
		in.TemplateKey = defaultProjectTemplates[in.TypeKey]
	}

	accountID, err := s.ResolveAccountID(ctx, lead)
	if err != nil {
		return JiraResult{}, err
	}
	if accountID == "" {
		user, err := s.CurrentUser(ctx)
		if err != nil {
			return JiraResult{}, err
		}
		accountID = user.AccountID
	}
	in.LeadAccountID = accountID

	if opts.DryRun {
		return JiraResult{
			Detail: fmt.Sprintf("create %s project %s (%s)", in.TypeKey, in.Key, in.Name),
			Keys:   []string{in.Key},
			DryRun: true,
		}, nil
	}

	project, err := s.gw.CreateProject(ctx, in)
	if err != nil {
		return JiraResult{}, err
	}
	return JiraResult{Detail: fmt.Sprintf("created project %s (%s)", project.Key, project.Name), Keys: []string{project.Key}}, nil
}

// UpdateProject changes a project's mutable attributes.
func (s *JiraService) UpdateProject(ctx context.Context, in jira.UpdateProjectInput, lead string, opts WriteOptions) (JiraResult, error) {
	if lead != "" {
		accountID, err := s.ResolveAccountID(ctx, lead)
		if err != nil {
			return JiraResult{}, err
		}
		in.LeadAccountID = accountID
	}
	if opts.DryRun {
		return JiraResult{Detail: "update project " + in.KeyOrID, Keys: []string{in.KeyOrID}, DryRun: true}, nil
	}

	project, err := s.gw.UpdateProject(ctx, in)
	if err != nil {
		return JiraResult{}, err
	}
	key := project.Key
	if key == "" {
		key = in.KeyOrID
	}
	return JiraResult{Detail: "updated project " + key, Keys: []string{key}}, nil
}

// DeleteProject deletes a project. With undo enabled it goes to the trash and
// project restore brings it back; without it the deletion is immediate.
func (s *JiraService) DeleteProject(ctx context.Context, keyOrID string, enableUndo bool, opts WriteOptions) (JiraResult, error) {
	if opts.DryRun {
		return JiraResult{Detail: "delete project " + keyOrID, Keys: []string{keyOrID}, DryRun: true}, nil
	}

	warning := "This cannot be undone."
	if enableUndo {
		warning = "It moves to the trash and can be restored."
	}
	ok, err := confirmOrAbort(opts, fmt.Sprintf("Delete project %s and everything in it? %s", keyOrID, warning))
	if err != nil {
		return JiraResult{}, err
	}
	if !ok {
		return aborted(), nil
	}

	if err := s.gw.DeleteProject(ctx, keyOrID, enableUndo); err != nil {
		return JiraResult{}, err
	}
	return JiraResult{Detail: "deleted project " + keyOrID, Keys: []string{keyOrID}}, nil
}

// SetProjectArchived archives or restores a project.
func (s *JiraService) SetProjectArchived(ctx context.Context, keyOrID string, archived bool, opts WriteOptions) (JiraResult, error) {
	verb, done := "restore", "restored"
	if archived {
		verb, done = "archive", "archived"
	}
	if opts.DryRun {
		return JiraResult{Detail: verb + " project " + keyOrID, Keys: []string{keyOrID}, DryRun: true}, nil
	}

	call := s.gw.RestoreProject
	if archived {
		call = s.gw.ArchiveProject
	}
	if err := call(ctx, keyOrID); err != nil {
		return JiraResult{}, err
	}
	return JiraResult{Detail: done + " project " + keyOrID, Keys: []string{keyOrID}}, nil
}

// ListFields returns every field on the site.
func (s *JiraService) ListFields(ctx context.Context) ([]jira.Field, error) {
	return s.FieldCatalog(ctx)
}

// CreateField creates a custom field.
func (s *JiraService) CreateField(ctx context.Context, in jira.NewFieldInput, opts WriteOptions) (JiraResult, error) {
	if in.Name == "" || in.Type == "" {
		return JiraResult{}, fmt.Errorf("--name and --type are required to create a field")
	}
	if opts.DryRun {
		return JiraResult{Detail: fmt.Sprintf("create custom field %q (%s)", in.Name, in.Type), DryRun: true}, nil
	}

	field, err := s.gw.CreateField(ctx, in)
	if err != nil {
		return JiraResult{}, err
	}
	return JiraResult{Detail: fmt.Sprintf("created custom field %q (%s)", field.Name, field.ID), Keys: []string{field.ID}}, nil
}

// SetFieldDeleted trashes or restores a custom field, accepting either a field
// id or its display name.
func (s *JiraService) SetFieldDeleted(ctx context.Context, nameOrID string, deleted bool, opts WriteOptions) (JiraResult, error) {
	field, err := s.ResolveField(ctx, nameOrID)
	if err != nil {
		return JiraResult{}, err
	}
	if deleted && !field.Custom {
		return JiraResult{}, fmt.Errorf("%s is a system field and cannot be deleted", field.Name)
	}

	verb, done := "restore", "restored"
	if deleted {
		verb, done = "trash", "moved to trash"
	}
	if opts.DryRun {
		return JiraResult{Detail: fmt.Sprintf("%s field %q (%s)", verb, field.Name, field.ID), Keys: []string{field.ID}, DryRun: true}, nil
	}

	if deleted {
		ok, err := confirmOrAbort(opts, fmt.Sprintf("Move custom field %q (%s) to the trash?", field.Name, field.ID))
		if err != nil {
			return JiraResult{}, err
		}
		if !ok {
			return aborted(), nil
		}
		if err := s.gw.DeleteField(ctx, field.ID); err != nil {
			return JiraResult{}, err
		}
	} else if err := s.gw.RestoreField(ctx, field.ID); err != nil {
		return JiraResult{}, err
	}
	return JiraResult{Detail: fmt.Sprintf("%s field %q (%s)", done, field.Name, field.ID), Keys: []string{field.ID}}, nil
}

// SearchBoards returns the Agile boards matching a query.
func (s *JiraService) SearchBoards(ctx context.Context, q jira.BoardQuery) ([]jira.Board, error) {
	return s.gw.SearchBoards(ctx, q)
}

// ResolveBoardID accepts a board id or a board name and returns the id.
func (s *JiraService) ResolveBoardID(ctx context.Context, value string) (int, error) {
	if id, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
		return id, nil
	}
	boards, err := s.gw.SearchBoards(ctx, jira.BoardQuery{Name: value})
	if err != nil {
		return 0, err
	}
	var matches []jira.Board
	for _, board := range boards {
		if strings.EqualFold(board.Name, value) {
			matches = append(matches, board)
		}
	}
	if len(matches) == 0 {
		matches = boards
	}
	switch len(matches) {
	case 1:
		return matches[0].ID, nil
	case 0:
		return 0, fmt.Errorf("%w: %q", jira.ErrBoardNotFound, value)
	default:
		return 0, fmt.Errorf("%q matches %d boards; pass a board id instead: %s",
			value, len(matches), describeBoards(matches))
	}
}

func describeBoards(boards []jira.Board) string {
	names := make([]string, 0, len(boards))
	for _, board := range boards {
		names = append(names, fmt.Sprintf("%d (%s)", board.ID, board.Name))
	}
	return strings.Join(names, ", ")
}

// ListSprints returns the sprints on a board.
func (s *JiraService) ListSprints(ctx context.Context, board, state string) ([]jira.Sprint, error) {
	boardID, err := s.ResolveBoardID(ctx, board)
	if err != nil {
		return nil, err
	}
	return s.gw.ListSprints(ctx, boardID, state)
}

// ListSprintWorkItems returns the work items in a sprint. The sprint may be
// given by id, or by name together with the board it belongs to.
func (s *JiraService) ListSprintWorkItems(ctx context.Context, sprint, board, jql string, fields []string) ([]jira.WorkItem, error) {
	sprintID, err := s.resolveSprintID(ctx, sprint, board)
	if err != nil {
		return nil, err
	}
	return s.gw.ListSprintWorkItems(ctx, sprintID, jql, fields)
}

func (s *JiraService) resolveSprintID(ctx context.Context, sprint, board string) (int, error) {
	if id, err := strconv.Atoi(strings.TrimSpace(sprint)); err == nil {
		return id, nil
	}
	if board == "" {
		return 0, fmt.Errorf("sprint %q is not an id; pass --board so the name can be looked up", sprint)
	}

	sprints, err := s.ListSprints(ctx, board, "")
	if err != nil {
		return 0, err
	}
	for _, candidate := range sprints {
		if strings.EqualFold(candidate.Name, sprint) {
			return candidate.ID, nil
		}
	}
	return 0, fmt.Errorf("%w: %q on board %s", jira.ErrSprintNotFound, sprint, board)
}

// SearchFilters searches saved filters across the site.
func (s *JiraService) SearchFilters(ctx context.Context, q jira.FilterQuery) ([]jira.Filter, error) {
	return s.gw.SearchFilters(ctx, q)
}

// ListMyFilters returns the caller's own and favourite filters.
func (s *JiraService) ListMyFilters(ctx context.Context, favouritesOnly bool) ([]jira.Filter, error) {
	return s.gw.ListMyFilters(ctx, favouritesOnly)
}

// ResolveFilterID accepts a filter id or an exact filter name.
func (s *JiraService) ResolveFilterID(ctx context.Context, value string) (jira.Filter, error) {
	if _, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
		return jira.Filter{ID: strings.TrimSpace(value), Name: value}, nil
	}
	filters, err := s.gw.SearchFilters(ctx, jira.FilterQuery{Name: value})
	if err != nil {
		return jira.Filter{}, err
	}
	var matches []jira.Filter
	for _, filter := range filters {
		if strings.EqualFold(filter.Name, value) {
			matches = append(matches, filter)
		}
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return jira.Filter{}, fmt.Errorf("no filter named %q", value)
	default:
		return jira.Filter{}, fmt.Errorf("%q matches %d filters; pass a filter id instead", value, len(matches))
	}
}

// AddFilterFavourite marks filters as favourites for the current user.
func (s *JiraService) AddFilterFavourite(ctx context.Context, values []string, opts WriteOptions) (JiraResult, error) {
	ids, names, err := s.resolveFilters(ctx, values)
	if err != nil {
		return JiraResult{}, err
	}
	if opts.DryRun {
		return JiraResult{Detail: "favourite filter(s) " + strings.Join(names, ", "), Keys: ids, DryRun: true}, nil
	}
	for _, id := range ids {
		if err := s.gw.AddFilterFavourite(ctx, id); err != nil {
			return JiraResult{}, fmt.Errorf("filter %s: %w", id, err)
		}
	}
	return JiraResult{Detail: "favourited filter(s) " + strings.Join(names, ", "), Keys: ids}, nil
}

// ChangeFilterOwner transfers filters to another account.
func (s *JiraService) ChangeFilterOwner(ctx context.Context, values []string, owner string, opts WriteOptions) (JiraResult, error) {
	accountID, err := s.ResolveAccountID(ctx, owner)
	if err != nil {
		return JiraResult{}, err
	}
	if accountID == "" {
		return JiraResult{}, fmt.Errorf("a new owner is required (--owner)")
	}
	ids, names, err := s.resolveFilters(ctx, values)
	if err != nil {
		return JiraResult{}, err
	}

	if opts.DryRun {
		return JiraResult{Detail: fmt.Sprintf("give filter(s) %s to %s", strings.Join(names, ", "), owner), Keys: ids, DryRun: true}, nil
	}
	for _, id := range ids {
		if err := s.gw.ChangeFilterOwner(ctx, id, accountID); err != nil {
			return JiraResult{}, fmt.Errorf("filter %s: %w", id, err)
		}
	}
	return JiraResult{Detail: fmt.Sprintf("gave filter(s) %s to %s", strings.Join(names, ", "), owner), Keys: ids}, nil
}

func (s *JiraService) resolveFilters(ctx context.Context, values []string) (ids, names []string, err error) {
	for _, value := range values {
		filter, err := s.ResolveFilterID(ctx, value)
		if err != nil {
			return nil, nil, err
		}
		ids = append(ids, filter.ID)
		names = append(names, filter.Name)
	}
	if len(ids) == 0 {
		return nil, nil, fmt.Errorf("no filters given")
	}
	return ids, names, nil
}

// SearchDashboards searches dashboards across the site.
func (s *JiraService) SearchDashboards(ctx context.Context, q jira.DashboardQuery) ([]jira.Dashboard, error) {
	return s.gw.SearchDashboards(ctx, q)
}
