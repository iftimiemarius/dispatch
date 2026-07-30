-- Migration 003: block auto-sync toggle.
--
-- Adds blocks.auto_sync (default true) so new blocks push to Outlook
-- automatically when connected. Idempotent: duplicate-column errors are
-- tolerated by the migration runner.

ALTER TABLE blocks ADD COLUMN auto_sync INTEGER NOT NULL DEFAULT 1;
