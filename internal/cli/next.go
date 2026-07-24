package cli

import (
	"fmt"
	"time"

	"github.com/iftimiemarius/dispatch/internal/models"
	"github.com/iftimiemarius/dispatch/internal/store"
	"github.com/iftimiemarius/dispatch/internal/ui"
	"github.com/spf13/cobra"
)

// newNextCmd implements `dispatch next` (alias `focus`): it picks the single
// highest-leverage task to work on right now and (optionally) marks it doing.
//
// Selection order:
//  1. Any task already "doing" (resume in-progress work).
//  2. Otherwise the highest-priority "todo" task, breaking ties by due date
//     then recency.
func newNextCmd() *cobra.Command {
	var start bool
	cmd := &cobra.Command{
		Use:     "next",
		Aliases: []string{"focus"},
		Short:   "Show the task to focus on right now",
		Long: `Show the single task to focus on right now.

Resumes any in-progress task first; otherwise picks the highest-priority todo
task. Use --start to mark the suggestion as in-progress.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st := MustStore(cmd)
			ctx := cmd.Context()
			out := cmd.OutOrStdout()

			tasks, err := st.ListTasks(ctx, store.TaskQuery{})
			if err != nil {
				return err
			}

			pick := pickNext(tasks)
			if pick == nil {
				fmt.Fprintln(out, ui.Dim("Nothing actionable. Inbox clear, or add a task with `dispatch add`."))
				return nil
			}

			fmt.Fprintln(out, ui.Section("Focus"))
			fmt.Fprintf(out, "  %s  %s  %s\n",
				ui.Dim(pick.ID),
				priorityLabel(pick.Priority),
				ui.Bold(pick.Title),
			)
			if pick.ProjectID != nil || pick.DueAt != nil {
				var bits []string
				if pick.ProjectID != nil {
					bits = append(bits, "project "+*pick.ProjectID)
				}
				if pick.DueAt != nil {
					bits = append(bits, "due "+ui.RelativeDate(*pick.DueAt))
				}
				fmt.Fprintf(out, "  %s\n", ui.Dim(join(bits, " · ")))
			}

			if start {
				if pick.Status == models.StatusDoing {
					fmt.Fprintln(out, ui.Dim("  (already in-progress)"))
				} else {
					pick.Status = models.StatusDoing
					pick.UpdatedAt = time.Now().UTC()
					if err := st.UpdateTask(ctx, pick); err != nil {
						return err
					}
					fmt.Fprintln(out, ui.Dim("  marked in-progress"))
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&start, "start", false, "mark the suggested task as in-progress")
	return cmd
}

// pickNext applies the selection heuristic to a task list.
func pickNext(tasks []*models.Task) *models.Task {
	// 1. Prefer a task already doing (resume in-progress work).
	for _, t := range tasks {
		if t.Status == models.StatusDoing {
			return t
		}
	}
	// 2. Highest-priority actionable task (todo). If none are triaged to todo,
	//    fall back to inbox tasks so capture still flows into focus.
	best := bestActionable(tasks, models.StatusTodo)
	if best != nil {
		return best
	}
	return bestActionable(tasks, models.StatusInbox)
}

// bestActionable returns the highest-priority task matching status, tie-break
// by due date (earlier first).
func bestActionable(tasks []*models.Task, status models.TaskStatus) *models.Task {
	var best *models.Task
	for _, t := range tasks {
		if t.Status != status {
			continue
		}
		if best == nil {
			best = t
			continue
		}
		if rankPriority(t.Priority) < rankPriority(best.Priority) {
			best = t
			continue
		}
		if rankPriority(t.Priority) == rankPriority(best.Priority) {
			if earlierDue(t.DueAt, best.DueAt) {
				best = t
			}
		}
	}
	return best
}

func rankPriority(p models.Priority) int {
	switch p {
	case models.PriorityUrgent:
		return 0
	case models.PriorityHigh:
		return 1
	case models.PriorityMedium:
		return 2
	case models.PriorityLow:
		return 3
	}
	return 4
}

// earlierDue reports whether a is an earlier due date than b, treating nil as
// "no due" (i.e., later than any concrete due).
func earlierDue(a, b *time.Time) bool {
	if a == nil {
		return false
	}
	if b == nil {
		return true
	}
	return a.Before(*b)
}

func priorityLabel(p models.Priority) string {
	return ui.Header(string(p))
}

func join(parts []string, sep string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += sep
		}
		out += p
	}
	return out
}
