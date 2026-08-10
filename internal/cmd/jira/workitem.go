package jira

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"acli-plus/internal/app"
	jiradomain "acli-plus/internal/domain/jira"
)

func newWorkItemCmd(deps Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "workitem",
		Aliases: []string{"issue", "wi"},
		Short:   "Create, find and change Jira work items",
	}
	cmd.AddCommand(
		newCreateCmd(deps),
		newCreateBulkCmd(deps),
		newViewCmd(deps),
		newSearchCmd(deps),
		newEditCmd(deps),
		newDeleteCmd(deps),
		newCloneCmd(deps),
		newArchiveCmd(deps, true),
		newArchiveCmd(deps, false),
		newAssignCmd(deps),
		newTransitionCmd(deps),
		newLinkCmd(deps),
		newCommentCreateCmd(deps),
		newCommentListCmd(deps),
		newCommentUpdateCmd(deps),
		newCommentDeleteCmd(deps),
		newCommentVisibilityCmd(deps),
		newAttachmentListCmd(deps),
		newAttachmentDeleteCmd(deps),
		newWatcherListCmd(deps),
		newWatcherRemoveCmd(deps),
	)
	return cmd
}

func newCreateCmd(deps Deps) *cobra.Command {
	attrs := &attributeFlags{}
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a work item",
		Example: "  acli-plus jira workitem create -p TEAM -t Task -s \"Fix login redirect\" -a @me\n" +
			"  acli-plus jira workitem create -p TEAM -t Story --from-file docs/story.md",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			attributes, err := attrs.attributes()
			if err != nil {
				return err
			}
			if attributes.Project == "" {
				attributes.Project = deps.defaults().Project
			}
			service, err := deps.Service("")
			if err != nil {
				return err
			}
			result, err := service.CreateWorkItem(cmd.Context(), attributes, deps.Options())
			if err != nil {
				return err
			}
			printResult(result, service.Host())
			return nil
		},
	}
	attrs.register(cmd, true)
	return cmd
}

// bulkItem is one entry in the --from-json file for create-bulk.
type bulkItem struct {
	Project     string            `json:"project"`
	Type        string            `json:"type"`
	Summary     string            `json:"summary"`
	Description string            `json:"description"`
	Assignee    string            `json:"assignee"`
	Reporter    string            `json:"reporter"`
	Parent      string            `json:"parent"`
	Priority    string            `json:"priority"`
	Due         string            `json:"due"`
	Labels      []string          `json:"labels"`
	Components  []string          `json:"components"`
	FixVersions []string          `json:"fixVersions"`
	Fields      map[string]string `json:"fields"`
}

func newCreateBulkCmd(deps Deps) *cobra.Command {
	var (
		fromJSON     string
		generateJSON bool
		defaults     = &attributeFlags{}
	)
	cmd := &cobra.Command{
		Use:   "create-bulk",
		Short: "Create many work items from a JSON file",
		Long: "Create many work items from a JSON file holding an array of objects.\n" +
			"Values left out of an entry fall back to the matching flag, so a shared\n" +
			"project and type can be given once on the command line.",
		Example: "  acli-plus jira workitem create-bulk --generate-json > items.json\n" +
			"  acli-plus jira workitem create-bulk --from-json items.json -p TEAM -t Task",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if generateJSON {
				return printJSON([]bulkItem{{
					Project: "TEAM", Type: "Task", Summary: "First item",
					Description: "Markdown **supported**", Assignee: "@me",
					Labels: []string{"backend"}, Fields: map[string]string{"Story Points": "3"},
				}})
			}
			if fromJSON == "" {
				return fmt.Errorf("--from-json is required (or --generate-json for a template)")
			}

			items, err := readBulkFile(fromJSON, defaults)
			if err != nil {
				return err
			}
			for i := range items {
				if items[i].Project == "" {
					items[i].Project = deps.defaults().Project
				}
			}
			service, err := deps.Service("")
			if err != nil {
				return err
			}
			result, err := service.CreateWorkItemsBulk(cmd.Context(), items, deps.Options())
			if err != nil {
				printResult(result, service.Host())
				return err
			}
			printResult(result, service.Host())
			return nil
		},
	}
	cmd.Flags().StringVar(&fromJSON, "from-json", "", "JSON file holding an array of work items")
	cmd.Flags().BoolVar(&generateJSON, "generate-json", false, "print a template file to stdout and exit")
	defaults.register(cmd, true)
	return cmd
}

