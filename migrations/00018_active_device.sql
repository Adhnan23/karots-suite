-- +goose Up
-- One person, one device: the device a user is signed in on now (when a plugin
-- turns the rule on); their other devices are locked out until they sign in there.
ALTER TABLE users ADD COLUMN active_device text, ADD COLUMN active_device_label text, ADD COLUMN active_device_at timestamptz;

-- +goose Down
ALTER TABLE users DROP COLUMN active_device, DROP COLUMN active_device_label, DROP COLUMN active_device_at;
