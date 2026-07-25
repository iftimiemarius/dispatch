-- Migration 002: GitHub + Outlook integration columns.
--
-- Adds:
--   projects.github_repo      — default repo for a project's tasks (nullable)
--   tasks.github_repo         — per-task repo override (nullable)
--   tasks.github_issue_number — linked issue/PR number (nullable)
--   blocks.outlook_event_id   — Microsoft Graph event id once synced (nullable)
--
-- Each statement is wrapped to tolerate already-existing columns so the
-- migration is idempotent if re-run. SQLite has no "ADD COLUMN IF NOT EXISTS",
-- so we add and ignore the "duplicate column" error at the Go layer.

ALTER TABLE projects ADD COLUMN github_repo TEXT;
ALTER TABLE tasks ADD COLUMN github_repo TEXT;
ALTER TABLE tasks ADD COLUMN github_issue_number INTEGER;
ALTER TABLE blocks ADD COLUMN outlook_event_id TEXT;
