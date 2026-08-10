package jira

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	jiradomain "acli-plus/internal/domain/jira"
)

func newProjectCmd(deps Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project",
		Short: "List and manage Jira projects",
	}
	cmd.AddCommand(
		newProjectListCmd(deps),
		newProjectViewCmd(deps),
		newProjectCreateCmd(deps),
		newProjectUpdateCmd(deps),
		newProjectDeleteCmd(deps),
		newProjectArchiveCmd(deps, true),
		newProjectArchiveCmd(deps, false),
	)
	return cmd
}

func newProjectListCmd(deps Deps) *cobra.Command {
	var (
		out   format
		query jiradomain.ProjectQuery
	)
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List the projects you can see",
		Example: "  acli-plus jira project list\n  acli-plus jira project list --query team --type software",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			service, err := deps.Service("")
			if err != nil {
				return err
			}
			projects, err := service.ListProjects(cmd.Context(), query)
			if err != nil {
				return err
			}
			table := newRows("KEY", "NAME", "TYPE", "STYLE", "LEAD")
			for _, project := range projects {
				table.add(project.Key, project.Name, project.TypeKey, project.Style, project.Lead.Name())
			}
			return table.render(out, projects)
		},
	}
	cmd.Flags().StringVar(&query.Query, "query", "", "match against project key and name")
	cmd.Flags().StringVar(&query.TypeKey, "type", "", "software, service_desk, or business")
	cmd.Flags().StringVar(&query.Status, "status", "", "live, archived, or deleted")
	cmd.Flags().IntVar(&query.MaxResults, "limit", 0, "stop after this many projects")
	out.register(cmd, true)
	return cmd
}

func newProjectViewCmd(deps Deps) *cobra.Command {
	var out format
	cmd := &cobra.Command{
		Use:   "view <key>",
		Short: "Show one project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			service, err := deps.Service("")
			if err != nil {
				return err
			}
			project, err := service.ViewProject(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if out.json {
				if len(project.Raw) > 0 {
					return printJSON(project.Raw)
				}
				return printJSON(project)
			}
			for _, pair := range [][2]string{
				{"Key", project.Key},
				{"Name", project.Name},
				{"Id", project.ID},
				{"Type", project.TypeKey},
				{"Style", project.Style},
				{"Lead", project.Lead.Name()},
				{"Archived", yesNo(project.Archived)},
				{"URL", project.URL},
			} {
				if pair[1] != "" {
					fmt.Printf("%-10s %s\n", pair[0]+":", pair[1])
				}
			}
			return nil
		},
	}
	out.register(cmd, false)
	return cmd
}

func newProjectCreateCmd(deps Deps) *cobra.Command {
	var (
		in   jiradomain.NewProjectInput
		lead string
	)
	cmd := &cobra.Command{
		Use:     "create",
		Short:   "Create a project",
		Example: "  acli-plus jira project create --key TEAM --name \"Team Space\" --type software",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			service, err := deps.Service("")
			if err != nil {
				return err
			}
			result, err := service.CreateProject(cmd.Context(), in, lead, deps.Options())
			if err != nil {
				return err
			}
			printResult(result, "")
			return nil
		},
	}
	cmd.Flags().StringVar(&in.Key, "key", "", "project key (required)")
	cmd.Flags().StringVar(&in.Name, "name", "", "project name (required)")
	cmd.Flags().StringVar(&in.TypeKey, "type", "software", "software, service_desk, or business")
	cmd.Flags().StringVar(&in.TemplateKey, "template", "", "project template key (defaults to one matching --type)")
	cmd.Flags().StringVar(&in.Description, "description", "", "project description")
	cmd.Flags().StringVar(&in.AssigneeType, "assignee-type", "", "PROJECT_LEAD or UNASSIGNED")
	cmd.Flags().StringVar(&lead, "lead", "", "project lead by email, name, or account id (defaults to you)")
	return cmd
}

func newProjectUpdateCmd(deps Deps) *cobra.Command {
	var (
		in   jiradomain.UpdateProjectInput
		lead string
	)
	cmd := &cobra.Command{
		Use:   "update <key>",
		Short: "Change a project's name, key, description, or lead",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			in.KeyOrID = args[0]
			service, err := deps.Service("")
			if err != nil {
				return err
			}
			result, err := service.UpdateProject(cmd.Context(), in, lead, deps.Options())
			if err != nil {
				return err
			}
			printResult(result, "")
			return nil
		},
	}
	cmd.Flags().StringVar(&in.NewKey, "new-key", "", "change the project key")
	cmd.Flags().StringVar(&in.Name, "name", "", "change the project name")
	cmd.Flags().StringVar(&in.Description, "description", "", "change the project description")
	cmd.Flags().StringVar(&in.AssigneeType, "assignee-type", "", "PROJECT_LEAD or UNASSIGNED")
	cmd.Flags().StringVar(&lead, "lead", "", "change the project lead")
	return cmd
}

