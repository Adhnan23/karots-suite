-- +goose Up
-- Toggle for live web-search grounding (Gemini only). Google meters grounded
-- requests on a much smaller free quota than plain ones; turning this off lets
-- catalog identify fall back to a plain request (abundant quota) when the search
-- quota is exhausted or a live lookup isn't needed. Default on to keep behaviour.
ALTER TABLE aicatalog_settings ADD COLUMN web_search BOOLEAN NOT NULL DEFAULT true;

-- +goose Down
ALTER TABLE aicatalog_settings DROP COLUMN web_search;