// readBulkFile parses the JSON array and folds the command-line flags in as
// defaults for entries that leave a value out.
func readBulkFile(path string, defaults *attributeFlags) ([]app.WorkItemAttributes, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	var entries []bulkItem
	if err := json.Unmarshal(source, &entries); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("%s holds no work items", path)
	}

	base, err := defaults.attributes()
	if err != nil {
		return nil, err
	}

	out := make([]app.WorkItemAttributes, 0, len(entries))
	for _, entry := range entries {
		attrs := app.WorkItemAttributes{
			Project:     firstNonEmpty(entry.Project, base.Project),
			Type:        firstNonEmpty(entry.Type, base.Type),
			Summary:     entry.Summary,
			Parent:      firstNonEmpty(entry.Parent, base.Parent),
			Priority:    firstNonEmpty(entry.Priority, base.Priority),
			Assignee:    firstNonEmpty(entry.Assignee, base.Assignee),
			Reporter:    firstNonEmpty(entry.Reporter, base.Reporter),
			Due:         firstNonEmpty(entry.Due, base.Due),
			Labels:      firstNonEmptyList(entry.Labels, base.Labels),
			Components:  firstNonEmptyList(entry.Components, base.Components),
			FixVersions: firstNonEmptyList(entry.FixVersions, base.FixVersions),
			Description: app.DescriptionSource{Text: entry.Description},
			Fields:      base.Fields,
		}
		for name, value := range entry.Fields {
			attrs.Fields = append(attrs.Fields, app.FieldAssignment{Name: name, Value: value})
		}
		out = append(out, attrs)
	}
	return out, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func firstNonEmptyList(values ...[]string) []string {
	for _, value := range values {
		if len(value) > 0 {
			return value
		}
	}
	return nil
}

func newViewCmd(deps Deps) *cobra.Command {
	var (
		out    format
		fields []string
	)
	cmd := &cobra.Command{
		Use:     "view <key>",
		Short:   "Show a work item",
		Example: "  acli-plus jira workitem view TEAM-123\n  acli-plus jira workitem view TEAM-123 --fields summary,status --json",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key, host, err := singleKey(args[0])
			if err != nil {
				return err
			}
			service, err := deps.Service(host)
			if err != nil {
				return err
			}
			item, err := service.ViewWorkItem(cmd.Context(), key, fields)
			if err != nil {
				return err
			}
			if out.json {
				return printJSON(rawOrItem(item))
			}
			printWorkItem(item, service.Host())
			return nil
		},
	}
	cmd.Flags().StringSliceVar(&fields, "fields", nil, "only fetch these fields, comma-separated")
	out.register(cmd, false)
	return cmd
}

// printWorkItem renders the detail view: a header block of the fields that fit
// on one line each, then the description.
func printWorkItem(item jiradomain.WorkItem, host string) {
	pairs := [][2]string{
		{"Key", item.Key},
		{"Summary", item.Summary},
		{"Type", item.Type},
		{"Status", item.Status},
		{"Priority", item.Priority},
		{"Resolution", item.Resolution},
		{"Assignee", item.Assignee.Name()},
		{"Reporter", item.Reporter.Name()},
		{"Parent", item.ParentKey},
		{"Labels", strings.Join(item.Labels, ", ")},
		{"Components", strings.Join(item.Components, ", ")},
		{"Fix versions", strings.Join(item.FixVersions, ", ")},
		{"Due", item.DueDate},
		{"Created", dateTime(item.Created)},
		{"Updated", dateTime(item.Updated)},
		{"URL", jiradomain.BrowseURL(host, item.Key)},
	}
	for _, pair := range pairs {
		if pair[1] != "" {
			fmt.Printf("%-13s %s\n", pair[0]+":", pair[1])
		}
	}
	if item.Description != "" {
		fmt.Printf("\n%s\n", item.Description)
	}
}

func newSearchCmd(deps Deps) *cobra.Command {
	var (
		out      format
		jql      string
		fields   []string
		limit    int
		paginate bool
	)
	cmd := &cobra.Command{
		Use:   "search",
		Short: "Find work items with JQL",
		Example: "  acli-plus jira workitem search --jql \"project = TEAM AND status = 'In Progress'\"\n" +
			"  acli-plus jira workitem search --jql \"assignee = currentUser()\" --paginate --csv",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			service, err := deps.Service("")
			if err != nil {
				return err
			}
			items, err := service.SearchWorkItems(cmd.Context(), app.SearchRequest{
				JQL: jql, Fields: fields, Limit: limit, Paginate: paginate,
			})
			if err != nil {
				return err
			}
			return renderWorkItems(items, out)
		},
	}
	cmd.Flags().StringVar(&jql, "jql", "", "JQL query (required)")
	cmd.Flags().StringSliceVar(&fields, "fields", nil, "fields to fetch, comma-separated")
	cmd.Flags().IntVar(&limit, "limit", 0, "stop after this many results (0 for one page)")
	cmd.Flags().BoolVar(&paginate, "paginate", false, "follow every page instead of just the first")
	out.register(cmd, true)
	return cmd
}

