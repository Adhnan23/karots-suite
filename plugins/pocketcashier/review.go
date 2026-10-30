package pocketcashier

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"karots-suite/internal/apperr"

	"github.com/labstack/echo/v4"
)

// What came in from each phone: the sends, newest first, and each one's actions
// with their flags — where an admin finds what to look at.

type SendRow struct {
	ID         int64  `json:"id"`
	Phone      string `json:"phone"`
	ReceivedAt string `json:"receivedAt"`
	Actions    int    `json:"actions"`
	Failed     int    `json:"failed"`
	Flagged    int    `json:"flagged"`
}

type ActionFlag struct {
	Kind string `json:"kind"`
	Note string `json:"note"`
}

type SendAction struct {
	Ref       string       `json:"ref"`
	Kind      string       `json:"kind"`
	UserName  string       `json:"userName"`
	At        string       `json:"at"`
	Status    string       `json:"status"`
	Note      string       `json:"note"`
	SaleID    int64        `json:"saleId"`
	ReceiptNo string       `json:"receiptNo"`
	Flags     []ActionFlag `json:"flags"`
}

type SendDetail struct {
	SendRow `tstype:",extends"`
	Actions []SendAction `json:"actions"`
}

const sendCols = `s.id, p.name AS phone, s.received_at, s.actions, s.failed, s.flagged FROM pocket_sends s JOIN pocket_phones p ON p.id = s.phone_id`

type sendDB struct {
	ID         int64     `db:"id"`
	Phone      string    `db:"phone"`
	ReceivedAt time.Time `db:"received_at"`
	Actions    int       `db:"actions"`
	Failed     int       `db:"failed"`
	Flagged    int       `db:"flagged"`
}

func (r sendDB) row() SendRow {
	return SendRow{ID: r.ID, Phone: r.Phone, ReceivedAt: r.ReceivedAt.UTC().Format(time.RFC3339), Actions: r.Actions, Failed: r.Failed, Flagged: r.Flagged}
}

func (h handlers) sends(c echo.Context) error {
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var rows []sendDB
	if err := h.p.core.DB.SelectContext(c.Request().Context(), &rows, `SELECT `+sendCols+` ORDER BY s.id DESC LIMIT $1`, limit); err != nil {
		return apperr.Internal("failed to list sends", err)
	}
	out := make([]SendRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.row())
	}
	return c.JSON(http.StatusOK, out)
}

func (h handlers) sendDetail(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return apperr.NotFound("send")
	}
	ctx := c.Request().Context()
	var s sendDB
	if err := h.p.core.DB.GetContext(ctx, &s, `SELECT `+sendCols+` WHERE s.id = $1`, id); err != nil {
		return apperr.NotFound("send")
	}
	var rows []struct {
		Ref       string    `db:"ref"`
		Kind      string    `db:"kind"`
		UserName  *string   `db:"user_name"`
		At        time.Time `db:"at"`
		Status    string    `db:"status"`
		Note      string    `db:"note"`
		SaleID    *int64    `db:"sale_id"`
		ReceiptNo *string   `db:"receipt_no"`
		Flags     []byte    `db:"flags"`
	}
	if err := h.p.core.DB.SelectContext(ctx, &rows, `
		SELECT a.ref, a.kind, u.name AS user_name, a.at, a.status, a.note, a.sale_id, sa.receipt_no,
		       COALESCE((SELECT json_agg(json_build_object('kind', f.kind, 'note', f.note) ORDER BY f.id) FROM sale_flags f WHERE f.sale_id = a.sale_id), '[]') AS flags
		FROM pocket_actions a LEFT JOIN users u ON u.id = a.user_id LEFT JOIN sales sa ON sa.id = a.sale_id
		WHERE a.send_id = $1 ORDER BY a.at, a.ref`, id); err != nil {
		return apperr.Internal("failed to load the send", err)
	}
	out := SendDetail{SendRow: s.row(), Actions: make([]SendAction, 0, len(rows))}
	for _, r := range rows {
		a := SendAction{Ref: r.Ref, Kind: r.Kind, At: r.At.UTC().Format(time.RFC3339), Status: r.Status, Note: r.Note, Flags: []ActionFlag{}}
		if r.UserName != nil {
			a.UserName = *r.UserName
		}
		if r.SaleID != nil {
			a.SaleID = *r.SaleID
		}
		if r.ReceiptNo != nil {
			a.ReceiptNo = *r.ReceiptNo
		}
		_ = json.Unmarshal(r.Flags, &a.Flags)
		out.Actions = append(out.Actions, a)
	}
	return c.JSON(http.StatusOK, out)
}
