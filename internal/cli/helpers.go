package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/iftimiemarius/dispatch/internal/store"
	"github.com/iftimiemarius/dispatch/internal/timeparse"
)

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

// resolveProjectID accepts a project name or ID and returns its ID, verifying
// it exists.
func resolveProjectID(ctx context.Context, st *store.Store, ref string) (string, error) {
	// Try by ID first (cheap).
	if p, err := st.GetProject(ctx, ref); err == nil {
		return p.ID, nil
	}
	p, err := st.GetProjectByName(ctx, ref)
	if err != nil {
		return "", fmt.Errorf("project %q not found", ref)
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
