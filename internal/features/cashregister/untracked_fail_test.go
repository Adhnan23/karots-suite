package cashregister_test

import (
	"context"
	"testing"

	"karots-suite/internal/features/cashregister"
	"karots-suite/internal/features/sales"
	"karots-suite/internal/ledgercheck/ltest"
)

// DRAW-12: when the "untracked cash" setting can't be read it counts as off, so
// cash leaving the till without a locker is refused (fails closed).
func TestDRAW12_UnreadableSettingRefusesCashOut(t *testing.T) {
	db := ltest.NewDB(t)
	ctx := context.Background()
	var uid int64
	if err := db.GetContext(ctx, &uid, `INSERT INTO users (name, phone, role, pin_hash) VALUES ('DRAW-12', '0700000912', 'cashier', 'x') RETURNING id`); err != nil {
		t.Fatal(err)
	}
	db.MustExec(`UPDATE settings SET allow_untracked_cash = true WHERE id = 1`)
	svc := cashregister.NewService(db, sales.NewService(db))
	if _, err := svc.Open(ctx, uid, cashregister.OpenInput{OpeningCash: "500"}); err != nil {
		t.Fatal(err)
	}
	// The setting is now unreadable (on in the data, but nobody can see it).
	db.MustExec(`ALTER TABLE settings RENAME COLUMN allow_untracked_cash TO allow_untracked_cash_gone`)

	if _, err := svc.Withdraw(ctx, uid, cashregister.MovementInput{Amount: "100"}); err == nil {
		t.Error("withdraw with no locker went through although the setting couldn't be read")
	}
	if _, err := svc.Close(ctx, uid, cashregister.CloseInput{ClosingCash: "500"}); err == nil {
		t.Error("close with cash and no locker went through although the setting couldn't be read")
	}
	var moved int
	if err := db.GetContext(ctx, &moved, `SELECT COUNT(*) FROM cash_movements WHERE user_id = $1 AND type <> 'opening'`, uid); err != nil {
		t.Fatal(err)
	}
	if moved != 0 {
		t.Errorf("%d cash movements written", moved)
	}
}
