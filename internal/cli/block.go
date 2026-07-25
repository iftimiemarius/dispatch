package cli

import (
	"context"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/iftimiemarius/dispatch/internal/calendar"
	"github.com/iftimiemarius/dispatch/internal/models"
	"github.com/iftimiemarius/dispatch/internal/store"
	"github.com/iftimiemarius/dispatch/internal/timeparse"
	"github.com/iftimiemarius/dispatch/internal/ui"
	"github.com/spf13/cobra"
)

func newBlockCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "block",
		Aliases: []string{"blocks"},
		Short:   "Manage calendar time blocks",
	}
	cmd.AddCommand(
		newBlockAddCmd(),
		newBlockLsCmd(),
		newBlockRmCmd(),
		newBlockSyncCmd(),
		newBlockUnsyncCmd(),
	)
	return cmd
}

func newBlockAddCmd() *cobra.Command {
	var (
		taskID string
		from   string
		to     string
		day    string
		dur    string
		title  string
		notes  string
	)
	cmd := &cobra.Command{
		Use:   "add [title]",
		Short: "Create a calendar block (optionally for a task)",
		Long: `Create a time block on the calendar.

Provide a time range with --from and --to, or --from and --duration. A day may
be given with --day (default: today). Examples:

  dispatch block add --task <id> --from 9am --to 11am --day tomorrow
  dispatch block "deep work" --from 14:00 --duration 2h`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st := MustStore(cmd)
			ctx := cmd.Context()
			now := time.Now()

			if from == "" {
				return fmt.Errorf("--from is required")
			}
			start, err := resolveBlockTime(ctx, day, from, now)
			if err != nil {
				return err
			}
			var end time.Time
			if to != "" {
				end, err = resolveBlockTime(ctx, day, to, now)
				if err != nil {
					return err
				}
			} else if dur != "" {
				// --duration is a Go duration offset from start (e.g. "2h", "90m").
				d2, err2 := time.ParseDuration(dur)
				if err2 != nil {
					return fmt.Errorf("invalid --duration %q: %w", dur, err2)
				}
				if d2 <= 0 {
					return fmt.Errorf("--duration must be positive")
				}
				end = start.Add(d2)
			} else {
				return fmt.Errorf("provide --to or --duration")
			}
			if !end.After(start) {
				return fmt.Errorf("block end must be after start")
			}

			title := joinArgs(args)
			if title == "" && taskID != "" {
				if t, err := resolveTask(ctx, st, taskID); err == nil {
					title = t.Title
					taskID = t.ID // normalize to full ID
				}
			}
			if title == "" {
				title = "focus block"
			}

			b := &models.Block{
				ID:        models.NewID(),
				Title:     title,
				Notes:     notes,
				StartsAt:  start,
				EndsAt:    end,
				CreatedAt: now.UTC(),
				UpdatedAt: now.UTC(),
			}
			if taskID != "" {
				// Verify the task exists before linking (and normalize to full ID).
				t, err := resolveTask(ctx, st, taskID)
				if err != nil {
					return err
				}
				full := t.ID
				b.TaskID = &full
			}
			if err := st.CreateBlock(ctx, b); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s blocked %s %s→%s: %s\n",
				ui.Dim(b.ID),
				relativeDay(start), start.Format("15:04"), end.Format("15:04"),
				title)
			return nil
		},
	}
	cmd.Flags().StringVarP(&taskID, "task", "t", "", "task ID to block time for")
	cmd.Flags().StringVar(&from, "from", "", "start time, e.g. 9am, 14:30")
	cmd.Flags().StringVar(&to, "to", "", "end time")
	cmd.Flags().StringVar(&day, "day", "", "day for the block (default: today)")
	cmd.Flags().StringVar(&dur, "duration", "", "block length, e.g. 2h, 90m")
	cmd.Flags().StringVarP(&title, "title", "T", "", "block title (overrides positional)")
	cmd.Flags().StringVarP(&notes, "notes", "n", "", "notes")
	return cmd
}

func joinArgs(args []string) string {
	out := ""
	for i, a := range args {
		if i > 0 {
			out += " "
		}
		out += a
	}
	return out
}

// resolveBlockTime combines a day token (optional) and a clock token into an
// absolute time. If the clock token alone parses to a time on its own (e.g.
// "tomorrow 9am"), day is ignored.
func resolveBlockTime(ctx context.Context, day, clock string, now time.Time) (time.Time, error) {
	s := clock
	if day != "" {
		s = day + " " + clock
	}
	t, err := timeparse.ParseNoRoll(s, now)
	if err != nil || t.IsZero() {
		return time.Time{}, fmt.Errorf("invalid time %q: %w", s, err)
	}
	return t, nil
}

func relativeDay(t time.Time) string {
	t = t.Local()
	today := time.Now().Local()
	ay, am, ad := t.Date()
	by, bm, bd := today.Date()
	if ay == by && am == bm && ad == bd {
		return "today"
	}
	if ay == by && am == bm && ad == bd+1 {
		return "tomorrow"
	}
	return t.Format("Mon Jan 2")
}

func newBlockLsCmd() *cobra.Command {
	var rangeFlag string
	cmd := &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Short:   "List upcoming blocks",
		RunE: func(cmd *cobra.Command, args []string) error {
			st := MustStore(cmd)
			ctx := cmd.Context()
			r := resolveBlockRange(rangeFlag)
			blocks, err := st.ListBlocks(ctx, r)
			if err != nil {
				return err
			}
			// Drop blocks already in the past unless range is "all".
			if rangeFlag != "all" {
				blocks = filterFuture(blocks)
			}
			if len(blocks) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), ui.Dim("No blocks."))
				return nil
			}
			renderBlocks(cmd.OutOrStdout(), blocks)
			return nil
		},
	}
	cmd.Flags().StringVar(&rangeFlag, "range", "week", "range: today|week|all")
	return cmd
}

func newBlockRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "rm <id>",
		Aliases: []string{"delete"},
		Short:   "Delete a block",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st := MustStore(cmd)
			ctx := cmd.Context()
			b, err := resolveBlockRef(ctx, st, args[0])
			if err != nil {
				return err
			}
			if err := st.DeleteBlock(ctx, b.ID); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s removed: %s\n", ui.Dim(b.ID), b.Title)
			return nil
		},
	}
}

// renderBlocks prints blocks with day headers.
func renderBlocks(out interface{ Write([]byte) (int, error) }, blocks []*models.Block) {
	sort.Slice(blocks, func(i, j int) bool { return blocks[i].StartsAt.Before(blocks[j].StartsAt) })
	w := func(s string) { fmt.Fprintln(out, s) }
	var lastDay string
	for _, b := range blocks {
		day := b.StartsAt.Local().Format("Mon Jan 2")
		if day != lastDay {
			if lastDay != "" {
				w("")
			}
			w(ui.Section(day))
			lastDay = day
		}
		label := fmt.Sprintf("%s–%s", b.StartsAt.Format("15:04"), b.EndsAt.Format("15:04"))
		title := b.Title
		if b.TaskID != nil {
			title += " " + ui.Dim("(task "+*b.TaskID+")")
		}
		w(fmt.Sprintf("%s  %s  %s", ui.Dim(b.ID), ui.Dim(label), title))
	}
}

// resolveBlockRange maps the --range flag to a BlockRange.
func resolveBlockRange(flag string) *store.BlockRange {
	now := time.Now().Local()
	switch flag {
	case "today":
		start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		return &store.BlockRange{From: start, To: start.Add(24 * time.Hour)}
	case "all":
		return nil
	default: // "week" and anything else
		start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		return &store.BlockRange{From: start, To: start.Add(7 * 24 * time.Hour)}
	}
}

func filterFuture(blocks []*models.Block) []*models.Block {
	now := time.Now()
	out := blocks[:0]
	for _, b := range blocks {
		if b.EndsAt.After(now) {
			out = append(out, b)
		}
	}
	return out
}

// --- calendar export ---

func newCalendarCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "calendar",
		Short: "Calendar export (.ics) and Outlook sync",
	}
	cmd.AddCommand(newCalendarExportCmd(), newCalendarSyncCmd())
	return cmd
}

// newCalendarSyncCmd syncs all blocks in a window to Outlook. It's the
// calendar-level entry point; `block sync --range` does the same thing.
func newCalendarSyncCmd() *cobra.Command {
	var rangeFlag string
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync all blocks to Outlook calendar",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st := MustStore(cmd)
			ctx := cmd.Context()
			_, gc, err := outlookClient()
			if err != nil {
				return err
			}
			r := resolveBlockRange(rangeFlag)
			blocks, err := st.ListBlocks(ctx, r)
			if err != nil {
				return err
			}
			if len(blocks) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), ui.Dim("No blocks in range."))
				return nil
			}
			ok, failed := 0, 0
			for _, b := range blocks {
				if err := syncOne(cmd, gc, b); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "  %s FAILED: %v\n", ui.Dim(b.ID), err)
					failed++
				} else {
					ok++
				}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Synced %d, failed %d.\n", ok, failed)
			return nil
		},
	}
	cmd.Flags().StringVar(&rangeFlag, "range", "all", "range: today|week|all")
	return cmd
}

func newCalendarExportCmd() *cobra.Command {
	var (
		out   string
		r     string
		print bool
	)
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export blocks to an .ics calendar file",
		Long: `Export time blocks to an iCalendar (.ics) file for import into any
calendar app. With --print, write to stdout instead of a file.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st := MustStore(cmd)
			ctx := cmd.Context()
			paths := MustPaths(cmd)

			blocks, err := st.ListBlocks(ctx, resolveBlockRange(r))
			if err != nil {
				return err
			}
			// Gather task titles for nicer event summaries.
			titles := blockTaskTitles(ctx, st, blocks)

			cal := calendar.FromBlocks(blocks, titles)
			encoded := cal.Encode()

			if print {
				fmt.Fprint(cmd.OutOrStdout(), encoded)
				return nil
			}
			target := paths.DefaultICS
			if out != "" {
				target = out
			}
			if err := os.WriteFile(target, []byte(encoded), 0o644); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Exported %d blocks to %s\n", len(blocks), target)
			return nil
		},
	}
	cmd.Flags().StringVarP(&out, "out", "o", "", "output path (default: XDG data dir/calendar.ics)")
	cmd.Flags().StringVar(&r, "range", "all", "range: today|week|all")
	cmd.Flags().BoolVar(&print, "print", false, "print .ics to stdout instead of writing a file")
	return cmd
}

func blockTaskTitles(ctx context.Context, st *store.Store, blocks []*models.Block) map[string]string {
	titles := make(map[string]string)
	for _, b := range blocks {
		if b.TaskID == nil {
			continue
		}
		if _, ok := titles[*b.TaskID]; ok {
			continue
		}
		if t, err := st.GetTask(ctx, *b.TaskID); err == nil {
			titles[*b.TaskID] = t.Title
		}
	}
	return titles
}
