package jira

import (
	"fmt"

	"github.com/spf13/cobra"

	jiradomain "acli-plus/internal/domain/jira"
)

func newFilterCmd(deps Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "filter",
		Short: "Find and manage saved JQL filters",
	}
	cmd.AddCommand(
		newFilterListCmd(deps),
		newFilterSearchCmd(deps),
		newFilterFavouriteCmd(deps),
		newFilterOwnerCmd(deps),
	)
	return cmd
}

// filterRows is the shared table shape for the two filter listings.
func filterRows(filters []jiradomain.Filter) *rows {
	table := newRows("ID", "NAME", "OWNER", "FAV", "JQL")
	for _, filter := range filters {
		table.add(filter.ID, filter.Name, filter.Owner.Name(), yesNo(filter.Favourite), filter.JQL)
	}
	return table
}

func newFilterListCmd(deps Deps) *cobra.Command {
	var (
		out            format
		favouritesOnly bool
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List your own and favourite filters",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			service, err := deps.Service("")
			if err != nil {
				return err
			}
			filters, err := service.ListMyFilters(cmd.Context(), favouritesOnly)
			if err != nil {
				return err
			}
			return filterRows(filters).render(out, filters)
		},
	}
	cmd.Flags().BoolVar(&favouritesOnly, "favourites", false, "only show filters you marked as favourites")
	out.register(cmd, true)
	return cmd
}

func newFilterSearchCmd(deps Deps) *cobra.Command {
	var (
		out   format
		query jiradomain.FilterQuery
		owner string
	)
	cmd := &cobra.Command{
		Use:     "search",
		Short:   "Search filters across the site",
		Example: "  acli-plus jira filter search --name sprint",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			service, err := deps.Service("")
			if err != nil {
				return err
			}
			if owner != "" {
				accountID, err := service.ResolveAccountID(cmd.Context(), owner)
				if err != nil {
					return err
				}
				query.AccountID = accountID
			}
			filters, err := service.SearchFilters(cmd.Context(), query)
			if err != nil {
				return err
			}
			return filterRows(filters).render(out, filters)
		},
	}
	cmd.Flags().StringVar(&query.Name, "name", "", "match against the filter name")
	cmd.Flags().StringVar(&owner, "owner", "", "only filters owned by this account (@me for yourself)")
	cmd.Flags().IntVar(&query.MaxResults, "limit", 0, "stop after this many filters")
	out.register(cmd, true)
	return cmd
}

func newFilterFavouriteCmd(deps Deps) *cobra.Command {
	var ids []string
	cmd := &cobra.Command{
		Use:     "add-favourite [filter...]",
		Short:   "Mark filters as favourites",
		Example: "  acli-plus jira filter add-favourite 10001\n  acli-plus jira filter add-favourite \"My open bugs\"",
		Args:    cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			selected := append(append([]string{}, args...), ids...)
			if len(selected) == 0 {
				return fmt.Errorf("pass a filter id or name")
			}
			service, err := deps.Service("")
			if err != nil {
				return err
			}
			result, err := service.AddFilterFavourite(cmd.Context(), selected, deps.Options())
			if err != nil {
				return err
			}
			printResult(result, "")
			return nil
		},
	}
	cmd.Flags().StringSliceVar(&ids, "id", nil, "filter id(s) or name(s), comma-separated")
	return cmd
}

func newFilterOwnerCmd(deps Deps) *cobra.Command {
	var (
		ids   []string
		owner string
	)
	cmd := &cobra.Command{
		Use:     "change-owner [filter...]",
		Short:   "Give filters to another account",
		Example: "  acli-plus jira filter change-owner 10001 --owner ann@acme.com",
		Args:    cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			selected := append(append([]string{}, args...), ids...)
			if len(selected) == 0 {
				return fmt.Errorf("pass a filter id or name")
			}
			if owner == "" {
				return fmt.Errorf("--owner is required")
			}
			service, err := deps.Service("")
			if err != nil {
				return err
			}
			result, err := service.ChangeFilterOwner(cmd.Context(), selected, owner, deps.Options())
			if err != nil {
				return err
			}
			printResult(result, "")
			return nil
		},
	}
	cmd.Flags().StringSliceVar(&ids, "id", nil, "filter id(s) or name(s), comma-separated")
	cmd.Flags().StringVar(&owner, "owner", "", "new owner by email, name, or account id")
	return cmd
}

func newDashboardCmd(deps Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dashboard",
		Short: "Find Jira dashboards",
	}
	cmd.AddCommand(newDashboardSearchCmd(deps))
	return cmd
}

func newDashboardSearchCmd(deps Deps) *cobra.Command {
	var (
		out   format
		query jiradomain.DashboardQuery
	)
	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search dashboards across the site",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			service, err := deps.Service("")
			if err != nil {
				return err
			}
			dashboards, err := service.SearchDashboards(cmd.Context(), query)
			if err != nil {
				return err
			}
			table := newRows("ID", "NAME", "OWNER", "FAV", "URL")
			for _, dashboard := range dashboards {
				table.add(dashboard.ID, dashboard.Name, dashboard.Owner.Name(),
					yesNo(dashboard.Favourite), dashboard.URL)
			}
			return table.render(out, dashboards)
		},
	}
	cmd.Flags().StringVar(&query.Name, "name", "", "match against the dashboard name")
	cmd.Flags().IntVar(&query.MaxResults, "limit", 0, "stop after this many dashboards")
	out.register(cmd, true)
	return cmd
}