// renderWorkItems prints a result set as a table, CSV, or raw JSON.
func renderWorkItems(items []jiradomain.WorkItem, out format) error {
	table := newRows("KEY", "TYPE", "STATUS", "ASSIGNEE", "SUMMARY")
	for _, item := range items {
		table.add(item.Key, item.Type, item.Status, item.Assignee.Name(), item.Summary)
	}
	return table.render(out, rawOrItems(items))
}

func newEditCmd(deps Deps) *cobra.Command {
	var (
		targets  targetFlags
		attrs    = &attributeFlags{}
		noNotify bool
	)
	cmd := &cobra.Command{
		Use:   "edit [key...]",
		Short: "Change fields on one or more work items",
		Example: "  acli-plus jira workitem edit TEAM-1 -s \"New summary\"\n" +
			"  acli-plus jira workitem edit --jql \"project = TEAM AND labels = old\" --label new",
		RunE: func(cmd *cobra.Command, args []string) error {
			selected, host, err := targets.resolve(args)
			if err != nil {
				return err
			}
			attributes, err := attrs.attributes()
			if err != nil {
				return err
			}
			service, err := deps.Service(host)
			if err != nil {
				return err
			}
			result, err := service.EditWorkItems(cmd.Context(), selected, attributes, !noNotify, deps.Options())
			if err != nil {
				return err
			}
			printResult(result, service.Host())
			return nil
		},
	}
	targets.register(cmd, true)
	attrs.register(cmd, false)
	cmd.Flags().BoolVar(&noNotify, "no-notify", false, "do not email watchers about the change")
	return cmd
}

func newDeleteCmd(deps Deps) *cobra.Command {
	var (
		targets      targetFlags
		withSubtasks bool
	)
	cmd := &cobra.Command{
		Use:   "delete [key...]",
		Short: "Permanently delete work items",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			selected, host, err := targets.resolve(args)
			if err != nil {
				return err
			}
			service, err := deps.Service(host)
			if err != nil {
				return err
			}
			result, err := service.DeleteWorkItems(cmd.Context(), selected, withSubtasks, deps.Options())
			if err != nil {
				return err
			}
			printResult(result, service.Host())
			return nil
		},
	}
	targets.register(cmd, true)
	cmd.Flags().BoolVar(&withSubtasks, "with-subtasks", false, "also delete the work item's subtasks")
	return cmd
}

func newCloneCmd(deps Deps) *cobra.Command {
	var (
		targets targetFlags
		attrs   = &attributeFlags{}
		prefix  string
	)
	cmd := &cobra.Command{
		Use:   "clone [key...]",
		Short: "Copy work items into new ones",
		Long: "Copy work items into new ones.\n\n" +
			"The copy carries over project, type, summary, description, priority,\n" +
			"labels, components, fix versions, assignee, due date and parent. History,\n" +
			"status, comments, attachments and links are not copied; flags given here\n" +
			"override the copied values.",
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			selected, host, err := targets.resolve(args)
			if err != nil {
				return err
			}
			service, err := deps.Service(host)
			if err != nil {
				return err
			}
			keys, err := service.Resolve(cmd.Context(), selected)
			if err != nil {
				return err
			}
			attributes, err := attrs.attributes()
			if err != nil {
				return err
			}
			result, err := service.CloneWorkItems(cmd.Context(), keys, prefix, attributes, deps.Options())
			if err != nil {
				return err
			}
			printResult(result, service.Host())
			return nil
		},
	}
	targets.register(cmd, true)
	attrs.register(cmd, false)
	cmd.Flags().StringVar(&prefix, "prefix", "CLONE -", "text put in front of the copied summary")
	return cmd
}

// newArchiveCmd builds both archive and unarchive, which differ only in verb.
func newArchiveCmd(deps Deps, archive bool) *cobra.Command {
	var targets targetFlags
	use, short := "unarchive", "Restore archived work items"
	if archive {
		use, short = "archive", "Archive work items (Jira Premium and Enterprise only)"
	}

	cmd := &cobra.Command{
		Use:   use + " [key...]",
		Short: short,
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			selected, host, err := targets.resolve(args)
			if err != nil {
				return err
			}
			service, err := deps.Service(host)
			if err != nil {
				return err
			}
			keys, err := service.Resolve(cmd.Context(), selected)
			if err != nil {
				return err
			}
			result, err := service.SetArchived(cmd.Context(), keys, archive, deps.Options())
			if err != nil {
				return err
			}
			printResult(result, service.Host())
			return nil
		},
	}
	targets.register(cmd, true)
	return cmd
}

