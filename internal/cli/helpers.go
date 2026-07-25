package cli

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/iftimiemarius/dispatch/internal/models"
	"github.com/iftimiemarius/dispatch/internal/store"
	"github.com/iftimiemarius/dispatch/internal/timeparse"
)

// resolveTask resolves a task reference (full ID, short ID suffix) into a task,
// turning store errors into friendly CLI messages.
func resolveTask(ctx context.Context, st *store.Store, ref string) (*models.Task, error) {
	t, err := st.ResolveTask(ctx, ref)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, fmt.Errorf("no task matching %q (try `dispatch ls`)", ref)
		}
		if errors.Is(err, store.ErrAmbiguous) {
			return nil, fmt.Errorf("%w — use more characters", err)
		}
		return nil, err
	}
	return t, nil
}

// resolveProjectRef resolves a project reference (full ID, short ID, or name).
func resolveProjectRef(ctx context.Context, st *store.Store, ref string) (*models.Project, error) {
	p, err := st.ResolveProject(ctx, ref)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, fmt.Errorf("no project matching %q", ref)
		}
		if errors.Is(err, store.ErrAmbiguous) {
			return nil, fmt.Errorf("%w — use more characters", err)
		}
		return nil, err
	}
	return p, nil
}

// resolveBlockRef resolves a block reference (full ID or short ID suffix).
func resolveBlockRef(ctx context.Context, st *store.Store, ref string) (*models.Block, error) {
	b, err := st.ResolveBlock(ctx, ref)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, fmt.Errorf("no block matching %q", ref)
		}
		if errors.Is(err, store.ErrAmbiguous) {
			return nil, fmt.Errorf("%w — use more characters", err)
		}
		return nil, err
	}
	return b, nil
}

// parseDue converts a due flag string to an absolute time, anchored to now.
func parseDue(s string, now time.Time) (time.Time, error) {
	t, err := timeparse.Parse(s, now)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid --due %q: %w", s, err)
	}
	if t.IsZero() {
		return time.Time{}, fmt.Errorf("invalid --due %q", s)
	}
	return t, nil
}

// resolveProjectID accepts a project name, full ID, or short ID and returns its
// full ID, verifying it exists.
func resolveProjectID(ctx context.Context, st *store.Store, ref string) (string, error) {
	p, err := resolveProjectRef(ctx, st, ref)
	if err != nil {
		return "", err
	}
	return p.ID, nil
}

// resolveInitiativeID accepts an initiative name or ID and returns its ID.
func resolveInitiativeID(ctx context.Context, st *store.Store, ref string) (string, error) {
	if i, err := st.GetInitiative(ctx, ref); err == nil {
		return i.ID, nil
	}
	// Fall back to scanning the list by name (names aren't unique on initiatives).
	list, err := st.ListInitiatives(ctx)
	if err != nil {
		return "", err
	}
	for _, i := range list {
		if i.Name == ref {
			return i.ID, nil
		}
	}
	return "", fmt.Errorf("initiative %q not found", ref)
}
