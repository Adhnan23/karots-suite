package requests

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

var (
	ErrNotFound = errors.New("request not found")
	// ErrClosed: the request is past the step asked for (asking again once the
	// item is in stock, merging or closing one that isn't open).
	ErrClosed = errors.New("request is not open")
)

type Store struct{ db *sqlx.DB }

func NewStore(db *sqlx.DB) *Store { return &Store{db: db} }

// Request is one wanted item. Its row is also its first ask (karots-pos's whole
// record), so an old backup reads as one ask with no mapping.
type Request struct {
	ID          int64     `db:"id"`
	Note        string    `db:"note"`
	Status      string    `db:"status"` // open | converted (has a product) | declined
	Reason      string    `db:"reason"`
	ProductID   *int64    `db:"product_id"`
	ProductName string    `db:"product_name"`
	Stage       string    `db:"stage"` // open | waiting | call | done — see stageSQL
	LastAsked   time.Time `db:"last_asked"`
	Asks        []Ask     `db:"-"`
}

// Ask is one customer asking. ID 0 is the request row itself.
type Ask struct {
	ID        int64      `db:"id"`
	RequestID int64      `db:"request_id"`
	Name      string     `db:"name"`
	Phone     string     `db:"phone"`
	By        string     `db:"by"`
	At        time.Time  `db:"at"`
	CalledAt  *time.Time `db:"called_at"`
}

type Counts struct {
	Open int `db:"open" json:"open"`
	Call int `db:"call" json:"call"`
	Done int `db:"done" json:"done"`
}

// toCallSQL: an ask (the row or a repeat) left a phone and hasn't been called.
const toCallSQL = `((COALESCE(r.customer_phone,'') <> '' AND r.called_at IS NULL)
	OR EXISTS (SELECT 1 FROM request_asks a WHERE a.request_id = r.id AND a.customer_phone <> '' AND a.called_at IS NULL))`

// baseSQL is every request with its stage (computed, never stored): waiting =
// has a product with no stock and someone to tell; call = that product is in.
const baseSQL = `WITH s AS (
	SELECT r.id, r.note, r.status, r.reason, r.product_id, COALESCE(p.name,'') AS product_name,
	       CASE WHEN r.status = 'open' THEN 'open'
	            WHEN r.status = 'converted' AND ` + toCallSQL + ` THEN
	                 CASE WHEN COALESCE(st.quantity,0) > 0 THEN 'call' ELSE 'waiting' END
	            ELSE 'done' END AS stage,
	       1 + (SELECT count(*) FROM request_asks a WHERE a.request_id = r.id) AS asks,
	       GREATEST(r.created_at, (SELECT max(a.created_at) FROM request_asks a WHERE a.request_id = r.id)) AS last_asked
	FROM requests r
	LEFT JOIN products p ON p.id = r.product_id
	LEFT JOIN stock st ON st.product_id = r.product_id)
`

const cols = `id, note, status, reason, product_id, product_name, stage, last_asked`

// words turns a search into ILIKE patterns, every one of which must match.
func words(q string) []string {
	var out []string
	for _, w := range strings.Fields(strings.ToLower(q)) {
		if len([]rune(w)) >= 2 {
			out = append(out, "%"+w+"%")
		}
	}
	return out
}

var tabStages = map[string][]string{"open": {"open", "waiting"}, "call": {"call"}, "done": {"done"}}

func stagesOf(tab string) []string {
	if st, ok := tabStages[tab]; ok {
		return st
	}
	return tabStages["open"]
}

// tabWhere keeps a tab's stages ($1) whose note matches every search word ($2).
const tabWhere = `WHERE stage = ANY($1) AND NOT EXISTS (SELECT 1 FROM unnest($2::text[]) w WHERE s.note NOT ILIKE w)`

// Count is how many requests List would page through.
func (s *Store) Count(ctx context.Context, tab, q string) (int, error) {
	var n int
	err := s.db.GetContext(ctx, &n, baseSQL+`SELECT count(*) FROM s `+tabWhere, pq.Array(stagesOf(tab)), pq.Array(words(q)))
	return n, err
}

