# Dispatch

> Plan work like a developer, not like a project manager.

A CLI-first work orchestration tool for developers. Dispatch bridges four layers that are usually separated — **inbox capture, structured execution, project/initiative planning, and time blocking** — into one fast, local-first system.

## Status

Early in development.

- [x] **M1** — Skeleton & embedded SQLite store (pure Go, no CGO)
- [ ] **M2** — Task capture & listing (`add`, `ls`, `done`, …)
- [ ] **M3** — Projects & initiatives
- [ ] **M4** — Calendar blocking & `.ics` export
- [ ] **M5** — Polish: focus picker, config, packaging

GitHub integration is **out of scope for v1** and will arrive once the core is mature.

## Build

Requires Go 1.23+.

```sh
go build -o dispatch ./cmd/dispatch
./dispatch version
```

## Data model

Hierarchy where every link is optional (a captured task starts in the **inbox**):

```
Initiative → Project → Task → Block
```

| Entity     | Role                                            |
|------------|-------------------------------------------------|
| `task`     | atomic unit of work; starts in the inbox        |
| `project`  | execution grouping ("the thing I'm building")   |
| `initiative` | strategic grouping ("the outcome I'm driving") |
| `block`    | a time reservation on the calendar for a task   |

Data lives in an embedded SQLite database at `~/.local/share/dispatch/dispatch.db` (XDG-aware). No network calls.
