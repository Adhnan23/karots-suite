package cashflow

import (
	"context"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/cashregister"
	"karots-suite/internal/features/lockers"
	"karots-suite/internal/features/sales"
	"karots-suite/internal/money"

	"github.com/jmoiron/sqlx"
)

// SaleNonCashLeg books a card/online sale tender into its money account (a card-
// kind locker for card, a bank-kind locker for online), on the sale's own tx. It
// is injected into sales via WithNonCashLeg so that package never imports
// cashflow — the same DI shape as TillLockerLeg. One External→account move per
// tender line writes a locker_ledger 'payment' row and one CR- money receipt,
// linked to the sale. LockerKind is 'payment' (deliberately never 'interest',
// which the P&L reads as other income), so no report double-counts.
//
// The account's kind must match the method; this is the trust boundary that
// stops a forged account_id from routing money into an arbitrary locker.
func (s *Service) SaleNonCashLeg(ctx context.Context, tx *sqlx.Tx, ev sales.NonCashPayment) error {
	dest, err := AccountLocationTx(ctx, tx, ev.AccountID, ev.Method)
	if err != nil {
		return err
	}
	party := ev.Party
	if party == "" {
		party = "Customer"
	}
	_, err = s.MoveTx(ctx, tx, MoveInput{
		From:        External(),
		To:          dest,
		Amount:      ev.Amount,
		Reason:      ev.Method + " sale",
		ReceiptKind: "payment",
		LockerKind:  "payment",
		Party:       party,
		Ref:         &Ref{Kind: "sale", ID: ev.SaleID},
		ActorID:     ev.ActorID,
	})
	return err
}

// NonCashAccountKind is the locker kind a non-cash payment method must land in:
// card takings in a card-machine clearing account, online transfers in a bank.
func NonCashAccountKind(method string) string {
	if method == "card" {
		return lockers.KindCard
	}
	return lockers.KindBank
}

// AccountLocationTx validates that accountID is an active locker of the kind the
// method requires (card→card, online→bank) and returns its Location. This is the
// single trust boundary for routing any non-cash payment (sale or credit
// collection) into an account — call it inside the caller's transaction so a
// forged account id cannot slip money into an arbitrary locker.
func AccountLocationTx(ctx context.Context, tx *sqlx.Tx, accountID int64, method string) (Location, error) {
	wantKind := NonCashAccountKind(method)
	acct, err := lockers.NewRepository(tx).GetForUpdate(ctx, accountID)
	if err != nil {
		return Location{}, apperr.NotFound("money account")
	}
	if !acct.IsActive || acct.Kind != wantKind {
		return Location{}, apperr.Validation("pick a valid " + wantKind + " account for a " + method + " payment")
	}
	return Locker(accountID), nil
}

// TillLockerLeg books the locker side (+ a CR- receipt) of a till cash event,
// inside the cashregister tx. It is injected into cashregister via WithLockerLeg
// so that package never imports cashflow. The till side is handled by the
// cashregister itself (opening_cash, closing, or the drawer movement), so this
// only writes the locker ledger row and the receipt.
//
// Direction (delta on the locker):
//   - open  / payin    : cash came FROM the locker → locker decreases (guarded)
//   - close / withdraw : cash went   TO the locker → locker increases
func (s *Service) TillLockerLeg(ctx context.Context, tx *sqlx.Tx, ev cashregister.TillCashEvent) (int64, error) {
	lrepo := lockers.NewRepository(tx)
	out := ev.Kind == "open" || ev.Kind == "payin" // money leaving the counterparty

	// The counterparty label is the locker's name when one is involved, else a
	// generic cash label describing the drawer event (opening float, plain
	// pay-in / withdrawal). A locker-less move writes only the receipt — the till
	// side is already booked by the cashregister (opening_cash / drawer movement).
	counterLabel := tillCashLabel(ev.Kind)
	if ev.LockerID > 0 {
		l, err := lrepo.GetForUpdate(ctx, ev.LockerID)
		if err != nil {
			return 0, apperr.NotFound("locker")
		}
		if !lockers.HoldsCash(l.Kind) {
			return 0, apperr.Validation("till cash can only go into or come from a cash locker, not " + l.Name)
		}
		if out && !l.AllowNegative && ev.Amount.GreaterThan(l.Balance) {
			return 0, apperr.Conflict(l.Name + " only has " + money.Display(l.Balance) + " available")
		}
		delta := ev.Amount
		if out {
			delta = ev.Amount.Neg()
		}
		sessID := ev.SessionID
		counter := "till"
		if _, err := lrepo.AddEntry(ctx, lockers.LedgerInput{
			LockerID: ev.LockerID, BalanceDelta: delta, Kind: "transfer",
			Counterparty: &counter, CounterTillSession: &sessID, Note: ev.Reason,
			CreatedBy: actorPtr(ev.UserID),
		}); err != nil {
			return 0, apperr.Internal("failed to record locker movement", err)
		}
		counterLabel = l.Name
	}

	tillLabel := "Till"
	var name string
	if err := tx.GetContext(ctx, &name, `SELECT name FROM users WHERE id = $1`, ev.UserID); err == nil && name != "" {
		tillLabel = "Till - " + name // plain hyphen: the thermal font has no em-dash glyph
	}
	from, to := tillLabel, counterLabel // money into the counterparty (close / withdraw)
	if out {
		from, to = counterLabel, tillLabel
	}
	rec, err := NewReceiptRepository(tx).Insert(ctx, ReceiptInput{
		Kind: "transfer", FromLabel: from, ToLabel: to, Amount: ev.Amount,
		Note: ev.Reason, CreatedBy: actorPtr(ev.UserID),
	})
	if err != nil {
		return 0, apperr.Internal("failed to record money receipt", err)
	}
	return rec.ID, nil
}

// tillCashLabel is the counterparty label shown on the receipt for a locker-less
// till cash event (no vault/locker chosen).
func tillCashLabel(kind string) string {
	switch kind {
	case "open":
		return "Opening cash"
	case "payin":
		return "Cash in"
	case "close":
		return "Cash counted"
	case "withdraw":
		return "Cash out"
	}
	return "Cash"
}

func actorPtr(id int64) *int64 {
	if id <= 0 {
		return nil
	}
	return &id
}
