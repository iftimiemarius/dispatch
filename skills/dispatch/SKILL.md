---
name: dispatch
description: Use to capture, plan, and track work with the `dispatch` CLI — a local-first work orchestration tool. Applies when the user wants to add/capture a task or todo, organize work into projects and initiatives, list or filter tasks, mark work done/in-progress, plan time blocks, export a calendar (.ics), or figure out what to focus on next. Provides the full command reference, the task lifecycle (inbox → todo → doing → done), the Initiative→Project→Task→Block data model, short-ID conventions, and the recommended workflow for turning a vague request into structured, scheduled work.
---

# Dispatch — Work Orchestration CLI

`dispatch` is a local-first CLI that bridges four layers: **capture**, **execution**, **planning**, and **time blocking**. All data lives in an embedded SQLite DB (no network). Every command is a one-shot shell invocation.

## Data model

A flexible hierarchy where every link is optional. A freshly captured task starts in the **inbox** until triaged:

```
Initiative → Project → Task → Block
```

| Entity     | Role                                          |
|------------|-----------------------------------------------|
| `task`     | atomic unit of work; starts in the inbox      |
| `project`  | execution grouping ("the thing I'm building") |
| `initiative` | strategic grouping ("the outcome I'm driving") |
| `block`    | a time reservation on the calendar for a task |

## Task lifecycle

`inbox` → `todo` → `doing` → `done`  (plus `blocked`, `cancelled`)

- `inbox`: just captured, not yet triaged
- `todo`: triaged, ready to work
- `doing`: in progress
- `done` / `cancelled`: finished

## IDs

Task IDs are ULIDs. In listings, **only the last 6 characters are shown** (e.g. `VGHBZX`). Commands accept either the short suffix or the full ID. If a short ID matches multiple tasks, you'll get an "ambiguous" error — use more characters.

## Command reference

### Capture

```sh
dispatch add "title"                       # capture into the inbox
dispatch + "title"                         # + is an alias for add
dispatch add "title" -P high -t tag1,tag2 --due tomorrow 9am
dispatch add "title" -p <project> -i <initiative>
```

### List & inspect

```sh
dispatch ls                  # open tasks (title first, short ID last)
dispatch ls --inbox          # only inbox
dispatch ls -p <project>     # filter by project
dispatch ls --group project  # grouped by project, inbox first
dispatch ls --status doing   # by status
dispatch ls --all            # include done/cancelled
dispatch task show <id>      # full details
```

### Lifecycle & edit

```sh
dispatch done <id>           # mark done (--undo to reopen)
dispatch start <id>          # mark in-progress
dispatch rm <id>             # delete
dispatch edit <id> -P urgent --due fri
dispatch mv <id> -p <project>  # move/assign to a project
```

### Projects & Initiatives

```sh
dispatch init add "Name" -o "outcome" --target "+60d"
dispatch project add <name> -d "desc" -i <initiative>
dispatch project ls          # with per-project open task counts
dispatch project show <name|id>
dispatch init show <name|id>
```

### Time blocking & calendar

```sh
dispatch block add "title" --from 9am --duration 2h --day today
dispatch block add --task <id> --from 14:00 --to 15:30 --day tomorrow
dispatch block ls            # upcoming, grouped by day
dispatch calendar export     # writes .ics (import to any calendar app)
dispatch calendar export --print --range today
```

### Focus

```sh
dispatch today               # agenda: schedule, due tasks, inbox
dispatch next                # the one task to focus on now
dispatch next --start        # ...and mark it in-progress
```

## Time input

`--due` and block times accept: `today`, `tomorrow 9am`, `fri`, `next mon`, `+2h`, `3d`, `1w`, `2025-12-01`, `9am`, `14:30`.

## Recommended workflow for a new request

When a user asks to "track", "plan", "remember", or "schedule" work, follow this flow:

1. **Capture first** — never lose a thought. `dispatch add "..."` drops it in the inbox. Add `-P`/`-t`/`--due` only if the user specified them; don't invent metadata.
2. **Structure if asked** — if the user names a project or initiative, create it (`dispatch project add` / `dispatch init add`) and `dispatch mv <id> -p <project>`.
3. **Verify with listings** — run `dispatch ls` or `dispatch ls --group project` to show the result.
4. **Update lifecycle as work progresses** — `dispatch start <id>`, then `dispatch done <id>`.
5. **Schedule focus time** — `dispatch block add --task <id> ...` when the user wants to reserve time.
6. **Surface what matters** — use `dispatch today` (agenda) and `dispatch next` (next focus) when the user asks "what should I work on".

## Notes for agents

- **Always pass the task ID** you captured back to the user (the short form is fine) so they can act on it.
- **Don't fabricate IDs.** Run `dispatch ls` to discover real IDs before acting on a task the user referenced by description.
- Prefer the **short ID** in your responses to keep them readable.
- `dispatch` is **idempotent and local** — safe to run repeatedly; it never makes network calls.
- If a command fails with "ambiguous", re-run `dispatch ls` and use a longer ID suffix.
