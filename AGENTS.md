# AGENTS.md — Dispatch

> Guidance for coding agents (Claude Code, Cursor, Codex, etc.) operating in
> this repository or driving the `dispatch` CLI.

## What Dispatch is

`dispatch` is a local-first CLI for work orchestration. It bridges capture,
execution, planning, and time-blocking into one tool. All data lives in an
embedded SQLite database — **no network calls, ever**.

Data model (every link optional; captured tasks start in the **inbox**):

```
Initiative → Project → Task → Block
```

Task lifecycle: `inbox → todo → doing → done` (plus `blocked`, `cancelled`).

IDs are ULIDs. Listings show only the **last 6 characters**; commands accept
either the short suffix or the full ID.

## Building & testing

```sh
make build              # produces ./dispatch
make test               # go test ./...
make vet                # go vet ./...
make build VERSION=0.1.0   # version-stamped build
```

- Go module: `github.com/iftimiemarius/dispatch`
- Pure-Go SQLite (`modernc.org/sqlite`) — **no CGO**. Keep it that way.
- Run `go vet ./...` after changes; CI runs `go test -race ./...`.

## Code layout

```
cmd/dispatch/      entrypoint (main only)
internal/
  cli/             Cobra command tree + per-entity command files
  models/          domain types + ULID generation
  store/           SQLite repository: schema.sql, migrations, CRUD, resolvers
  calendar/        RFC 5545 .ics writer
  config/          XDG path resolution
  release/         GitHub releases client + checksum verify (for `dispatch upgrade`)
  timeparse/       natural-ish time parsing
  ui/              terminal rendering (lipgloss)
```

## Conventions

- **One command family per file** in `internal/cli/` (task.go, project.go, …).
- **Store layer** owns all SQL; the CLI never writes raw SQL. Add new queries as
  methods on `*Store` in the matching `internal/store/<entity>.go`.
- **Migrations**: add new columns/tables in `internal/store/migrations/NNN_*.sql`
  AND `schema.sql` (baseline), then bump `currentSchemaVersion`. The runner
  tolerates duplicate-column errors (idempotent).
- **Resolvers**: use `Resolve*` (full ID, short ID, or name) for lookups — never
  assume a full ID. `ResolveTask`/`ResolveProject`/`ResolveBlock` exist.
- **Display**: titles first, short IDs last, medium-priority hidden in tables.
  See `internal/ui/render.go`.
- **Time input** flows through `internal/timeparse` — extend there, not inline.
- **Integrations**: GitHub reuses `gh` (internal/github); Outlook uses Microsoft
  Graph OAuth+PKCE (internal/graph). Both degrade gracefully when unavailable.
- **New commands** must be registered in `internal/cli/root.go`'s `AddCommand`.
  Commands that don't need the DB set `Annotations: map[string]string{"skip_db": "true"}`.

## When driving dispatch from a shell

If a user asks you to capture/plan/track work:

1. **Capture first**: `dispatch add "..."` (don't invent metadata they didn't give).
2. **Discover real IDs** with `dispatch ls` before acting — never fabricate IDs.
3. **Prefer short IDs** in your responses (last 6 chars).
4. **Verify** by running the relevant `ls`/`show` after a mutation.
5. For "what's next" questions, use `dispatch today` and `dispatch next`.

Full command reference and workflow: see `skills/dispatch/SKILL.md`.
