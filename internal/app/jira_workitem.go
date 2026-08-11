package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	jira "acli-plus/internal/domain/jira"
	"acli-plus/internal/markdown"
)

// bulkChunkSize is the number of work items Jira accepts in one bulk create.
const bulkChunkSize = 50

// WorkItemAttributes is the flag-level description of a work item, shared by
// create, create-bulk, clone and edit. Empty values are left out of the request
// so edit only touches what the user actually passed.
type WorkItemAttributes struct {
	Project     string
	Type        string
	Summary     string
	Parent      string
	Priority    string
	Assignee    string
	Reporter    string
	Due         string
	Labels      []string
	Components  []string
	FixVersions []string
	Description DescriptionSource
	Fields      []FieldAssignment
}

// Targets selects the work items a command acts on: an explicit key list, a JQL
// query, or both. acli accepts either, and so does every bulk command here.
type Targets struct {
	Keys []string
	JQL  string
}

// Resolve expands the targets into concrete work item keys, running the JQL
// query when one is given.
func (s *JiraService) Resolve(ctx context.Context, in Targets) ([]string, error) {
	keys := append([]string(nil), in.Keys...)

	jql := strings.TrimSpace(in.JQL)
	if jql != "" {
		page, err := s.SearchWorkItems(ctx, SearchRequest{JQL: jql, Fields: []string{"summary"}, Paginate: true})
		if err != nil {
			return nil, err
		}
		for _, item := range page {
			keys = append(keys, item.Key)
		}
		// A query that ran fine but matched nothing is a different problem from
		// forgetting to select anything, and saying "pass --jql" to someone who
		// just passed --jql sends them looking in the wrong place.
		if len(keys) == 0 {
			return nil, fmt.Errorf("the JQL query matched no work items: %s", jql)
		}
	}

	if len(keys) == 0 {
		return nil, fmt.Errorf("no work items selected: pass --key or --jql")
	}
	return keys, nil
}

// buildFields turns attributes into a Jira fields object. requireCore is set by
// create, where project, type and summary are mandatory; edit leaves it off so
// a partial update stays partial.
func (s *JiraService) buildFields(ctx context.Context, attrs WorkItemAttributes, requireCore bool) (jira.FieldValues, []string, error) {
	fields := jira.FieldValues{}
	var warnings []string

	rendered, err := RenderDescription(attrs.Description)
	if err != nil {
		return nil, nil, err
	}
	warnings = append(warnings, rendered.Warnings...)

	summary := attrs.Summary
	if summary == "" {
		// A Markdown file's frontmatter title or leading H1 becomes the summary,
		// which is what --from-file means.
		summary = rendered.Title
	}
	if summary != "" {
		fields["summary"] = summary
	}
	if !rendered.Body.Empty() {
		fields["description"] = rendered.Body
	}
	if attrs.Project != "" {
		fields["project"] = map[string]string{"key": attrs.Project}
	}
	if attrs.Type != "" {
		fields["issuetype"] = map[string]string{"name": attrs.Type}
	}
	if attrs.Parent != "" {
		fields["parent"] = map[string]string{"key": strings.ToUpper(attrs.Parent)}
	}
	if attrs.Priority != "" {
		fields["priority"] = map[string]string{"name": attrs.Priority}
	}
	if attrs.Due != "" {
		fields["duedate"] = attrs.Due
	}
	if len(attrs.Labels) > 0 {
		fields["labels"] = attrs.Labels
	}
	if len(attrs.Components) > 0 {
		fields["components"] = namedList(attrs.Components, "name")
	}
	if len(attrs.FixVersions) > 0 {
		fields["fixVersions"] = namedList(attrs.FixVersions, "name")
	}
	for flag, value := range map[string]string{"assignee": attrs.Assignee, "reporter": attrs.Reporter} {
		if value == "" {
			continue
		}
		accountID, err := s.ResolveAccountID(ctx, value)
		if err != nil {
			return nil, nil, err
		}
		fields[flag] = map[string]any{"accountId": nilIfEmpty(accountID)}
	}

	explicit, err := s.BuildFields(ctx, attrs.Fields)
	if err != nil {
		return nil, nil, err
	}
	fields = fields.Merge(explicit)

	if requireCore {
		for _, required := range []struct{ key, flag string }{
			{"project", "--project"},
			{"issuetype", "--type"},
			{"summary", "--summary"},
		} {
			if _, ok := fields[required.key]; !ok {
				return nil, nil, fmt.Errorf("%s is required to create a work item", required.flag)
			}
		}
	}
	return fields, warnings, nil
}

func nilIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}

