package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/iftimiemarius/dispatch/internal/models"
	"github.com/iftimiemarius/dispatch/internal/ui"
	"github.com/spf13/cobra"
)

// flag bundle shared by add + edit for task fields.
type taskFlags struct {
	priority string
	status   string
	project  string // name or id
	tags     []string
	due      string
	notes    string
	repo     string // GitHub repo override (owner/name)
	issue    int    // GitHub issue/PR number
}

func (f *taskFlags) addFlags(c *cobra.Command) {
	c.Flags().StringVarP(&f.priority, "priority", "P", "", "priority: low|medium|high|urgent")
	c.Flags().StringVar(&f.status, "status", "", "status: inbox|todo|doing|done|blocked|cancelled")
	c.Flags().StringVarP(&f.project, "project", "p", "", "project name or ID")
	c.Flags().StringSliceVarP(&f.tags, "tag", "t", nil, "tags (repeatable or comma-separated)")
	c.Flags().StringVar(&f.due, "due", "", "due date/time, e.g. 'tomorrow', 'fri 9am', '2025-12-01', '+3d'")
	c.Flags().StringVarP(&f.notes, "notes", "n", "", "longer notes")
	c.Flags().StringVar(&f.repo, "repo", "", "GitHub repo owner/name (overrides project default)")
	c.Flags().IntVar(&f.issue, "issue", 0, "GitHub issue/PR number to link")
}

func newAddCmd() *cobra.Command {
	var flags taskFlags
	cmd := &cobra.Command{
		Use:     "add [title]",
		Aliases: []string{"+", "capture"},
		Short:   "Capture a new task into the inbox",
		Long: `Capture a new task.

The title may be passed as a single quoted argument or built from multiple
positional words. A freshly captured task lands in the inbox unless you set
a status or project explicitly.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st := MustStore(cmd)
			ctx := cmd.Context()

			title := strings.Join(args, " ")
			now := time.Now().UTC()

			t := &models.Task{
				ID:        models.NewID(),
				Title:     title,
				Notes:     flags.notes,
				Status:    models.StatusInbox,
				Priority:  models.PriorityMedium,
				CreatedAt: now,
				UpdatedAt: now,
			}
			if flags.status != "" {
				s, err := parseStatus(flags.status)
				if err != nil {
					return err
				}
				t.Status = s
			}
			if flags.priority != "" {
				p, err := parsePriority(flags.priority)
				if err != nil {
					return err
				}
				t.Priority = p
			}
			t.Tags = flags.tags

			if flags.project != "" {
				pid, err := resolveProjectID(ctx, st, flags.project)
				if err != nil {
					return err
				}
				t.ProjectID = &pid
			}
			if flags.due != "" {
				due, err := parseDue(flags.due, now)
				if err != nil {
					return err
				}
				t.DueAt = &due
			}
			if flags.repo != "" {
				r := flags.repo
				t.GitHubRepo = &r
			}
			if flags.issue != 0 {
				n := flags.issue
				t.GitHubIssue = &n
			}

			if err := st.CreateTask(ctx, t); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s captured: %s\n", ui.Dim(t.ID), t.Title)
			return nil
		},
	}
	flags.addFlags(cmd)
	return cmd
}

func parseStatus(s string) (models.TaskStatus, error) {
	switch strings.ToLower(s) {
	case "inbox":
		return models.StatusInbox, nil
	case "todo", "next", "pending":
		return models.StatusTodo, nil
	case "doing", "in-progress", "wip", "started":
		return models.StatusDoing, nil
	case "done", "complete", "completed":
		return models.StatusDone, nil
	case "blocked", "waiting":
		return models.StatusBlocked, nil
	case "cancelled", "canceled", "skipped":
		return models.StatusCancelled, nil
	}
	return "", fmt.Errorf("invalid status %q (want inbox|todo|doing|done|blocked|cancelled)", s)
}

func parsePriority(s string) (models.Priority, error) {
	switch strings.ToLower(s) {
	case "low", "l":
		return models.PriorityLow, nil
	case "medium", "med", "m", "":
		return models.PriorityMedium, nil
	case "high", "h":
		return models.PriorityHigh, nil
	case "urgent", "u", "critical":
		return models.PriorityUrgent, nil
	}
	return "", fmt.Errorf("invalid priority %q (want low|medium|high|urgent)", s)
}
