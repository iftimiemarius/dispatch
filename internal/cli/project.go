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

func newProjectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project",
		Short: "Manage projects (execution groupings)",
		Aliases: []string{"proj", "projects"},
	}
	cmd.AddCommand(
		newProjectAddCmd(),
		newProjectLsCmd(),
		newProjectShowCmd(),
		newProjectRmCmd(),
		newProjectEditCmd(),
	)
	return cmd
}

func newProjectAddCmd() *cobra.Command {
	var (
		desc       string
		color      string
		status     string
		initiative string
	)
	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Create a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st := MustStore(cmd)
			ctx := cmd.Context()
			now := time.Now().UTC()
			p := &models.Project{
				ID:        models.NewID(),
				Name:      args[0],
				Description: desc,
				Status:    "active",
				Color:     color,
				CreatedAt: now,
				UpdatedAt: now,
			}
			if status != "" {
				p.Status = status
			}
			if initiative != "" {
				iid, err := resolveInitiativeID(ctx, st, initiative)
				if err != nil {
					return err
				}
				p.InitiativeID = &iid
			}
			if err := st.CreateProject(ctx, p); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s project: %s\n", ui.Dim(p.ID), p.Name)
			return nil
		},
	}
	cmd.Flags().StringVarP(&desc, "description", "d", "", "short description")
	cmd.Flags().StringVar(&color, "color", "", "color label")
	cmd.Flags().StringVar(&status, "status", "active", "status: active|on_hold|done|archived")
	cmd.Flags().StringVarP(&initiative, "initiative", "i", "", "parent initiative name or ID")
	return cmd
}

func newProjectLsCmd() *cobra.Command {
	var initiative string
	cmd := &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Short:   "List projects",
		RunE: func(cmd *cobra.Command, args []string) error {
			st := MustStore(cmd)
			ctx := cmd.Context()
			var iid *string
			if initiative != "" {
				id, err := resolveInitiativeID(ctx, st, initiative)
				if err != nil {
					return err
				}
				iid = &id
			}
			projects, err := st.ListProjects(ctx, iid)
			if err != nil {
				return err
			}
			if len(projects) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), ui.Dim("No projects."))
				return nil
			}
			// Task counts per project.
			counts := projectTaskCounts(ctx, st)
			for _, p := range projects {
				open := counts[p.ID]
				name := ui.Bold(p.Name)
				if p.Color != "" {
					name += " " + ui.Dim("("+p.Color+")")
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-8s  %-8s  %s  %s\n",
					ui.Dim(p.ID), ui.Dim(p.Status), name, ui.Dim(taskCountLabel(open)))
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&initiative, "initiative", "i", "", "filter by initiative")
	return cmd
}

func newProjectShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <name|id>",
		Short: "Show a project and its tasks",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st := MustStore(cmd)
			ctx := cmd.Context()
			p, err := findProject(ctx, st, args[0])
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			fmt.Fprintln(out, ui.Bold(p.Name))
			fmt.Fprintf(out, "%s  %s\n", ui.Dim(p.ID), ui.Dim(p.Status))
			if p.Description != "" {
				fmt.Fprintln(out, p.Description)
			}
			if p.InitiativeID != nil {
				fmt.Fprintf(out, "%s%s\n", ui.Dim("initiative: "), *p.InitiativeID)
			}
			fmt.Fprintln(out)
			fmt.Fprintln(out, ui.Section("Tasks"))
			pid := p.ID
			tasks, err := st.ListTasks(ctx, store.TaskQuery{Filter: store.TaskFilter{ProjectID: &pid}})
			if err != nil {
				return err
			}
			if len(tasks) == 0 {
				fmt.Fprintln(out, ui.Dim("No tasks."))
				return nil
			}
			rows := make([]ui.TaskRow, 0, len(tasks))
			for _, t := range tasks {
				rows = append(rows, ui.TaskRow{
					ID: t.ID, Priority: t.Priority, Status: t.Status,
					Title: t.Title, Tags: t.Tags, DueAt: t.DueAt, Project: p.Name,
				})
			}
			fmt.Fprintln(out, ui.RenderTaskTable(rows))
			return nil
		},
	}
	return cmd
}

func newProjectRmCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "rm <name|id>",
		Aliases: []string{"delete"},
		Short:   "Delete a project (tasks keep existing, unlinked)",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st := MustStore(cmd)
			ctx := cmd.Context()
			p, err := findProject(ctx, st, args[0])
			if err != nil {
				return err
			}
			if err := st.DeleteProject(ctx, p.ID); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s %s: %s\n", ui.Dim(p.ID), "removed", p.Name)
			return nil
		},
	}
	return cmd
}

func newProjectEditCmd() *cobra.Command {
	var (
		name       string
		desc       string
		status     string
		initiative string
	)
	cmd := &cobra.Command{
		Use:   "edit <name|id>",
		Short: "Edit a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st := MustStore(cmd)
			ctx := cmd.Context()
			p, err := findProject(ctx, st, args[0])
			if err != nil {
				return err
			}
			if cmd.Flags().Changed("name") {
				p.Name = name
			}
			if cmd.Flags().Changed("description") {
				p.Description = desc
			}
			if cmd.Flags().Changed("status") {
				p.Status = status
			}
			if cmd.Flags().Changed("initiative") {
				if initiative == "" {
					p.InitiativeID = nil
				} else {
					iid, err := resolveInitiativeID(ctx, st, initiative)
					if err != nil {
						return err
					}
					p.InitiativeID = &iid
				}
			}
			p.UpdatedAt = time.Now().UTC()
			if err := st.UpdateProject(ctx, p); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s updated: %s\n", ui.Dim(p.ID), p.Name)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "rename the project")
	cmd.Flags().StringVarP(&desc, "description", "d", "", "description")
	cmd.Flags().StringVar(&status, "status", "", "status: active|on_hold|done|archived")
	cmd.Flags().StringVarP(&initiative, "initiative", "i", "", "parent initiative (use \"\" to unlink)")
	return cmd
}

// findProject resolves a name, full ID, or short ID to a project.
func findProject(ctx context.Context, st *store.Store, ref string) (*models.Project, error) {
	return resolveProjectRef(ctx, st, ref)
}

// projectTaskCounts returns project-id -> count of non-finished tasks.
func projectTaskCounts(ctx context.Context, st *store.Store) map[string]int {
	tasks, err := st.ListTasks(ctx, store.TaskQuery{})
	if err != nil {
		return nil
	}
	m := make(map[string]int)
	for _, t := range tasks {
		if t.ProjectID == nil || t.Status == models.StatusDone || t.Status == models.StatusCancelled {
			continue
		}
		m[*t.ProjectID]++
	}
	return m
}

func taskCountLabel(n int) string {
	if n == 0 {
		return "no open tasks"
	}
	return fmt.Sprintf("%d open", n)
}