// CreateWorkItem creates a single work item.
func (s *JiraService) CreateWorkItem(ctx context.Context, attrs WorkItemAttributes, opts WriteOptions) (JiraResult, error) {
	fields, warnings, err := s.buildFields(ctx, attrs, true)
	if err != nil {
		return JiraResult{}, err
	}

	summary, _ := fields["summary"].(string)
	if opts.DryRun {
		return JiraResult{
			Detail:   fmt.Sprintf("create %s %q in %s", attrs.Type, summary, attrs.Project),
			DryRun:   true,
			Warnings: warnings,
		}, nil
	}

	item, err := s.gw.CreateWorkItem(ctx, jira.NewWorkItemInput{Fields: fields})
	if err != nil {
		return JiraResult{}, err
	}
	return JiraResult{
		Detail:   fmt.Sprintf("created %s %q", item.Key, summary),
		Keys:     []string{item.Key},
		Warnings: warnings,
	}, nil
}

// CreateWorkItemsBulk creates many work items, chunked to the API's limit.
func (s *JiraService) CreateWorkItemsBulk(ctx context.Context, all []WorkItemAttributes, opts WriteOptions) (JiraResult, error) {
	inputs := make([]jira.NewWorkItemInput, 0, len(all))
	var warnings []string
	for i, attrs := range all {
		fields, itemWarnings, err := s.buildFields(ctx, attrs, true)
		if err != nil {
			return JiraResult{}, fmt.Errorf("item %d: %w", i+1, err)
		}
		inputs = append(inputs, jira.NewWorkItemInput{Fields: fields})
		warnings = append(warnings, itemWarnings...)
	}

	if opts.DryRun {
		return JiraResult{
			Detail:   fmt.Sprintf("create %d work items", len(inputs)),
			DryRun:   true,
			Warnings: warnings,
		}, nil
	}

	var keys []string
	for start := 0; start < len(inputs); start += bulkChunkSize {
		end := min(start+bulkChunkSize, len(inputs))
		items, err := s.gw.CreateWorkItemsBulk(ctx, inputs[start:end])
		for _, item := range items {
			keys = append(keys, item.Key)
		}
		if err != nil {
			// Report what did get created before failing, so a partial bulk run
			// is recoverable rather than a mystery.
			return JiraResult{Keys: keys, Warnings: warnings}, err
		}
	}
	return JiraResult{
		Detail:   fmt.Sprintf("created %d work items: %s", len(keys), strings.Join(keys, ", ")),
		Keys:     keys,
		Warnings: warnings,
	}, nil
}

// ViewWorkItem fetches one work item.
func (s *JiraService) ViewWorkItem(ctx context.Context, key string, fields []string) (jira.WorkItem, error) {
	return s.gw.GetWorkItem(ctx, key, fields)
}

// SearchRequest is a JQL query with its paging behavior.
type SearchRequest struct {
	JQL    string
	Fields []string
	// Limit caps the rows returned. Zero means one API page.
	Limit int
	// Paginate follows next-page tokens until the query is exhausted or Limit
	// is reached.
	Paginate bool
}

// SearchWorkItems runs a JQL query, following pages when asked. The search API
// reports no total, so the only way to know the size of a result set is to walk
// it — which is what --paginate does.
func (s *JiraService) SearchWorkItems(ctx context.Context, req SearchRequest) ([]jira.WorkItem, error) {
	if strings.TrimSpace(req.JQL) == "" {
		return nil, fmt.Errorf("a JQL query is required (--jql)")
	}

	var collected []jira.WorkItem
	token := ""
	for {
		page, err := s.gw.Search(ctx, jira.SearchInput{
			JQL:        req.JQL,
			Fields:     req.Fields,
			MaxResults: req.Limit,
			PageToken:  token,
		})
		if err != nil {
			return nil, err
		}
		collected = append(collected, page.Items...)

		if req.Limit > 0 && len(collected) >= req.Limit {
			return collected[:req.Limit], nil
		}
		if !req.Paginate || page.NextPageToken == "" || len(page.Items) == 0 {
			return collected, nil
		}
		token = page.NextPageToken
	}
}

