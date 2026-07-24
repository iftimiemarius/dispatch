# Dispatch

> Plan work like a developer, not like a project manager.

A CLI-first work orchestration tool for developers. Dispatch bridges four layers that are usually separated — **inbox capture, structured execution, project/initiative planning, and time blocking** — into one fast, local-first system.

## Why

Most planning tools live in the browser and force you to think like a project manager. Dispatch lives in your terminal and fits how developers actually work: you capture a thought in two seconds, organize it when you can, and turn it into a real block of focus time. Everything is local, fast, and scriptable.

## Status

- [x] **M1** — Skeleton & embedded SQLite store (pure Go, no CGO)
- [x] **M2** — Task capture & listing
- [x] **M3** — Projects & initiatives
- [x] **M4** — Calendar blocking & `.ics` export
- [x] **M5** — Focus picker, packaging

GitHub integration is **out of scope for v1** and will arrive once the core is mature.

## Install

Requires Go 1.23+.

```sh
git clone https://github.com/iftimiemarius/dispatch.git
cd dispatch
make install        # builds and installs to $GOPATH/bin
```

Or build directly:

```sh
make build VERSION=0.1.0   # produces ./dispatch
```

## Data model

A flexible hierarchy where every link is optional (a captured task starts in the **inbox**):

```
Initiative → Project → Task → Block
```

| Entity       | Role                                              |
|--------------|---------------------------------------------------|
| `task`       | atomic unit of work; starts in the inbox          |
| `project`    | execution grouping ("the thing I'm building")     |
| `initiative` | strategic grouping ("the outcome I'm driving")    |
| `block`      | a time reservation on the calendar for a task     |

Data lives in an embedded SQLite database at `~/.local/share/dispatch/dispatch.db` (XDG-aware). No network calls.

## Usage

### Capture

```sh
dispatch add "fix login redirect loop"            # quick inbox capture
dispatch + "write docs" -P low                    # + is an alias for add
dispatch add "ship endpoint" -p api -P high -t backend,auth --due tomorrow 9am
```

### Tasks

```sh
dispatch ls                  # list open tasks
dispatch ls --inbox          # just the inbox
dispatch ls -p api           # filter by project
dispatch ls --group project  # grouped by project, inbox first
dispatch ls --all            # include done/cancelled

dispatch task show <id>      # details
dispatch done <id>           # mark done (--undo to reopen)
dispatch start <id>          # mark in-progress
dispatch edit <id> -P urgent --due fri
dispatch mv <id> -p docs     # move to a project
dispatch rm <id>
```

### Projects & Initiatives

```sh
dispatch init add "Q3 Launch" -o "ship v2 to early users" --target "+60d"
dispatch project add api -d "the REST API" -i "Q3 Launch"
dispatch project ls          # with per-project open task counts
dispatch project show api    # the project and its tasks
dispatch init show "Q3 Launch"
```

### Time blocking

```sh
dispatch block add "deep work" --from 9am --duration 2h --day today
dispatch block add --task <id> --from 14:00 --to 15:30 --day tomorrow
dispatch block ls            # upcoming, grouped by day
dispatch block rm <id>

dispatch calendar export                 # writes ~/.local/share/dispatch/calendar.ics
dispatch calendar export --print --range today
```

Import the `.ics` into any calendar app (Google Calendar, Apple Calendar, etc.).

### Focus

```sh
dispatch today              # agenda: schedule, due tasks, inbox triage
dispatch next               # the one task to focus on now
dispatch next --start       # ...and mark it in-progress
```

`next` resumes any in-progress task first, otherwise picks the highest-priority todo (falling back to inbox), tie-breaking by due date.

## Time input

Due dates and block times accept developer-friendly forms:

| Input             | Meaning                              |
|-------------------|--------------------------------------|
| `today`, `now`    | today                                |
| `tomorrow 9am`    | tomorrow at 09:00                    |
| `fri`, `next mon` | this/next weekday                    |
| `+2h`, `3d`, `1w` | relative offset from now             |
| `2025-12-01`      | absolute date                        |
| `9am`, `14:30`    | a clock (defaults to today)          |

For due dates, a clock on "today" that has already passed rolls forward to tomorrow. For blocks, the literal time is used.

## Project layout

```
cmd/dispatch/         entrypoint
internal/
  cli/                Cobra command tree
  models/             domain types + ID generation
  store/              SQLite repository (schema, migrations, CRUD)
  calendar/           RFC 5545 .ics writer
  config/             XDG path resolution
  ui/                 terminal tables/colors (lipgloss)
  timeparse/          natural-ish time parsing
```

## License

MIT
