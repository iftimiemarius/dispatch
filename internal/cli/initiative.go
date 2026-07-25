package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/iftimiemarius/dispatch/internal/models"
	"github.com/iftimiemarius/dispatch/internal/store"
	"github.com/iftimiemarius/dispatch/internal/ui"
	"github.com/spf13/cobra"
)

// newInitiativeCmd uses "init" as the primary command name (short for
// initiative) to avoid clashing with cobra's reserved init() semantics at the
// user level. Aliases include "initiative" and "initiatives".
func newInitiativeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "init",
		Aliases: []string{"initiative", "initiatives"},
		Short:   "Manage initiatives (strategic outcomes)",
	}
	cmd.AddCommand(
		newInitAddCmd(),
		newInitLsCmd(),
		newInitShowCmd(),
		newInitRmCmd(),
		newInitEditCmd(),
	)
	return cmd
}

func newInitAddCmd() *cobra.Command {
	var (
		outcome string
		status  string
		start   string
		target  string
	)
	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Create an initiative",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st := MustStore(cmd)
			ctx := cmd.Context()
			now := time.Now().UTC()
			i := &models.Initiative{
				ID:        models.NewID(),
				Name:      args[0],
				Outcome:   outcome,
				Status:    "active",
				CreatedAt: now,
				UpdatedAt: now,
			}
			if status != "" {
				i.Status = status
			}
			if start != "" {
				s, err := parseDue(start, now)
				if err != nil {
					return err
				}
				i.StartAt = &s
			}
			if target != "" {
				t, err := parseDue(target, now)
				if err != nil {
					return err
				}
				i.TargetAt = &t
			}
			if err := st.CreateInitiative(ctx, i); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s initiative: %s\n", ui.Dim(i.ID), i.Name)
			return nil
		},
	}
	cmd.Flags().StringVarP(&outcome, "outcome", "o", "", "the outcome this initiative drives")
	cmd.Flags().StringVar(&status, "status", "active", "status: active|on_hold|done|cancelled")
	cmd.Flags().StringVar(&start, "start", "", "start date (e.g. 2025-06-01, -7d)")
	cmd.Flags().StringVar(&target, "target", "", "target completion date")
	return cmd
}

func newInitLsCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Short:   "List initiatives",
		RunE: func(cmd *cobra.Command, args []string) error {
			st := MustStore(cmd)
			ctx := cmd.Context()
			inits, err := st.ListInitiatives(ctx)
			if err != nil {
				return err
			}
			if len(inits) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), ui.Dim("No initiatives."))
				return nil
			}
			for _, i := range inits {
				name := ui.Bold(i.Name)
				if i.Outcome != "" {
					name += " " + ui.Dim("— "+i.Outcome)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-8s  %-8s  %s", ui.Dim(i.ID), ui.Dim(i.Status), name)
				if i.TargetAt != nil {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s", ui.Dim("by "+i.TargetAt.Format("Jan 2")))
				}
				fmt.Fprintln(cmd.OutOrStdout())
			}
			return nil
		},
	}
}

func newInitShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <name|id>",
		Short: "Show an initiative, its projects and tasks",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st := MustStore(cmd)
			ctx := cmd.Context()
			i, err := findInitiative(ctx, st, args[0])
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			fmt.Fprintln(out, ui.Bold(i.Name))
			fmt.Fprintf(out, "%s  %s\n", ui.Dim(i.ID), ui.Dim(i.Status))
			if i.Outcome != "" {
				fmt.Fprintf(out, "%s%s\n", ui.Dim("outcome: "), i.Outcome)
			}
			if i.StartAt != nil && i.TargetAt != nil {
				fmt.Fprintf(out, "%s%s → %s\n", ui.Dim("window: "),
					i.StartAt.Format("Jan 2"), i.TargetAt.Format("Jan 2"))
			}
			iid := i.ID

			// Projects under this initiative.
			projects, _ := st.ListProjects(ctx, &iid)
			if len(projects) > 0 {
				fmt.Fprintln(out)
				fmt.Fprintln(out, ui.Section("Projects"))
				for _, p := range projects {
					fmt.Fprintf(out, "  %s\n", p.Name)
				}
			}

			// Tasks directly linked to this initiative (and optionally a project).
			fmt.Fprintln(out)
			fmt.Fprintln(out, ui.Section("Tasks"))
			tasks, err := st.ListTasks(ctx, store.TaskQuery{Filter: store.TaskFilter{InitiativeID: &iid}})
			if err != nil {
				return err
			}
			if len(tasks) == 0 {
				fmt.Fprintln(out, ui.Dim("No tasks."))
				return nil
			}
			rows := make([]ui.TaskRow, 0, len(tasks))
			for _, t := range tasks {
				rows = append(rows, taskRow(t))
			}
			fmt.Fprintln(out, ui.RenderTaskTable(rows))
			return nil
		},
	}
}

func newInitRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "rm <name|id>",
		Aliases: []string{"delete"},
		Short:   "Delete an initiative",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st := MustStore(cmd)
			ctx := cmd.Context()
			i, err := findInitiative(ctx, st, args[0])
			if err != nil {
				return err
			}
			if err := st.DeleteInitiative(ctx, i.ID); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s %s: %s\n", ui.Dim(i.ID), "removed", i.Name)
			return nil
		},
	}
}

func newInitEditCmd() *cobra.Command {
	var (
		name    string
		outcome string
		status  string
		target  string
	)
	cmd := &cobra.Command{
		Use:   "edit <name|id>",
		Short: "Edit an initiative",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st := MustStore(cmd)
			ctx := cmd.Context()
			i, err := findInitiative(ctx, st, args[0])
			if err != nil {
				return err
			}
			if cmd.Flags().Changed("name") {
				i.Name = name
			}
			if cmd.Flags().Changed("outcome") {
				i.Outcome = outcome
			}
			if cmd.Flags().Changed("status") {
				i.Status = status
			}
			if cmd.Flags().Changed("target") {
				if target == "" {
					i.TargetAt = nil
				} else {
					t, err := parseDue(target, time.Now().UTC())
					if err != nil {
						return err
					}
					i.TargetAt = &t
				}
			}
			i.UpdatedAt = time.Now().UTC()
			if err := st.UpdateInitiative(ctx, i); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s updated: %s\n", ui.Dim(i.ID), i.Name)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "rename")
	cmd.Flags().StringVarP(&outcome, "outcome", "o", "", "the outcome")
	cmd.Flags().StringVar(&status, "status", "", "status: active|on_hold|done|cancelled")
	cmd.Flags().StringVar(&target, "target", "", "target completion date (use \"\" to clear)")
	return cmd
}

// findInitiative resolves an initiative name or ID.
func findInitiative(ctx context.Context, st *store.Store, ref string) (*models.Initiative, error) {
	if i, err := st.GetInitiative(ctx, ref); err == nil {
		return i, nil
	}
	list, err := st.ListInitiatives(ctx)
	if err != nil {
		return nil, err
	}
	for _, i := range list {
		if i.Name == ref {
			return i, nil
		}
	}
	return nil, fmt.Errorf("initiative %q not found", ref)
}