// EditWorkItems applies the same field changes to every selected work item.
func (s *JiraService) EditWorkItems(ctx context.Context, in Targets, attrs WorkItemAttributes, notify bool, opts WriteOptions) (JiraResult, error) {
	keys, err := s.Resolve(ctx, in)
	if err != nil {
		return JiraResult{}, err
	}
	fields, warnings, err := s.buildFields(ctx, attrs, false)
	if err != nil {
		return JiraResult{}, err
	}
	if len(fields) == 0 {
		return JiraResult{}, fmt.Errorf("nothing to change: pass at least one field flag")
	}

	if opts.DryRun {
		return JiraResult{
			Detail:   fmt.Sprintf("edit %s (%s)", strings.Join(keys, ", "), strings.Join(fieldNames(fields), ", ")),
			Keys:     keys,
			DryRun:   true,
			Warnings: warnings,
		}, nil
	}

	for _, key := range keys {
		if err := s.gw.UpdateWorkItem(ctx, jira.EditWorkItemInput{Key: key, Fields: fields, Notify: notify}); err != nil {
			return JiraResult{}, fmt.Errorf("%s: %w", key, err)
		}
	}
	return JiraResult{
		Detail:   fmt.Sprintf("updated %s", strings.Join(keys, ", ")),
		Keys:     keys,
		Warnings: warnings,
	}, nil
}

func fieldNames(fields jira.FieldValues) []string {
	names := make([]string, 0, len(fields))
	for name := range fields {
		names = append(names, name)
	}
	return names
}

// DeleteWorkItems permanently deletes work items after confirmation. Unlike a
// Confluence page delete, this is not reversible, so the prompt says so.
func (s *JiraService) DeleteWorkItems(ctx context.Context, in Targets, withSubtasks bool, opts WriteOptions) (JiraResult, error) {
	keys, err := s.Resolve(ctx, in)
	if err != nil {
		return JiraResult{}, err
	}

	if opts.DryRun {
		return JiraResult{Detail: "delete " + strings.Join(keys, ", "), Keys: keys, DryRun: true}, nil
	}

	prompt := fmt.Sprintf("Permanently delete %d work item(s): %s? This cannot be undone.",
		len(keys), strings.Join(keys, ", "))
	ok, err := confirmOrAbort(opts, prompt)
	if err != nil {
		return JiraResult{}, err
	}
	if !ok {
		return aborted(), nil
	}

	for _, key := range keys {
		if err := s.gw.DeleteWorkItem(ctx, key, withSubtasks); err != nil {
			return JiraResult{}, fmt.Errorf("%s: %w", key, err)
		}
	}
	return JiraResult{Detail: "deleted " + strings.Join(keys, ", "), Keys: keys}, nil
}

// cloneFields are the fields a clone copies. Read-only and history fields
// (status, resolution, comments, attachments, links, worklog) are deliberately
// left out: Jira rejects them on create, and a clone starts fresh anyway.
var cloneFields = []string{
	"project", "issuetype", "summary", "description", "priority",
	"labels", "components", "fixVersions", "assignee", "duedate", "parent",
}

// CloneWorkItems copies work items into new ones, prefixing the summary.
func (s *JiraService) CloneWorkItems(ctx context.Context, keys []string, prefix string, attrs WorkItemAttributes, opts WriteOptions) (JiraResult, error) {
	overrides, warnings, err := s.buildFields(ctx, attrs, false)
	if err != nil {
		return JiraResult{}, err
	}

	if opts.DryRun {
		return JiraResult{
			Detail:   fmt.Sprintf("clone %s", strings.Join(keys, ", ")),
			Keys:     keys,
			DryRun:   true,
			Warnings: warnings,
		}, nil
	}

	created := make([]string, 0, len(keys))
	for _, key := range keys {
		source, err := s.gw.GetWorkItem(ctx, key, cloneFields)
		if err != nil {
			return JiraResult{}, fmt.Errorf("%s: %w", key, err)
		}

		fields := cloneFieldsOf(source)
		fields["summary"] = strings.TrimSpace(prefix + " " + source.Summary)
		fields = fields.Merge(overrides)

		item, err := s.gw.CreateWorkItem(ctx, jira.NewWorkItemInput{Fields: fields})
		if err != nil {
			return JiraResult{Keys: created}, fmt.Errorf("cloning %s: %w", key, err)
		}
		created = append(created, item.Key)
	}
	return JiraResult{
		Detail:   fmt.Sprintf("cloned into %s", strings.Join(created, ", ")),
		Keys:     created,
		Warnings: warnings,
	}, nil
}

