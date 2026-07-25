package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/iftimiemarius/dispatch/internal/models"
	"github.com/iftimiemarius/dispatch/internal/store"
	"github.com/iftimiemarius/dispatch/internal/ui"
	"github.com/spf13/cobra"
)

func newLsCmd() *cobra.Command {
	var (
		status   string
		priority string
		project  string
		tag      string
		inbox    bool
		all      bool // include done/cancelled
		group    string
	)
	cmd := &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list", "tasks"},
		Short:   "List tasks",
		Long: `List tasks, optionally filtered.

By default, finished tasks (done, cancelled) are hidden. Use --all to include
them. Common filters: --inbox, --status doing, --project api, -t bug.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			st := MustStore(cmd)
			ctx := cmd.Context()

			f := store.TaskFilter{
				InboxOnly: inbox,
				Tag:       tag,
			}
			if status != "" {
				s, err := parseStatus(status)
				if err != nil {
					return err
				}
				f.Status = &s
			}
			if priority != "" {
				p, err := parsePriority(priority)
				if err != nil {
					return err
				}
				f.Priority = &p
			}
			if project != "" {
				pid, err := resolveProjectID(ctx, st, project)
				if err != nil {
					return err
				}
				f.ProjectID = &pid
			}

			tasks, err := st.ListTasks(ctx, store.TaskQuery{Filter: f})
			if err != nil {
				return err
			}

			// Filter out finished tasks unless --all or explicitly requested.
			tasks = filterFinished(tasks, all, status)

			if len(tasks) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), ui.Dim("No tasks."))
				return nil
			}

			projectNames := loadProjectNames(ctx, st)

			if group == "project" {
				renderGroupedByProject(cmd.OutOrStdout(), tasks, projectNames)
				return nil
			}

			rows := make([]ui.TaskRow, 0, len(tasks))
			for _, t := range tasks {
				row := taskRow(t)
				if t.ProjectID != nil {
					if name, ok := projectNames[*t.ProjectID]; ok {
						row.Project = name
					}
				}
				rows = append(rows, row)
			}
			fmt.Fprintln(cmd.OutOrStdout(), ui.RenderTaskTable(rows))
			return nil
		},
	}
	cmd.Flags().StringVar(&status, "status", "", "filter by status")
	cmd.Flags().StringVarP(&priority, "priority", "P", "", "filter by priority")
	cmd.Flags().StringVarP(&project, "project", "p", "", "filter by project name or ID")
	cmd.Flags().StringVarP(&tag, "tag", "t", "", "filter by tag")
	cmd.Flags().BoolVar(&inbox, "inbox", false, "show only inbox tasks")
	cmd.Flags().BoolVar(&all, "all", false, "include done/cancelled tasks")
	cmd.Flags().StringVar(&group, "group", "", "group tasks by: project")
	return cmd
}

// renderGroupedByProject prints tasks grouped under project headers, with an
// "Inbox" bucket for unlinked tasks first.
func renderGroupedByProject(out interface{ Write([]byte) (int, error) }, tasks []*models.Task, projectNames map[string]string) {
	// bucket key = project id, "" for inbox
	buckets := map[string][]*models.Task{}
	order := []string{""} // inbox first
	for _, t := range tasks {
		key := ""
		if t.ProjectID != nil {
			key = *t.ProjectID
			if _, seen := buckets[key]; !seen {
				order = append(order, key)
			}
		}
		buckets[key] = append(buckets[key], t)
	}
	w := func(s string) { fmt.Fprintln(out, s) }
	for i, key := range order {
		if i > 0 {
			w("")
		}
		label := ui.Section("Inbox")
		if key != "" {
			name := projectNames[key]
			if name == "" {
				name = key
			}
			label = ui.Section("#" + name)
		}
		w(label)
		rows := make([]ui.TaskRow, 0, len(buckets[key]))
		for _, t := range buckets[key] {
			rows = append(rows, taskRow(t))
		}
		w(ui.RenderTaskTable(rows))
	}
}

// taskRow builds a ui.TaskRow from a task, including its GitHub issue badge.
func taskRow(t *models.Task) ui.TaskRow {
	row := ui.TaskRow{
		ID: t.ID, Priority: t.Priority, Status: t.Status,
		Title: t.Title, Tags: t.Tags, DueAt: t.DueAt,
	}
	if t.GitHubIssue != nil {
		n := *t.GitHubIssue
		row.GitHubIssue = &n
	}
	return row
}

// filterFinished removes done/cancelled unless --all was set or the user
// explicitly filtered for a finished status.
func filterFinished(tasks []*models.Task, all bool, statusFlag string) []*models.Task {
	if all || statusFlag != "" {
		return tasks
	}
	out := tasks[:0]
	for _, t := range tasks {
		if t.Status == models.StatusDone || t.Status == models.StatusCancelled {
			continue
		}
		out = append(out, t)
	}
	return out
}

// loadProjectNames returns a project-id -> name map for resolving list rows.
func loadProjectNames(ctx context.Context, st *store.Store) map[string]string {
	projects, err := st.ListProjects(ctx, nil)
	if err != nil {
		return nil
	}
	m := make(map[string]string, len(projects))
	for _, p := range projects {
		m[p.ID] = strings.ToLower(p.Name)
	}
	return m
}