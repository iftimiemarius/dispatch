package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/iftimiemarius/dispatch/internal/github"
	"github.com/iftimiemarius/dispatch/internal/store"
	"github.com/iftimiemarius/dispatch/internal/ui"
	"github.com/spf13/cobra"
)

func newGhCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gh",
		Short: "GitHub link & view (uses your gh CLI auth)",
		Long: `Link tasks to GitHub issues/PRs and view them.

Uses your existing gh CLI for authentication — run 'gh auth login' once.
The repo is resolved from the task's project default, a per-task override, or
--repo. References accept #42, 42, owner/name#42, or a full URL.`,
	}
	cmd.AddCommand(
		newGhLinkCmd(),
		newGhShowCmd(),
		newGhUnlinkCmd(),
		newGhPRsCmd(),
		newGhIssuesCmd(),
	)
	return cmd
}

// ghAvailable wraps a check that prints a friendly hint on failure.
func ghAvailable(ctx context.Context) (*github.Client, error) {
	c := github.NewClient()
	if err := c.Available(ctx); err != nil {
		return nil, fmt.Errorf("%w\nInstall gh from https://cli.github.com and run `gh auth login`", err)
	}
	return c, nil
}

// resolveGhRepo determines the effective GitHub repo for a task: the task's own
// override, else its project's default. Returns "" if none is set.
func resolveGhRepo(ctx context.Context, st *store.Store, taskRef, override string) (string, error) {
	if override != "" {
		return github.NormalizeRepoArg(override), nil
	}
	t, err := resolveTask(ctx, st, taskRef)
	if err != nil {
		return "", err
	}
	if t.GitHubRepo != nil && *t.GitHubRepo != "" {
		return *t.GitHubRepo, nil
	}
	if t.ProjectID != nil {
		if p, err := st.GetProject(ctx, *t.ProjectID); err == nil && p.GitHubRepo != nil {
			return *p.GitHubRepo, nil
		}
	}
	return "", nil
}

func newGhLinkCmd() *cobra.Command {
	var repoFlag string
	cmd := &cobra.Command{
		Use:   "link <task-id> <ref>",
		Short: "Link a task to a GitHub issue/PR",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			st := MustStore(cmd)
			c, err := ghAvailable(ctx)
			if err != nil {
				return err
			}
			taskRef, refArg := args[0], args[1]
			ref, err := github.ParseRef(refArg)
			if err != nil {
				return err
			}
			// Resolve repo: ref's own repo → --repo → task/project default.
			repo := ref.Repo
			if repo == "" {
				repo, err = resolveGhRepo(ctx, st, taskRef, repoFlag)
				if err != nil {
					return err
				}
			}
			if repo == "" {
				return fmt.Errorf("no repo: provide --repo owner/name or set a project default (dispatch project edit <p> --github-repo owner/name)")
			}

			t, err := resolveTask(ctx, st, taskRef)
			if err != nil {
				return err
			}
			// Verify the issue exists (and is reachable) before linking.
			issue, err := c.GetIssue(ctx, repo, ref.Number)
			if err != nil {
				return fmt.Errorf("fetch %s#%d: %w", repo, ref.Number, err)
			}
			r := repo
			n := ref.Number
			t.GitHubRepo = &r
			t.GitHubIssue = &n
			t.UpdatedAt = time.Now().UTC()
			if err := st.UpdateTask(ctx, t); err != nil {
				return err
			}
			kind := "issue"
			if issue.IsPR {
				kind = "PR"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s linked to %s %s#%d: %s\n",
				ui.Dim(t.ID[len(t.ID)-6:]), kind, repo, ref.Number, issue.Title)
			return nil
		},
	}
	cmd.Flags().StringVar(&repoFlag, "repo", "", "GitHub repo owner/name (overrides task/project default)")
	return cmd
}

func newGhShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <task-id>",
		Short: "Show the GitHub issue/PR linked to a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			st := MustStore(cmd)
			c, err := ghAvailable(ctx)
			if err != nil {
				return err
			}
			t, err := resolveTask(ctx, st, args[0])
			if err != nil {
				return err
			}
			if t.GitHubRepo == nil || t.GitHubIssue == nil {
				return fmt.Errorf("task has no GitHub link (use `dispatch gh link %s <ref>`)", t.ID[len(t.ID)-6:])
			}
			issue, err := c.GetIssue(ctx, *t.GitHubRepo, *t.GitHubIssue)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			kind := "Issue"
			if issue.IsPR {
				kind = "PR"
			}
			fmt.Fprintf(out, "%s %s#%d\n", kind, *t.GitHubRepo, issue.Number)
			fmt.Fprintf(out, "  %s\n", ui.Bold(issue.Title))
			stateStyle := ui.Dim
			if issue.State == "open" {
				stateStyle = ui.Green
			}
			fmt.Fprintf(out, "  state: %s  %s\n", stateStyle(issue.State), ui.Dim(issue.HTMLURL))
			return nil
		},
	}
	return cmd
}

func newGhUnlinkCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unlink <task-id>",
		Short: "Remove the GitHub link from a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			st := MustStore(cmd)
			t, err := resolveTask(ctx, st, args[0])
			if err != nil {
				return err
			}
			t.GitHubRepo = nil
			t.GitHubIssue = nil
			t.UpdatedAt = time.Now().UTC()
			if err := st.UpdateTask(ctx, t); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s unlinked\n", ui.Dim(t.ID[len(t.ID)-6:]))
			return nil
		},
	}
	return cmd
}

func newGhPRsCmd() *cobra.Command {
	var repoFlag string
	cmd := &cobra.Command{
		Use:     "prs",
		Aliases: []string{"pr"},
		Short:   "List open pull requests",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			c, err := ghAvailable(ctx)
			if err != nil {
				return err
			}
			if repoFlag == "" {
				return fmt.Errorf("--repo owner/name is required (e.g. dispatch gh prs --repo owner/name)")
			}
			repo := github.NormalizeRepoArg(repoFlag)
			prs, err := c.ListOpenPRs(ctx, repo)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if len(prs) == 0 {
				fmt.Fprintln(out, ui.Dim("No open PRs."))
				return nil
			}
			for _, pr := range prs {
				marker := " "
				if pr.Draft {
					marker = ui.Dim("d")
				}
				fmt.Fprintf(out, "%s #%d  %s  %s\n", marker, pr.Number, pr.Title, ui.Dim(pr.HTMLURL))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&repoFlag, "repo", "", "GitHub repo owner/name")
	return cmd
}

func newGhIssuesCmd() *cobra.Command {
	var repoFlag string
	cmd := &cobra.Command{
		Use:     "issues",
		Aliases: []string{"issue"},
		Short:   "List open issues",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			c, err := ghAvailable(ctx)
			if err != nil {
				return err
			}
			if repoFlag == "" {
				return fmt.Errorf("--repo owner/name is required")
			}
			repo := github.NormalizeRepoArg(repoFlag)
			issues, err := c.ListOpenIssues(ctx, repo)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if len(issues) == 0 {
				fmt.Fprintln(out, ui.Dim("No open issues."))
				return nil
			}
			for _, is := range issues {
				kind := "#"
				if is.IsPR {
					kind = ui.Dim("PR")
				}
				fmt.Fprintf(out, "%s #%d  %s\n", kind, is.Number, is.Title)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&repoFlag, "repo", "", "GitHub repo owner/name")
	return cmd
}