// List is one tab's page, most-asked first, with every tab's count.
func (s *Store) List(ctx context.Context, tab, q string, limit, offset int) ([]Request, Counts, error) {
	var counts Counts
	if err := s.db.GetContext(ctx, &counts, baseSQL+`SELECT
		count(*) FILTER (WHERE stage IN ('open','waiting')) AS open,
		count(*) FILTER (WHERE stage = 'call') AS call,
		count(*) FILTER (WHERE stage = 'done') AS done FROM s`); err != nil {
		return nil, counts, err
	}
	rows := []Request{}
	err := s.db.SelectContext(ctx, &rows, baseSQL+`SELECT `+cols+` FROM s `+tabWhere+`
		ORDER BY asks DESC, last_asked DESC, id DESC LIMIT $3 OFFSET $4`,
		pq.Array(stagesOf(tab)), pq.Array(words(q)), limit, offset)
	if err != nil {
		return nil, counts, err
	}
	return rows, counts, s.loadAsks(ctx, rows)
}

// Similar finds what a cashier is typing among requests still worth a +1 ask
// (open, waiting) and won't-stock ones, so the till can say why.
func (s *Store) Similar(ctx context.Context, q string) ([]Request, error) {
	ws := words(q)
	rows := []Request{}
	if len(ws) == 0 {
		return rows, nil
	}
	err := s.db.SelectContext(ctx, &rows, baseSQL+`SELECT `+cols+` FROM s
		WHERE (stage IN ('open','waiting') OR status = 'declined')
		  AND NOT EXISTS (SELECT 1 FROM unnest($1::text[]) w WHERE s.note NOT ILIKE w)
		ORDER BY asks DESC, last_asked DESC LIMIT 5`, pq.Array(ws))
	if err != nil {
		return nil, err
	}
	return rows, s.loadAsks(ctx, rows)
}

func (s *Store) loadAsks(ctx context.Context, rows []Request) error {
	if len(rows) == 0 {
		return nil
	}
	ids := make([]int64, len(rows))
	at := map[int64]int{}
	for i, r := range rows {
		ids[i], at[r.ID] = r.ID, i
	}
	var asks []Ask
	err := s.db.SelectContext(ctx, &asks, `
		SELECT x.id, x.request_id, x.name, x.phone, COALESCE(u.name,'') AS by, x.at, x.called_at FROM (
			SELECT 0::bigint AS id, r.id AS request_id, COALESCE(r.customer_name,'') AS name,
			       COALESCE(r.customer_phone,'') AS phone, r.created_by, r.created_at AS at, r.called_at
			FROM requests r WHERE r.id = ANY($1)
			UNION ALL
			SELECT a.id, a.request_id, a.customer_name, a.customer_phone, a.created_by, a.created_at, a.called_at
			FROM request_asks a WHERE a.request_id = ANY($1)) x
		LEFT JOIN users u ON u.id = x.created_by
		ORDER BY x.at, x.id`, pq.Array(ids))
	for _, a := range asks {
		i := at[a.RequestID]
		rows[i].Asks = append(rows[i].Asks, a)
	}
	return err
}

func (s *Store) Add(ctx context.Context, note, name, phone string, userID int64) (int64, error) {
	var id int64
	err := s.db.GetContext(ctx, &id, `INSERT INTO requests (note, customer_name, customer_phone, created_by)
		VALUES ($1, $2, $3, NULLIF($4, 0)) RETURNING id`, note, name, phone, userID)
	return id, err
}

// stage reads one request's status and stage.
func (s *Store) stage(ctx context.Context, q sqlx.QueryerContext, id int64) (status, stage string, err error) {
	var r struct {
		Status string `db:"status"`
		Stage  string `db:"stage"`
	}
	err = sqlx.GetContext(ctx, q, &r, baseSQL+`SELECT status, stage FROM s WHERE id = $1`, id)
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrNotFound
	}
	return r.Status, r.Stage, err
}

