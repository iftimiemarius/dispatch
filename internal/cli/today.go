package cli

import (
	"fmt"
	"time"

	"github.com/iftimiemarius/dispatch/internal/models"
	"github.com/iftimiemarius/dispatch/internal/store"
	"github.com/iftimiemarius/dispatch/internal/ui"
	"github.com/spf13/cobra"
)

func newTodayCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "today",
		Short: "Show today's agenda: blocks, due tasks, and inbox triage",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st := MustStore(cmd)
			ctx := cmd.Context()
			out := cmd.OutOrStdout()
			now := time.Now().Local()

			fmt.Fprintf(out, "%s\n\n", ui.Bold(now.Format("Monday, January 2")))

			// 1. Today's blocks.
			start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
			blocks, _ := st.ListBlocks(ctx, &store.BlockRange{From: start, To: start.Add(24 * time.Hour)})
			fmt.Fprintln(out, ui.Section("Schedule"))
			if len(blocks) == 0 {
				fmt.Fprintln(out, ui.Dim("  no blocks today"))
			} else {
				for _, b := range blocks {
					marker := " "
					if b.TaskID != nil {
						marker = ui.Dim("●")
					}
					fmt.Fprintf(out, "  %s %s  %s\n",
						ui.Dim(b.StartsAt.Format("15:04")+"–"+b.EndsAt.Format("15:04")),
						marker, b.Title)
				}
			}

			// 2. Tasks due today or overdue.
			fmt.Fprintln(out)
			fmt.Fprintln(out, ui.Section("Due"))
			tasks, _ := st.ListTasks(ctx, store.TaskQuery{})
			due := filterDueToday(tasks, now)
			if len(due) == 0 {
				fmt.Fprintln(out, ui.Dim("  nothing due"))
			} else {
				rows := make([]ui.TaskRow, 0, len(due))
				for _, t := range due {
					rows = append(rows, ui.TaskRow{
						ID: t.ID, Priority: t.Priority, Status: t.Status,
						Title: t.Title, Tags: t.Tags, DueAt: t.DueAt,
					})
				}
				fmt.Fprintln(out, indentTable(ui.RenderTaskTable(rows)))
			}

			// 3. Inbox triage.
			fmt.Fprintln(out)
			fmt.Fprintln(out, ui.Section("Inbox"))
			inbox, _ := st.ListTasks(ctx, store.TaskQuery{Filter: store.TaskFilter{InboxOnly: true}})
			if len(inbox) == 0 {
				fmt.Fprintln(out, ui.Dim("  inbox empty"))
			} else {
				rows := make([]ui.TaskRow, 0, len(inbox))
				for _, t := range inbox {
					rows = append(rows, ui.TaskRow{
						ID: t.ID, Status: t.Status, Title: t.Title,
					})
				}
				fmt.Fprintln(out, indentTable(ui.RenderTaskTable(rows)))
			}
			return nil
		},
	}
}

// filterDueToday returns unfinished tasks due today or earlier.
func filterDueToday(tasks []*models.Task, now time.Time) []*models.Task {
	endOfToday := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location())
	var out []*models.Task
	for _, t := range tasks {
		if t.Status == models.StatusDone || t.Status == models.StatusCancelled {
			continue
		}
		if t.DueAt == nil || t.DueAt.After(endOfToday) {
			continue
		}
		out = append(out, t)
	}
	return out
}

// indentTable prefixes every line with two spaces for nested display.
func indentTable(s string) string {
	out := ""
	for i, line := range splitLines(s) {
		if i > 0 {
			out += "\n"
		}
		out += "  " + line
	}
	return out
}

func splitLines(s string) []string {
	var lines []string
	cur := ""
	for _, r := range s {
		if r == '\n' {
			lines = append(lines, cur)
			cur = ""
			continue
		}
		cur += string(r)
	}
	lines = append(lines, cur)
	return lines
}
