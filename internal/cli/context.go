package cli

import (
	"context"

	"github.com/iftimiemarius/dispatch/internal/config"
	"github.com/iftimiemarius/dispatch/internal/store"
)

type (
	ctxStoreKey struct{}
	ctxPathsKey struct{}
)

// contextWithStore returns a new context carrying the store.
func contextWithStore(ctx context.Context, st *store.Store) context.Context {
	return context.WithValue(ctx, ctxStoreKey{}, st)
}

// storeFromContext retrieves the store attached to ctx, if any.
func storeFromContext(ctx context.Context) (*store.Store, bool) {
	st, ok := ctx.Value(ctxStoreKey{}).(*store.Store)
	return st, ok
}

// contextWithPaths returns a new context carrying the resolved paths.
func contextWithPaths(ctx context.Context, p *config.Paths) context.Context {
	return context.WithValue(ctx, ctxPathsKey{}, p)
}

// pathsFromContext retrieves the resolved paths attached to ctx, if any.
func pathsFromContext(ctx context.Context) (*config.Paths, bool) {
	p, ok := ctx.Value(ctxPathsKey{}).(*config.Paths)
	return p, ok
}

// MustStore returns the store from the command's context, panicking if absent.
// Subcommands should only call this when their parent has opened a store
// (i.e. the command does not carry skip_db).
func MustStore(cmd interface{ Context() context.Context }) *store.Store {
	st, ok := storeFromContext(cmd.Context())
	if !ok {
		panic("dispatch: store not opened; command should set skip_db or be a child of root")
	}
	return st
}

// MustPaths returns the resolved paths from the command's context.
func MustPaths(cmd interface{ Context() context.Context }) *config.Paths {
	p, ok := pathsFromContext(cmd.Context())
	if !ok {
		panic("dispatch: paths not resolved")
	}
	return p
}
