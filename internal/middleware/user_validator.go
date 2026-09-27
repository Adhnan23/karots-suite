package middleware

import (
	"context"

	"github.com/jmoiron/sqlx"
)

// DBUserValidator is the production UserValidator: one indexed lookup per
// authenticated request that rejects deleted/deactivated accounts (fails closed)
// and resolves the per-user flags guards and /me read. Shared by the server
// (internal/app) and tests so the two can never drift.
func DBUserValidator(db *sqlx.DB) UserValidator {
	return func(ctx context.Context, userID int64) (UserFlags, bool) {
		var row struct {
			Active             bool   `db:"is_active"`
			CanHandleSuppliers bool   `db:"can_handle_suppliers"`
			CanManageCredit    bool   `db:"can_manage_credit"`
			CanSeeCost         bool   `db:"can_see_cost"`
			IsSystem           bool   `db:"is_system"`
			Role               string `db:"role"`
		}
		if err := db.GetContext(ctx, &row,
			`SELECT is_active, can_handle_suppliers, can_manage_credit, can_see_cost, is_system, role FROM users WHERE id = $1`, userID); err != nil {
			return UserFlags{}, false
		}
		return UserFlags{
			CanHandleSuppliers: row.CanHandleSuppliers,
			CanManageCredit:    row.CanManageCredit,
			CanSeeCost:         row.CanSeeCost,
			// Admin or the hidden support account may escape the kiosk; a cashier never can.
			CanExitKiosk: row.IsSystem || row.Role == "admin",
			IsSystem:     row.IsSystem,
		}, row.Active
	}
}