func newAssignCmd(deps Deps) *cobra.Command {
	var (
		targets  targetFlags
		assignee string
	)
	cmd := &cobra.Command{
		Use:   "assign [key...]",
		Short: "Set the assignee on work items",
		Example: "  acli-plus jira workitem assign TEAM-1 --assignee @me\n" +
			"  acli-plus jira workitem assign TEAM-1 --assignee \"\"   # unassign",
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("assignee") {
				return fmt.Errorf("--assignee is required (pass an empty value to unassign)")
			}
			selected, host, err := targets.resolve(args)
			if err != nil {
				return err
			}
			service, err := deps.Service(host)
			if err != nil {
				return err
			}
			result, err := service.AssignWorkItems(cmd.Context(), selected, assignee, deps.Options())
			if err != nil {
				return err
			}
			printResult(result, service.Host())
			return nil
		},
	}
	targets.register(cmd, true)
	cmd.Flags().StringVarP(&assignee, "assignee", "a", "", "email, display name, or account id; @me for yourself, empty to unassign")
	return cmd
}

func newTransitionCmd(deps Deps) *cobra.Command {
	var (
		targets targetFlags
		attrs   = &attributeFlags{}
		status  string
		list    bool
		out     format
	)
	cmd := &cobra.Command{
		Use:   "transition [key...]",
		Short: "Move work items to another status",
		Example: "  acli-plus jira workitem transition TEAM-1 --status Done\n" +
			"  acli-plus jira workitem transition TEAM-1 --list",
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			selected, host, err := targets.resolve(args)
			if err != nil {
				return err
			}
			service, err := deps.Service(host)
			if err != nil {
				return err
			}

			if list {
				keys, err := service.Resolve(cmd.Context(), selected)
				if err != nil {
					return err
				}
				transitions, err := service.ListTransitions(cmd.Context(), keys[0])
				if err != nil {
					return err
				}
				table := newRows("ID", "NAME", "MOVES TO", "SCREEN")
				for _, transition := range transitions {
					table.add(transition.ID, transition.Name, transition.ToName, yesNo(transition.HasScreen))
				}
				return table.render(out, transitions)
			}

			if status == "" {
				return fmt.Errorf("--status is required (or --list to see what is available)")
			}
			attributes, err := attrs.attributes()
			if err != nil {
				return err
			}
			result, err := service.TransitionWorkItems(cmd.Context(), selected, status, attributes, deps.Options())
			if err != nil {
				return err
			}
			printResult(result, service.Host())
			return nil
		},
	}
	targets.register(cmd, true)
	attrs.register(cmd, false)
	cmd.Flags().StringVar(&status, "status", "", "destination status (e.g. Done)")
	cmd.Flags().BoolVar(&list, "list", false, "list the transitions available from the current status")
	out.register(cmd, false)
	return cmd
}

func newLinkCmd(deps Deps) *cobra.Command {
	var (
		linkType  string
		inward    string
		outward   string
		comment   string
		listTypes bool
		out       format
	)
	cmd := &cobra.Command{
		Use:   "link",
		Short: "Link two work items",
		Example: "  acli-plus jira workitem link --type Blocks --inward TEAM-1 --outward TEAM-2\n" +
			"  acli-plus jira workitem link --list-types",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			service, err := deps.Service("")
			if err != nil {
				return err
			}

			if listTypes {
				types, err := service.ListLinkTypes(cmd.Context())
				if err != nil {
					return err
				}
				table := newRows("NAME", "INWARD", "OUTWARD")
				for _, linkType := range types {
					table.add(linkType.Name, linkType.Inward, linkType.Outward)
				}
				return table.render(out, types)
			}

			if linkType == "" || inward == "" || outward == "" {
				return fmt.Errorf("--type, --inward and --outward are required (or --list-types)")
			}
			inwardKey, _, err := singleKey(inward)
			if err != nil {
				return err
			}
			outwardKey, _, err := singleKey(outward)
			if err != nil {
				return err
			}

			result, err := service.LinkWorkItems(cmd.Context(), jiradomain.NewLinkInput{
				Type: linkType, Inward: inwardKey, Outward: outwardKey, Comment: comment,
			}, deps.Options())
			if err != nil {
				return err
			}
			printResult(result, service.Host())
			return nil
		},
	}
	cmd.Flags().StringVar(&linkType, "type", "", "link type name (e.g. Blocks, Relates)")
	cmd.Flags().StringVar(&inward, "inward", "", "work item on the inward side of the link")
	cmd.Flags().StringVar(&outward, "outward", "", "work item on the outward side of the link")
	cmd.Flags().StringVar(&comment, "comment", "", "comment to add alongside the link")
	cmd.Flags().BoolVar(&listTypes, "list-types", false, "list the link types configured on this site")
	out.register(cmd, false)
	return cmd
}