// cloneFieldsOf rebuilds a create payload from a fetched work item.
func cloneFieldsOf(source jira.WorkItem) jira.FieldValues {
	fields := jira.FieldValues{
		"project":   map[string]string{"key": source.ProjectKey},
		"issuetype": map[string]string{"name": source.Type},
	}
	if source.Priority != "" {
		fields["priority"] = map[string]string{"name": source.Priority}
	}
	if len(source.Labels) > 0 {
		fields["labels"] = source.Labels
	}
	if len(source.Components) > 0 {
		fields["components"] = namedList(source.Components, "name")
	}
	if len(source.FixVersions) > 0 {
		fields["fixVersions"] = namedList(source.FixVersions, "name")
	}
	if source.Assignee.AccountID != "" {
		fields["assignee"] = map[string]string{"accountId": source.Assignee.AccountID}
	}
	if source.DueDate != "" {
		fields["duedate"] = source.DueDate
	}
	if source.ParentKey != "" {
		fields["parent"] = map[string]string{"key": source.ParentKey}
	}
	if body := descriptionADF(source); len(body) > 0 {
		fields["description"] = jira.Document(body)
	}
	return fields
}

// descriptionADF recovers the original ADF description from a fetched work
// item's raw payload, so a clone keeps the source's formatting instead of the
// plain-text flattening used for display. It falls back to that flattening when
// the raw payload did not carry the field, and returns nil when there is no
// description at all — the presence of ADF is what decides, not the flattened
// text, which can be empty for a description made only of media or mentions.
func descriptionADF(source jira.WorkItem) json.RawMessage {
	var payload struct {
		Fields struct {
			Description json.RawMessage `json:"description"`
		} `json:"fields"`
	}
	if err := json.Unmarshal(source.Raw, &payload); err == nil {
		if body := payload.Fields.Description; len(body) > 0 && string(body) != "null" {
			return body
		}
	}
	if source.Description == "" {
		return nil
	}
	return markdown.TextToADF(source.Description)
}

// SetArchived archives or restores work items.
func (s *JiraService) SetArchived(ctx context.Context, keys []string, archived bool, opts WriteOptions) (JiraResult, error) {
	verb := "unarchive"
	if archived {
		verb = "archive"
	}
	if opts.DryRun {
		return JiraResult{Detail: verb + " " + strings.Join(keys, ", "), Keys: keys, DryRun: true}, nil
	}

	call := s.gw.UnarchiveWorkItems
	if archived {
		call = s.gw.ArchiveWorkItems
	}
	if err := call(ctx, keys); err != nil {
		return JiraResult{}, err
	}
	return JiraResult{Detail: verb + "d " + strings.Join(keys, ", "), Keys: keys}, nil
}

// AssignWorkItems sets the assignee on every selected work item.
func (s *JiraService) AssignWorkItems(ctx context.Context, in Targets, assignee string, opts WriteOptions) (JiraResult, error) {
	keys, err := s.Resolve(ctx, in)
	if err != nil {
		return JiraResult{}, err
	}
	accountID, err := s.ResolveAccountID(ctx, assignee)
	if err != nil {
		return JiraResult{}, err
	}

	target := assignee
	if accountID == "" {
		target = "nobody"
	}
	if opts.DryRun {
		return JiraResult{
			Detail: fmt.Sprintf("assign %s to %s", strings.Join(keys, ", "), target),
			Keys:   keys,
			DryRun: true,
		}, nil
	}

	for _, key := range keys {
		if err := s.gw.AssignWorkItem(ctx, key, accountID); err != nil {
			return JiraResult{}, fmt.Errorf("%s: %w", key, err)
		}
	}
	return JiraResult{
		Detail: fmt.Sprintf("assigned %s to %s", strings.Join(keys, ", "), target),
		Keys:   keys,
	}, nil
}

// ListTransitions returns the moves available from a work item's current status.
func (s *JiraService) ListTransitions(ctx context.Context, key string) ([]jira.Transition, error) {
	return s.gw.ListTransitions(ctx, key)
}

// TransitionWorkItems moves work items to a named status. The status is matched
// against the transitions Jira currently offers, by destination status first
// and then by transition name.
func (s *JiraService) TransitionWorkItems(ctx context.Context, in Targets, status string, attrs WorkItemAttributes, opts WriteOptions) (JiraResult, error) {
	keys, err := s.Resolve(ctx, in)
	if err != nil {
		return JiraResult{}, err
	}
	fields, warnings, err := s.buildFields(ctx, attrs, false)
	if err != nil {
		return JiraResult{}, err
	}

	if opts.DryRun {
		return JiraResult{
			Detail:   fmt.Sprintf("transition %s to %q", strings.Join(keys, ", "), status),
			Keys:     keys,
			DryRun:   true,
			Warnings: warnings,
		}, nil
	}

	for _, key := range keys {
		transitions, err := s.gw.ListTransitions(ctx, key)
		if err != nil {
			return JiraResult{}, fmt.Errorf("%s: %w", key, err)
		}
		match, ok := matchTransition(transitions, status)
		if !ok {
			return JiraResult{}, fmt.Errorf("%s: %w (available: %s)",
				key, jira.ErrTransitionNotFound, describeTransitions(transitions))
		}
		if err := s.gw.TransitionWorkItem(ctx, key, match.ID, fields); err != nil {
			return JiraResult{}, fmt.Errorf("%s: %w", key, err)
		}
	}
	return JiraResult{
		Detail:   fmt.Sprintf("moved %s to %q", strings.Join(keys, ", "), status),
		Keys:     keys,
		Warnings: warnings,
	}, nil
}

