-- Dispatch database schema (v1).
--
-- Tables: initiatives, projects, tasks, blocks.
-- Relationships are optional everywhere so a freshly captured task lives in
-- the "inbox" until triaged.

-- Schema version tracking for hand-rolled migrations.
CREATE TABLE IF NOT EXISTS schema_version (
    version    INTEGER PRIMARY KEY,
    applied_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS initiatives (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    outcome    TEXT NOT NULL DEFAULT '',
    status     TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'on_hold', 'done', 'cancelled')),
    start_at   TEXT,
    target_at  TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_initiatives_status ON initiatives(status);

CREATE TABLE IF NOT EXISTS projects (
    id           TEXT PRIMARY KEY,
    name         TEXT NOT NULL UNIQUE,
    description  TEXT NOT NULL DEFAULT '',
    status       TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'on_hold', 'done', 'archived')),
    color        TEXT NOT NULL DEFAULT '',
    initiative_id TEXT,
    github_repo  TEXT,
    created_at   TEXT NOT NULL,
    updated_at   TEXT NOT NULL,
    FOREIGN KEY (initiative_id) REFERENCES initiatives(id) ON DELETE SET NULL
);
CREATE INDEX IF NOT EXISTS idx_projects_status ON projects(status);
CREATE INDEX IF NOT EXISTS idx_projects_initiative ON projects(initiative_id);

CREATE TABLE IF NOT EXISTS tasks (
    id           TEXT PRIMARY KEY,
    title        TEXT NOT NULL,
    notes        TEXT NOT NULL DEFAULT '',
    status       TEXT NOT NULL DEFAULT 'inbox' CHECK (status IN ('inbox', 'todo', 'doing', 'done', 'blocked', 'cancelled')),
    priority     TEXT NOT NULL DEFAULT 'medium' CHECK (priority IN ('low', 'medium', 'high', 'urgent')),
    project_id   TEXT,
    initiative_id TEXT,
    tags         TEXT NOT NULL DEFAULT '',  -- comma-separated
    due_at       TEXT,
    github_repo  TEXT,
    github_issue_number INTEGER,
    created_at   TEXT NOT NULL,
    updated_at   TEXT NOT NULL,
    completed_at TEXT,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE SET NULL,
    FOREIGN KEY (initiative_id) REFERENCES initiatives(id) ON DELETE SET NULL
);
CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
CREATE INDEX IF NOT EXISTS idx_tasks_project ON tasks(project_id);
CREATE INDEX IF NOT EXISTS idx_tasks_initiative ON tasks(initiative_id);
CREATE INDEX IF NOT EXISTS idx_tasks_priority ON tasks(priority);
CREATE INDEX IF NOT EXISTS idx_tasks_due ON tasks(due_at);

CREATE TABLE IF NOT EXISTS blocks (
    id               TEXT PRIMARY KEY,
    task_id          TEXT,
    title            TEXT NOT NULL,
    notes            TEXT NOT NULL DEFAULT '',
    starts_at        TEXT NOT NULL,
    ends_at          TEXT NOT NULL,
    outlook_event_id TEXT,
    auto_sync        INTEGER NOT NULL DEFAULT 1,
    created_at       TEXT NOT NULL,
    updated_at       TEXT NOT NULL,
    FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE SET NULL
);
CREATE INDEX IF NOT EXISTS idx_blocks_task ON blocks(task_id);
CREATE INDEX IF NOT EXISTS idx_blocks_start ON blocks(starts_at);