// AddAsk is "asked again": only while the item is still wanted or declined.
func (s *Store) AddAsk(ctx context.Context, id int64, name, phone string, userID int64) error {
	status, stage, err := s.stage(ctx, s.db, id)
	if err != nil {
		return err
	}
	if stage != "open" && stage != "waiting" && status != "declined" {
		return ErrClosed
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO request_asks (request_id, customer_name, customer_phone, created_by)
		VALUES ($1, $2, $3, NULLIF($4, 0))`, id, name, phone, userID)
	return err
}

// exec runs a one-row write and turns "no row" into ErrNotFound.
func (s *Store) exec(ctx context.Context, q string, args ...any) error {
	res, err := s.db.ExecContext(ctx, q, args...)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) UpdateNote(ctx context.Context, id int64, note string) error {
	return s.exec(ctx, `UPDATE requests SET note = $1, updated_at = now() WHERE id = $2`, note, id)
}

func (s *Store) UpdateAsk(ctx context.Context, id, ask int64, name, phone string) error {
	if ask == 0 {
		return s.exec(ctx, `UPDATE requests SET customer_name = $1, customer_phone = $2, updated_at = now() WHERE id = $3`, name, phone, id)
	}
	return s.exec(ctx, `UPDATE request_asks SET customer_name = $1, customer_phone = $2 WHERE id = $3 AND request_id = $4`, name, phone, ask, id)
}

// DeleteAsk removes a repeat ask; the first ask is the request itself.
func (s *Store) DeleteAsk(ctx context.Context, id, ask int64) error {
	return s.exec(ctx, `DELETE FROM request_asks WHERE id = $1 AND request_id = $2`, ask, id)
}

func (s *Store) MarkCalled(ctx context.Context, id, ask int64) error {
	if ask == 0 {
		return s.exec(ctx, `UPDATE requests SET called_at = now(), updated_at = now() WHERE id = $1`, id)
	}
	return s.exec(ctx, `UPDATE request_asks SET called_at = now() WHERE id = $1 AND request_id = $2`, ask, id)
}

func (s *Store) Delete(ctx context.Context, id int64) error {
	return s.exec(ctx, `DELETE FROM requests WHERE id = $1`, id)
}

// close moves an open request to status, refusing one that isn't open.
func (s *Store) close(ctx context.Context, id int64, q string, args ...any) error {
	status, _, err := s.stage(ctx, s.db, id)
	if err != nil {
		return err
	}
	if status != "open" {
		return ErrClosed
	}
	return s.exec(ctx, q, args...)
}

// MarkConverted links an open request to a product, made from it or existing.
func (s *Store) MarkConverted(ctx context.Context, id, productID int64) error {
	return s.close(ctx, id, `UPDATE requests SET status = 'converted', product_id = $1, updated_at = now()
		WHERE id = $2 AND status = 'open'`, productID, id)
}

func (s *Store) Decline(ctx context.Context, id int64, reason string) error {
	return s.close(ctx, id, `UPDATE requests SET status = 'declined', reason = $1, updated_at = now()
		WHERE id = $2 AND status = 'open'`, reason, id)
}

func (s *Store) Reopen(ctx context.Context, id int64) error {
	return s.exec(ctx, `UPDATE requests SET status = 'open', product_id = NULL, reason = '', updated_at = now() WHERE id = $1`, id)
}

// Merge folds open request from into open request into: from's own ask becomes
// a repeat ask of into, its repeat asks move over, and from is removed.
func (s *Store) Merge(ctx context.Context, from, into int64) error {
	if from == into {
		return ErrClosed
	}
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	for _, id := range []int64{from, into} {
		status, _, err := s.stage(ctx, tx, id)
		if err != nil {
			return err
		}
		if status != "open" {
			return ErrClosed
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO request_asks (request_id, customer_name, customer_phone, created_by, created_at, called_at)
		SELECT $1, COALESCE(customer_name,''), COALESCE(customer_phone,''), created_by, created_at, called_at FROM requests WHERE id = $2`, into, from); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE request_asks SET request_id = $1 WHERE request_id = $2`, into, from); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM requests WHERE id = $1`, from); err != nil {
		return err
	}
	return tx.Commit()
}

// ClearDone deletes requests that have a product and nobody left to call.
// Won't-stock ones stay: they are what the till shows the next asker.
func (s *Store) ClearDone(ctx context.Context) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM requests r WHERE r.status = 'converted' AND NOT `+toCallSQL)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
