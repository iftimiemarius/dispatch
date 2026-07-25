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

### One-liner (Linux & macOS)

```sh
curl -fsSL https://raw.githubusercontent.com/iftimiemarius/dispatch/main/install.sh | sh
```

This downloads the latest release for your platform, verifies the SHA-256
checksum, and installs `dispatch` to `~/.local/bin`. Install somewhere else
with `DISPATCH_INSTALL_DIR`, or pin a version with `DISPATCH_VERSION`:

```sh
curl -fsSL .../install.sh | sh -s -- --install-dir ~/bin
DISPATCH_VERSION=v0.1.0 curl -fsSL .../install.sh | sh
```

### Windows (PowerShell)

```powershell
irm https://raw.githubusercontent.com/iftimiemarius/dispatch/main/install.ps1 | iex
```

Installs `dispatch.exe` to `%LOCALAPPDATA%\dispatch` and adds it to your user
PATH. (Note: the binary is unsigned, so Windows may show a SmartScreen warning
on first run — click "More info" → "Run anyway".)

### Update

Once installed, self-update to the latest release:

```sh
dispatch upgrade          # download and install the latest version
dispatch upgrade --check  # just report whether an update is available
```

You can also re-run the install command above to reinstall.

### Build from source

Requires Go 1.23+.

```sh
git clone https://github.com/iftimiemarius/dispatch.git
cd dispatch
make install                  # builds and installs to $GOPATH/bin
make build VERSION=0.1.0      # produces ./dispatch
```

Releases are cross-compiled and published automatically via GoReleaser when a
`v*` tag is pushed.

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

#### Sync to Outlook (Microsoft Graph)

Connect your Microsoft account once, then push blocks as real calendar events:

```sh
dispatch auth login outlook      # one-time OAuth (PKCE, no client secret needed)
dispatch auth status             # see what's connected

dispatch block sync <id>         # push a block → Outlook event (creates or updates)
dispatch block sync --range week # sync all blocks in a window
dispatch block unsync <id>       # delete the Outlook event, clear the link
dispatch calendar sync --range week   # alias for syncing a window
```

Re-syncing a block updates the existing event (idempotent via the stored Graph
event id). See [Setup: Outlook](#setup-outlook) for the Azure app registration.

### GitHub

Link tasks to issues/PRs and view them. Dispatch reuses your `gh` CLI for auth
(run `gh auth login` once) — no token juggling.

```sh
# Set a project default repo, then link by number (repo auto-resolves)
dispatch project add "api" --github-repo owner/name
dispatch add "fix bug" -p api
dispatch gh link <id> "#42"

# Or link directly to any repo
dispatch gh link <id> "owner/name#42"
dispatch gh link <id> "https://github.com/owner/name/pull/42"

dispatch gh show <id>            # linked issue/PR: title, state, URL
dispatch gh prs --repo owner/name
dispatch gh issues --repo owner/name
dispatch gh unlink <id>
```

Repos resolve in order: the link ref's own repo → `--repo` flag → the task's
override (`--repo`) → the project's default (`--github-repo`). Linked tasks
show a `GH#42` badge in listings.

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

## Setup: Outlook (optional)

Outlook sync uses Microsoft Graph with OAuth (Authorization Code + PKCE), so **no client secret** is required. One-time Azure setup:

1. Azure Portal → Microsoft Entra ID → App registrations → New registration
2. Supported account type: **"Accounts in any organizational directory + Personal"** (or "Personal only")
3. **Add a platform → "Mobile and desktop applications"** → redirect URI: `http://localhost:8484/callback`
   - *(Important: use the Mobile/desktop platform, NOT "Web" — Web marks the app as a confidential client and demands a secret.)*
4. Advanced settings → **"Allow public client flows"** → **Yes** → Save
5. API permissions → add Microsoft Graph → **Delegated** → `Calendars.ReadWrite`
6. Copy the **Application (client) ID** into your config

Then create `~/.config/dispatch/config.toml`:

```toml
[outlook]
client_id     = "your-app-id-here"
tenant        = "common"      # "common" (any+personal), "consumers" (personal only), "organizations", or a tenant GUID
redirect_port = 8484          # must match the redirect URI
```

Finally connect: `dispatch auth login outlook`. Tokens are stored in your OS keyring (Secret Service / Keychain / Credential Manager) with an encrypted-file fallback.

## Setup: GitHub (optional)

GitHub features reuse the `gh` CLI — just ensure it's installed and authenticated:

```sh
gh auth login
```

That's it. Dispatch shells out to `gh` for all GitHub calls, so it inherits your existing scopes (needs `repo`). No config file entry required.

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
