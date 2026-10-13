-- +goose Up
-- Product naming convention, applied ONLY to catalog identify/enrich prompts
-- (not Insights or category optimize). Separate from the global custom
-- instructions so a naming rule doesn't ride prompts where it's irrelevant.
ALTER TABLE aicatalog_settings ADD COLUMN naming_style TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE aicatalog_settings DROP COLUMN naming_style;
