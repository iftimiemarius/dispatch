package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/iftimiemarius/dispatch/internal/graph"
	"github.com/iftimiemarius/dispatch/internal/models"
	"github.com/iftimiemarius/dispatch/internal/ui"
	"github.com/spf13/cobra"
)

// outlookClient builds an authenticated Graph client, failing with a helpful
// hint if Outlook isn't configured or authenticated.
func outlookClient() (*graph.Authenticator, *graph.Client, error) {
	_, a, err := loadOutlookConfig()
	if err != nil {
		return nil, nil, err
	}
	c, err := graph.NewClientFromAuthenticator(cmdCtx(), a)
	if err != nil {
		return nil, nil, err
	}
	return a, c, nil
}

// newBlockSyncCmd pushes a block (or a range of blocks) to Outlook.
func newBlockSyncCmd() *cobra.Command {
	var rangeFlag string
	cmd := &cobra.Command{
		Use:   "sync [<block-id>] [--range today|week|all]",
		Short: "Push block(s) to Outlook calendar",
		Long: `Push a block to your Outlook calendar, creating an event if none exists
or updating the linked event otherwise. With --range, syncs all blocks in the
window.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st := MustStore(cmd)
			ctx := cmd.Context()
			_, gc, err := outlookClient()
			if err != nil {
				return err
			}

			// Single block by ID.
			if len(args) == 1 {
				b, err := resolveBlockRef(ctx, st, args[0])
				if err != nil {
					return err
				}
				return syncOne(cmd, gc, b)
			}

			// Range sync.
			if rangeFlag == "" {
				return fmt.Errorf("provide a block id or --range today|week|all")
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
					fmt.Fprintf(cmd.ErrOrStderr(), "  %s %s: %v\n", ui.Dim(b.ID), "FAILED", err)
					failed++
				} else {
					ok++
				}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Synced %d, failed %d.\n", ok, failed)
			return nil
		},
	}
	cmd.Flags().StringVar(&rangeFlag, "range", "", "sync all blocks in: today|week|all")
	return cmd
}

// syncOne creates or updates a single block's Outlook event and stores the id.
func syncOne(cmd *cobra.Command, gc *graph.Client, b *models.Block) error {
	ctx := cmd.Context()
	ev := graph.EventFromBlock(blockToGraph(b), "")

	if b.OutlookEventID != nil && *b.OutlookEventID != "" {
		// Update existing event.
		if err := gc.UpdateEvent(ctx, *b.OutlookEventID, ev); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s updated event %s\n", ui.Dim(b.ID), *b.OutlookEventID)
		return nil
	}
	// Create new event.
	id, err := gc.CreateEvent(ctx, ev)
	if err != nil {
		return err
	}
	st := MustStore(cmd)
	b.OutlookEventID = &id
	b.UpdatedAt = time.Now().UTC()
	if err := st.UpdateBlock(ctx, b); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s created event %s\n", ui.Dim(b.ID), ui.Dim(id))
	return nil
}

// blockToGraph converts a models.Block to the graph.blockLike mapper input.
func blockToGraph(b *models.Block) graph.BlockLike {
	return graph.BlockLike{
		Title: b.Title, Notes: b.Notes, TaskID: b.TaskID,
		StartsAt: b.StartsAt, EndsAt: b.EndsAt,
	}
}

// newBlockUnsyncCmd deletes a block's Outlook event and clears the link.
func newBlockUnsyncCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "unsync <block-id>",
		Short: "Delete a block's Outlook event",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st := MustStore(cmd)
			ctx := cmd.Context()
			b, err := resolveBlockRef(ctx, st, args[0])
			if err != nil {
				return err
			}
			if b.OutlookEventID == nil || *b.OutlookEventID == "" {
				return fmt.Errorf("block is not synced to Outlook")
			}
			_, gc, err := outlookClient()
			if err != nil {
				return err
			}
			if err := gc.DeleteEvent(ctx, *b.OutlookEventID); err != nil {
				return err
			}
			b.OutlookEventID = nil
			b.UpdatedAt = time.Now().UTC()
			if err := st.UpdateBlock(ctx, b); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s Outlook event deleted\n", ui.Dim(b.ID))
			return nil
		},
	}
}

// cmdCtx returns the current command's context; defined here to satisfy the
// outlookClient helper's standalone use. Commands pass their own ctx in the
// RunE paths above, so this is only used by helper scaffolding.
func cmdCtx() context.Context { return context.Background() }
