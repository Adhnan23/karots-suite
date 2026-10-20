-- +goose Up
-- Where this PC copies its automatic backups (M8c2): the owner's copy folder
-- (USB / share) and the vendor's Karots cloud. Describes this PC, not the shop's
-- data — never in a backup, never touched by a restore (backup.skipTable).
-- preset = the BACKUP_COPY_DIR last seeded from .env, so a folder the owner
-- cleared doesn't come back on the next start.
CREATE TABLE backup_copies (
    kind           TEXT PRIMARY KEY CHECK (kind IN ('folder', 'cloud')),
    enabled        BOOLEAN NOT NULL,
    path           TEXT NOT NULL DEFAULT '',
    preset         TEXT NOT NULL DEFAULT '',
    last_copied_at TIMESTAMPTZ,
    last_file      TEXT NOT NULL DEFAULT '',
    last_error     TEXT NOT NULL DEFAULT '',
    last_tried_at  TIMESTAMPTZ
);

-- +goose Down
DROP TABLE backup_copies;