func matchTransition(transitions []jira.Transition, status string) (jira.Transition, bool) {
	for _, transition := range transitions {
		if strings.EqualFold(transition.ToName, status) {
			return transition, true
		}
	}
	for _, transition := range transitions {
		if strings.EqualFold(transition.Name, status) || transition.ID == status {
			return transition, true
		}
	}
	return jira.Transition{}, false
}

func describeTransitions(transitions []jira.Transition) string {
	if len(transitions) == 0 {
		return "none"
	}
	names := make([]string, 0, len(transitions))
	for _, transition := range transitions {
		names = append(names, transition.ToName)
	}
	return strings.Join(names, ", ")
}

// LinkWorkItems creates a link between two work items.
func (s *JiraService) LinkWorkItems(ctx context.Context, in jira.NewLinkInput, opts WriteOptions) (JiraResult, error) {
	types, err := s.gw.ListLinkTypes(ctx)
	if err != nil {
		return JiraResult{}, err
	}
	name, ok := matchLinkType(types, in.Type)
	if !ok {
		return JiraResult{}, fmt.Errorf("%w: %q (available: %s)",
			jira.ErrLinkTypeNotFound, in.Type, describeLinkTypes(types))
	}
	in.Type = name

	detail := fmt.Sprintf("link %s %s %s", in.Inward, strings.ToLower(name), in.Outward)
	if opts.DryRun {
		return JiraResult{Detail: detail, Keys: []string{in.Inward, in.Outward}, DryRun: true}, nil
	}
	if err := s.gw.CreateLink(ctx, in); err != nil {
		return JiraResult{}, err
	}
	return JiraResult{Detail: "linked " + in.Inward + " -> " + in.Outward + " (" + name + ")",
		Keys: []string{in.Inward, in.Outward}}, nil
}

// ListLinkTypes returns the link types configured on the site.
func (s *JiraService) ListLinkTypes(ctx context.Context) ([]jira.LinkType, error) {
	return s.gw.ListLinkTypes(ctx)
}

// matchLinkType accepts the type name or either of its directional phrases, so
// "blocks" and "Blocks" both resolve.
func matchLinkType(types []jira.LinkType, value string) (string, bool) {
	for _, linkType := range types {
		if strings.EqualFold(linkType.Name, value) ||
			strings.EqualFold(linkType.Inward, value) ||
			strings.EqualFold(linkType.Outward, value) {
			return linkType.Name, true
		}
	}
	return "", false
}

func describeLinkTypes(types []jira.LinkType) string {
	names := make([]string, 0, len(types))
	for _, linkType := range types {
		names = append(names, linkType.Name)
	}
	return strings.Join(names, ", ")
}

// ListWatchers returns who is watching a work item.
func (s *JiraService) ListWatchers(ctx context.Context, key string) ([]jira.User, error) {
	return s.gw.ListWatchers(ctx, key)
}

// RemoveWatchers removes watchers from a work item.
func (s *JiraService) RemoveWatchers(ctx context.Context, key string, watchers []string, opts WriteOptions) (JiraResult, error) {
	accountIDs := make([]string, 0, len(watchers))
	for _, watcher := range watchers {
		accountID, err := s.ResolveAccountID(ctx, watcher)
		if err != nil {
			return JiraResult{}, err
		}
		if accountID == "" {
			return JiraResult{}, fmt.Errorf("%w: %s", jira.ErrUserNotFound, watcher)
		}
		accountIDs = append(accountIDs, accountID)
	}

	if opts.DryRun {
		return JiraResult{
			Detail: fmt.Sprintf("remove %d watcher(s) from %s", len(accountIDs), key),
			Keys:   []string{key},
			DryRun: true,
		}, nil
	}
	for _, accountID := range accountIDs {
		if err := s.gw.RemoveWatcher(ctx, key, accountID); err != nil {
			return JiraResult{}, err
		}
	}
	return JiraResult{
		Detail: fmt.Sprintf("removed %d watcher(s) from %s", len(accountIDs), key),
		Keys:   []string{key},
	}, nil
}
