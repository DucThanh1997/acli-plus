package jira

import (
	"fmt"

	"github.com/spf13/cobra"

	jiradomain "acli-plus/internal/domain/jira"
)

func newBoardCmd(deps Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "board",
		Short: "Find Agile boards and their sprints",
	}
	cmd.AddCommand(newBoardSearchCmd(deps), newBoardSprintsCmd(deps))
	return cmd
}

func newBoardSearchCmd(deps Deps) *cobra.Command {
	var (
		out   format
		query jiradomain.BoardQuery
	)
	cmd := &cobra.Command{
		Use:     "search",
		Short:   "Search the boards you can see",
		Example: "  acli-plus jira board search --project TEAM",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if query.ProjectKey == "" {
				query.ProjectKey = deps.defaults().Project
			}
			service, err := deps.Service("")
			if err != nil {
				return err
			}
			boards, err := service.SearchBoards(cmd.Context(), query)
			if err != nil {
				return err
			}
			table := newRows("ID", "NAME", "TYPE", "PROJECT")
			for _, board := range boards {
				table.add(itoa(board.ID), board.Name, board.Type, board.ProjectKey)
			}
			return table.render(out, boards)
		},
	}
	cmd.Flags().StringVar(&query.Name, "name", "", "match against the board name")
	cmd.Flags().StringVar(&query.ProjectKey, "project", "", "only boards belonging to this project")
	cmd.Flags().StringVar(&query.Type, "type", "", "scrum or kanban")
	cmd.Flags().IntVar(&query.MaxResults, "limit", 0, "stop after this many boards")
	out.register(cmd, true)
	return cmd
}

func newBoardSprintsCmd(deps Deps) *cobra.Command {
	var (
		out   format
		board string
		state string
	)
	cmd := &cobra.Command{
		Use:     "list-sprints",
		Short:   "List the sprints on a board",
		Example: "  acli-plus jira board list-sprints --board 42 --state active",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if board == "" {
				board = deps.defaults().Board
			}
			if board == "" {
				return fmt.Errorf("--board is required (an id, or a name to look up)")
			}
			service, err := deps.Service("")
			if err != nil {
				return err
			}
			sprints, err := service.ListSprints(cmd.Context(), board, state)
			if err != nil {
				return err
			}
			table := newRows("ID", "NAME", "STATE", "START", "END", "GOAL")
			for _, sprint := range sprints {
				table.add(itoa(sprint.ID), sprint.Name, sprint.State,
					shortDate(sprint.Start), shortDate(sprint.End), sprint.Goal)
			}
			return table.render(out, sprints)
		},
	}
	cmd.Flags().StringVar(&board, "board", "", "board id or name (required)")
	cmd.Flags().StringVar(&state, "state", "", "future, active, or closed")
	out.register(cmd, true)
	return cmd
}

func newSprintCmd(deps Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sprint",
		Short: "Look inside a sprint",
	}
	cmd.AddCommand(newSprintWorkItemsCmd(deps))
	return cmd
}

func newSprintWorkItemsCmd(deps Deps) *cobra.Command {
	var (
		out    format
		sprint string
		board  string
		jql    string
		fields []string
	)
	cmd := &cobra.Command{
		Use:   "list-workitems",
		Short: "List the work items in a sprint",
		Example: "  acli-plus jira sprint list-workitems --sprint 128\n" +
			"  acli-plus jira sprint list-workitems --sprint \"Sprint 7\" --board 42",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if sprint == "" {
				return fmt.Errorf("--sprint is required (an id, or a name together with --board)")
			}
			if board == "" {
				board = deps.defaults().Board
			}
			service, err := deps.Service("")
			if err != nil {
				return err
			}
			items, err := service.ListSprintWorkItems(cmd.Context(), sprint, board, jql, fields)
			if err != nil {
				return err
			}
			return renderWorkItems(items, out)
		},
	}
	cmd.Flags().StringVar(&sprint, "sprint", "", "sprint id, or a sprint name with --board (required)")
	cmd.Flags().StringVar(&board, "board", "", "board id or name, needed to look a sprint up by name")
	cmd.Flags().StringVar(&jql, "jql", "", "narrow the sprint's work items with JQL")
	cmd.Flags().StringSliceVar(&fields, "fields", nil, "fields to fetch, comma-separated")
	out.register(cmd, true)
	return cmd
}
