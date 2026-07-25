// Package cli wires up the Cobra command tree for the dispatch binary.
//
// The root command resolves config paths and opens the store, then exposes
// them to subcommands through the root's *store.Store field via the Store()
// accessor. Subcommands obtain the store with Store(cmd).
package cli

import (
	"fmt"
	"os"

	"github.com/iftimiemarius/dispatch/internal/config"
	"github.com/iftimiemarius/dispatch/internal/tui"
	"github.com/iftimiemarius/dispatch/internal/store"
	"github.com/spf13/cobra"
)

// Version is the current dispatch version. Overridden at link time for releases.
var Version = "0.1.0-dev"

// storeKey is the context key under which the open store is attached.
type storeKey struct{}

// Root builds and returns the root command. Calling Execute on it runs the app.
func Root() *cobra.Command {
	var (
		dbPathOverride string
		noTUI          bool
	)

	root := &cobra.Command{
		Use:           "dispatch",
		Short:         "A CLI-first work orchestration tool for developers",
		Long:          "Dispatch captures tasks, organizes them into projects and initiatives,\nand turns priorities into calendar blocks.",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Commands may opt out of DB init with the skip_db annotation.
			if cmd.Annotations["skip_db"] == "true" {
				return nil
			}
			paths, err := config.Resolve()
			if err != nil {
				return err
			}
			dbPath := paths.DBPath
			if dbPathOverride != "" {
				dbPath = dbPathOverride
			}
			st, err := store.Open(dbPath)
			if err != nil {
				return err
			}
			// Attach the store to this command's context so descendants inherit it.
			ctx := cmd.Context()
			ctx = contextWithStore(ctx, st)
			ctx = contextWithPaths(ctx, paths)
			cmd.SetContext(ctx)
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Bare `dispatch` (no args, no flags) launches the interactive TUI.
			// `--help`/`--version`/subcommands resolve before RunE, so they're
			// unaffected. `--no-tui` forces the legacy help-on-bare behavior.
			if noArgsNoFlags() && !noTUI {
				return tui.Run(cmd.Context(), MustStore(cmd))
			}
			return cmd.Help()
		},
	}

	root.PersistentFlags().StringVar(&dbPathOverride, "db", "", "path to the database file (overrides XDG default)")
	root.PersistentFlags().BoolVar(&noTUI, "no-tui", false, "don't launch the TUI on bare invocation (print help instead)")

	// noArgsNoFlags (package-level) reports whether the user invoked dispatch
	// with no arguments and no flags — the trigger for launching the TUI.

	root.AddCommand(
		newVersionCmd(),
		newAddCmd(),
		newLsCmd(),
		newTaskCmd(),
		newProjectCmd(),
		newInitiativeCmd(),
		newBlockCmd(),
		newCalendarCmd(),
		newTodayCmd(),
		newNextCmd(),
		newUpgradeCmd(),
		newGhCmd(),
		newAuthCmd(),
		// Top-level convenience shortcuts for the most common task actions.
		newTaskDoneCmd(),
		newTaskRmCmd(),
		newTaskStartCmd(),
		newTaskMvCmd(),
	)
	return root
}

// noArgsNoFlags reports whether dispatch was invoked with no arguments and no
// flags (just `dispatch`) — the trigger for launching the TUI. os.Args[0] is
// the program name; any additional entry is an arg or flag.
func noArgsNoFlags() bool {
	return len(os.Args) <= 1
}

// Execute runs the root command, prints errors to stderr, and returns an exit
// code. It closes any store that was opened.
func Execute() int {
	root := Root()
	err := root.Execute()
	// Best-effort cleanup of the store if one was attached to the root context.
	if st, ok := storeFromContext(root.Context()); ok {
		_ = st.Close()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	return 0
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:        "version",
		Short:      "Print the dispatch version",
		Annotations: map[string]string{"skip_db": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintf(cmd.OutOrStdout(), "dispatch v%s\n", Version)
			return nil
		},
	}
}