func newProjectDeleteCmd(deps Deps) *cobra.Command {
	var noUndo bool
	cmd := &cobra.Command{
		Use:   "delete <key>",
		Short: "Delete a project and everything in it",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			service, err := deps.Service("")
			if err != nil {
				return err
			}
			result, err := service.DeleteProject(cmd.Context(), args[0], !noUndo, deps.Options())
			if err != nil {
				return err
			}
			printResult(result, "")
			return nil
		},
	}
	cmd.Flags().BoolVar(&noUndo, "no-undo", false, "delete immediately instead of moving the project to the trash")
	return cmd
}

// newProjectArchiveCmd builds both archive and restore.
func newProjectArchiveCmd(deps Deps, archive bool) *cobra.Command {
	use, short := "restore", "Restore an archived or trashed project"
	if archive {
		use, short = "archive", "Archive a project (Jira Premium and Enterprise only)"
	}
	return &cobra.Command{
		Use:   use + " <key>",
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			service, err := deps.Service("")
			if err != nil {
				return err
			}
			result, err := service.SetProjectArchived(cmd.Context(), args[0], archive, deps.Options())
			if err != nil {
				return err
			}
			printResult(result, "")
			return nil
		},
	}
}

func newFieldCmd(deps Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "field",
		Short: "List and manage Jira fields",
	}
	cmd.AddCommand(
		newFieldListCmd(deps),
		newFieldCreateCmd(deps),
		newFieldDeleteCmd(deps, true),
		newFieldDeleteCmd(deps, false),
	)
	return cmd
}

func newFieldListCmd(deps Deps) *cobra.Command {
	var (
		out        format
		customOnly bool
		query      string
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List fields and their ids",
		Long: "List fields and their ids.\n\n" +
			"This is how you find the customfield_NNNNN id behind a custom field, though\n" +
			"'workitem create' and 'workitem edit' also accept a field's display name.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			service, err := deps.Service("")
			if err != nil {
				return err
			}
			fields, err := service.ListFields(cmd.Context())
			if err != nil {
				return err
			}
			table := newRows("ID", "NAME", "TYPE", "CUSTOM")
			for _, field := range fields {
				if customOnly && !field.Custom {
					continue
				}
				if query != "" && !strings.Contains(strings.ToLower(field.Name), strings.ToLower(query)) {
					continue
				}
				schema := field.SchemaType
				if field.ItemType != "" {
					schema += "[" + field.ItemType + "]"
				}
				table.add(field.ID, field.Name, schema, yesNo(field.Custom))
			}
			return table.render(out, fields)
		},
	}
	cmd.Flags().BoolVar(&customOnly, "custom", false, "only show custom fields")
	cmd.Flags().StringVar(&query, "query", "", "only show fields whose name contains this text")
	out.register(cmd, true)
	return cmd
}

func newFieldCreateCmd(deps Deps) *cobra.Command {
	var in jiradomain.NewFieldInput
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a custom field",
		Example: "  acli-plus jira field create --name \"Team\" \\\n" +
			"    --type com.atlassian.jira.plugin.system.customfieldtypes:textfield",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			service, err := deps.Service("")
			if err != nil {
				return err
			}
			result, err := service.CreateField(cmd.Context(), in, deps.Options())
			if err != nil {
				return err
			}
			printResult(result, "")
			return nil
		},
	}
	cmd.Flags().StringVar(&in.Name, "name", "", "field name (required)")
	cmd.Flags().StringVar(&in.Type, "type", "", "fully qualified custom field type key (required)")
	cmd.Flags().StringVar(&in.Description, "description", "", "field description")
	cmd.Flags().StringVar(&in.SearcherKey, "searcher", "", "fully qualified searcher key")
	return cmd
}

// newFieldDeleteCmd builds delete and cancel-delete, which acli names after
// trashing and restoring a custom field.
func newFieldDeleteCmd(deps Deps, delete bool) *cobra.Command {
	use, short := "cancel-delete <field>", "Restore a custom field from the trash"
	if delete {
		use, short = "delete <field>", "Move a custom field to the trash"
	}
	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			service, err := deps.Service("")
			if err != nil {
				return err
			}
			result, err := service.SetFieldDeleted(cmd.Context(), args[0], delete, deps.Options())
			if err != nil {
				return err
			}
			printResult(result, "")
			return nil
		},
	}
}
