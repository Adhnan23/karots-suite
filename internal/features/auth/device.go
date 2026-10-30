package auth

import (
	"context"

	"karots-suite/internal/apperr"
)

// ClaimDevice records the device a user is signed in on now; their other
// devices are then locked out (one person, one device).
func (s *Service) ClaimDevice(ctx context.Context, userID int64, device, label string) error {
	if _, err := s.db.ExecContext(ctx, `UPDATE users SET active_device = $2, active_device_label = $3, active_device_at = now() WHERE id = $1`,
		userID, device, label); err != nil {
		return apperr.Internal("failed to record the device", err)
	}
	return nil
}

// ReleaseDevice forgets a user's active device (a lost or dead one), so any
// device may take them again.
func (s *Service) ReleaseDevice(ctx context.Context, userID int64) error {
	if _, err := s.db.ExecContext(ctx, `UPDATE users SET active_device = NULL, active_device_label = NULL, active_device_at = NULL WHERE id = $1`,
		userID); err != nil {
		return apperr.Internal("failed to release the device", err)
	}
	return nil
}

// DeviceOf is a user's active device ("" = none) and whether they are the
// hidden system user (never held to one device).
func (s *Service) DeviceOf(ctx context.Context, userID int64) (device string, system bool, err error) {
	var row struct {
		Device string `db:"device"`
		System bool   `db:"is_system"`
	}
	if err := s.db.GetContext(ctx, &row, `SELECT COALESCE(active_device, '') AS device, is_system FROM users WHERE id = $1`, userID); err != nil {
		return "", false, apperr.Internal("failed to read the device", err)
	}
	return row.Device, row.System, nil
}
