package tui

import (
	"context"
	"fmt"
	"time"

	"github.com/iftimiemarius/dispatch/internal/config"
	"github.com/iftimiemarius/dispatch/internal/graph"
	"github.com/iftimiemarius/dispatch/internal/models"
)

// syncBlockToOutlook pushes a block to Outlook and returns the Graph event id.
// It returns a friendly error if Outlook isn't configured or authenticated.
func syncBlockToOutlook(ctx context.Context, b *models.Block) (string, error) {
	paths, err := config.Resolve()
	if err != nil {
		return "", fmt.Errorf("resolve config: %w", err)
	}
	cfg, err := config.Load(paths.ConfigFile)
	if err != nil {
		return "", fmt.Errorf("load config: %w", err)
	}
	if !cfg.OutlookEnabled() {
		return "", fmt.Errorf("not configured (run `dispatch auth login outlook`)")
	}
	auth := graph.NewAuthenticator(cfg.Outlook.ClientID, cfg.Outlook.Tenant, cfg.Outlook.RedirectPort)
	gc, err := graph.NewClientFromAuthenticator(ctx, auth)
	if err != nil {
		return "", err
	}
	// If already synced, update; otherwise create.
	if b.OutlookEventID != nil && *b.OutlookEventID != "" {
		if err := gc.UpdateEvent(ctx, *b.OutlookEventID, graph.EventFromBlock(blockLike(b), "")); err != nil {
			return "", err
		}
		return *b.OutlookEventID, nil
	}
	id, err := gc.CreateEvent(ctx, graph.EventFromBlock(blockLike(b), ""))
	if err != nil {
		return "", err
	}
	return id, nil
}

// blockLike converts a models.Block to the graph.BlockLike mapper input.
func blockLike(b *models.Block) graph.BlockLike {
	return graph.BlockLike{
		Title: b.Title, Notes: b.Notes, TaskID: b.TaskID,
		StartsAt: b.StartsAt, EndsAt: b.EndsAt,
	}
}

// keep time referenced for potential future use.
var _ = time.Now
