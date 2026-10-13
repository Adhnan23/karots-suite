-- +goose Up
-- Standing custom instructions the owner types once and that ride along with
-- every AI prompt (manual and API mode) — e.g. "list products in this order" or
-- a preferred JSON field ordering. Appended to the system prompt of every flow.
ALTER TABLE aicatalog_settings ADD COLUMN custom_prompt TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE aicatalog_settings DROP COLUMN custom_prompt;
