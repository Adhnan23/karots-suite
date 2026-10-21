-- +goose Up
-- Custom palettes the system user made on Appearance (docs/specs/2026-10-21-palettes-design.md §3).
-- css is the whole palette file; settings.skin = 'custom-<id>' turns one on. base = the palette it
-- was started from ("Reset to base"). Part of the shop's data, so backups carry it.
CREATE TABLE palettes (
    id         BIGSERIAL PRIMARY KEY,
    theme      TEXT NOT NULL DEFAULT 'dock',
    name       TEXT NOT NULL,
    base       TEXT NOT NULL,
    css        TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX palettes_theme_name ON palettes (theme, lower(name));

-- +goose Down
DROP TABLE palettes;
