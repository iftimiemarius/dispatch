package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/iftimiemarius/dispatch/internal/models"
	"github.com/iftimiemarius/dispatch/internal/ui"
	"github.com/spf13/cobra"
)

func newTaskCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "task",
		Short: "Show or act on a single task",
	}
	cmd.AddCommand(
		newTaskShowCmd(),
		newTaskDoneCmd(),
		newTaskEditCmd(),
		newTaskRmCmd(),
		newTaskMvCmd(),
		newTaskStartCmd(),
	)
	return cmd
}

// dispatch task <id>   (also: dispatch show <id>)
func newTaskShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "show <id>",
		Aliases: []string{"info"},
		Short:   "Show details of a task",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st := MustStore(cmd)
			ctx := cmd.Context()
			t, err := resolveTask(ctx, st, args[0])
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			fmt.Fprintln(out, ui.Bold(t.Title))
			fmt.Fprintf(out, "%s  %s  %s\n",
				ui.Dim(t.ID),
				ui.Dim("status:")+string(t.Status),
				ui.Dim("priority:")+string(t.Priority),
			)
			if t.ProjectID != nil {
				fmt.Fprintf(out, "%s%s\n", ui.Dim("project: "), *t.ProjectID)
			}
			if t.InitiativeID != nil {
				fmt.Fprintf(out, "%s%s\n", ui.Dim("initiative: "), *t.InitiativeID)
			}
			if len(t.Tags) > 0 {
				fmt.Fprintf(out, "%s%s\n", ui.Dim("tags: "), strings.Join(t.Tags, ", "))
			}
			if t.DueAt != nil {
				fmt.Fprintf(out, "%s%s\n", ui.Dim("due: "), t.DueAt.Local().Format("Mon Jan 2 15:04"))
			}
			if t.Notes != "" {
				fmt.Fprintln(out)
				fmt.Fprintln(out, t.Notes)
			}
			return nil
		},
	}
}

// dispatch done <id> [id...]
func newTaskDoneCmd() *cobra.Command {
	var undo bool
	cmd := &cobra.Command{
		Use:   "done <id> [id...]",
		Short: "Mark a task as done",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st := MustStore(cmd)
			ctx := cmd.Context()
			now := time.Now().UTC()
			for _, id := range args {
				t, err := resolveTask(ctx, st, id)
				if err != nil {
					return err
				}
				if undo {
					t.Status = models.StatusTodo
					t.CompletedAt = nil
				} else {
					t.Status = models.StatusDone
					t.CompletedAt = &now
				}
				t.UpdatedAt = now
				if err := st.UpdateTask(ctx, t); err != nil {
					return err
				}
				verb := "done"
				if undo {
					verb = "reopened"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s %s: %s\n", ui.Dim(id), verb, t.Title)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&undo, "undo", false, "reopen the task (set back to todo)")
	return cmd
}

// dispatch start <id>  — mark in-progress.
func newTaskStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start <id>",
		Short: "Mark a task as in-progress (doing)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return setStatus(cmd, args[0], models.StatusDoing)
		},
	}
}

func setStatus(cmd *cobra.Command, id string, status models.TaskStatus) error {
	st := MustStore(cmd)
	ctx := cmd.Context()
	t, err := resolveTask(ctx, st, id)
	if err != nil {
		return err
	}
	t.Status = status
	t.UpdatedAt = time.Now().UTC()
	if err := st.UpdateTask(ctx, t); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s → %s\n", ui.Dim(id), status)
	return nil
}

// dispatch edit <id> [--title ...] [-P ...] [--status ...] ...
func newTaskEditCmd() *cobra.Command {
	var flags taskFlags
	var title string
	cmd := &cobra.Command{
		Use:   "edit <id>",
		Short: "Edit a task's fields",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st := MustStore(cmd)
			ctx := cmd.Context()
			t, err := resolveTask(ctx, st, args[0])
			if err != nil {
				return err
			}
			now := time.Now().UTC()

			if cmd.Flags().Changed("title") {
				t.Title = title
			}
			if cmd.Flags().Changed("priority") {
				p, err := parsePriority(flags.priority)
				if err != nil {
					return err
				}
				t.Priority = p
			}
			if cmd.Flags().Changed("status") {
				s, err := parseStatus(flags.status)
				if err != nil {
					return err
				}
				t.Status = s
				if s == models.StatusDone && t.CompletedAt == nil {
					c := now
					t.CompletedAt = &c
				}
			}
			if cmd.Flags().Changed("tag") {
				t.Tags = flags.tags
			}
			if cmd.Flags().Changed("notes") {
				t.Notes = flags.notes
			}
			if cmd.Flags().Changed("project") {
				if flags.project == "" {
					t.ProjectID = nil
				} else {
					pid, err := resolveProjectID(ctx, st, flags.project)
					if err != nil {
						return err
					}
					t.ProjectID = &pid
				}
			}
			if cmd.Flags().Changed("due") {
				if flags.due == "" {
					t.DueAt = nil
				} else {
					due, err := parseDue(flags.due, now)
					if err != nil {
						return err
					}
					t.DueAt = &due
				}
			}
			t.UpdatedAt = now
			if err := st.UpdateTask(ctx, t); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s updated: %s\n", ui.Dim(t.ID), t.Title)
			return nil
		},
	}
	flags.addFlags(cmd)
	cmd.Flags().StringVar(&title, "title", "", "new title")
	return cmd
}

// dispatch rm <id> [id...]
func newTaskRmCmd() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:     "rm <id> [id...]",
		Aliases: []string{"delete", "remove"},
		Short:   "Delete a task",
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st := MustStore(cmd)
			ctx := cmd.Context()
			for _, id := range args {
				t, err := resolveTask(ctx, st, id)
				if err != nil {
					return err
				}
				if err := st.DeleteTask(ctx, id); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s %s: %s\n", ui.Dim(id), "removed", t.Title)
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip confirmation (reserved)")
	return cmd
}

// dispatch mv <id> -p <project>  (move/assign a task)
func newTaskMvCmd() *cobra.Command {
	var (
		project    string
		initiative string
		fromProj   bool
	)
	cmd := &cobra.Command{
		Use:   "mv <id>",
		Short: "Move a task to a project or initiative",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st := MustStore(cmd)
			ctx := cmd.Context()
			t, err := resolveTask(ctx, st, args[0])
			if err != nil {
				return err
			}
			if cmd.Flags().Changed("project") {
				if project == "" {
					t.ProjectID = nil
				} else {
					pid, err := resolveProjectID(ctx, st, project)
					if err != nil {
						return err
					}
					t.ProjectID = &pid
				}
			}
			if cmd.Flags().Changed("initiative") {
				if initiative == "" {
					t.InitiativeID = nil
				} else {
					iid, err := resolveInitiativeID(ctx, st, initiative)
					if err != nil {
						return err
					}
					t.InitiativeID = &iid
				}
			}
			t.UpdatedAt = time.Now().UTC()
			if err := st.UpdateTask(ctx, t); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s moved\n", ui.Dim(t.ID))
			return nil
		},
	}
	cmd.Flags().StringVarP(&project, "project", "p", "", "target project name or ID (use \"\" to unlink)")
	cmd.Flags().StringVarP(&initiative, "initiative", "i", "", "target initiative name or ID")
	cmd.Flags().BoolVar(&fromProj, "from-project", false, "remove project link (alias for -p \"\")")
	return cmd
}
