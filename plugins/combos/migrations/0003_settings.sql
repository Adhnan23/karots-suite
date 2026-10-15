-- +goose Up
-- Suite: the shop's combo settings. auto_apply rings a full set in the cart as
-- the combo without the cashier tapping Apply.
CREATE TABLE combos_settings (
    id         INTEGER PRIMARY KEY DEFAULT 1,
    auto_apply BOOLEAN NOT NULL DEFAULT false,
    CONSTRAINT combos_settings_singleton CHECK (id = 1)
);
INSERT INTO combos_settings (id) VALUES (1);

-- +goose Down
DROP TABLE combos_settings;
