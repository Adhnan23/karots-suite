package catalogue

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	appdb "karots-suite/internal/db"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// What the Catalogue app publishes (the catalogue repo's README: index.json, v<N>.json.gz, c<N>.json.gz).
type (
	Field struct {
		Name    string   `json:"name"`
		Type    string   `json:"type"`
		Options []string `json:"options,omitempty"`
		Hint    string   `json:"hint,omitempty"`
	}
	fileRef struct {
		File   string `json:"file"`
		Sha256 string `json:"sha256"`
	}
	changeRef struct {
		To     int    `json:"to"`
		File   string `json:"file"`
		Sha256 string `json:"sha256"`
	}
	index struct {
		Version int         `json:"version"`
		Count   int         `json:"count"`
		Fields  []Field     `json:"fields"`
		Full    fileRef     `json:"full"`
		Changes []changeRef `json:"changes"`
	}
	Item struct {
		ID     int64             `json:"id"`
		Name   string            `json:"name"`
		Unit   string            `json:"unit"`
		Kinds  string            `json:"kinds"`
		Fields map[string]string `json:"fields"`
	}
	gone struct {
		ID   int64 `json:"id"`
		Into int64 `json:"into,omitempty"`
	}
	version struct {
		Version  int     `json:"version"`
		Fields   []Field `json:"fields"`
		Products []Item  `json:"products"`
	}
	changes struct {
		From    int     `json:"from"`
		To      int     `json:"to"`
		Added   []Item  `json:"added"`
		Changed []Item  `json:"changed"`
		Gone    []gone  `json:"gone"`
		Fields  []Field `json:"fields,omitempty"`
	}
)

// Syncer brings the shop's copy up to the published version (CTL-2, CTL-3).
type Syncer struct {
	db   *sqlx.DB
	base string // the published folder; "" = no address in this build
	http *http.Client
	// afterChange runs inside the sync's transaction with what changed, for the links' review step (CTL-8, 12).
	afterChange func(ctx context.Context, tx *sqlx.Tx, changed map[int64]Item, gone []gone) error
}

func NewSyncer(db *sqlx.DB, base string) *Syncer {
	return &Syncer{db: db, base: strings.TrimRight(base, "/"), http: &http.Client{Timeout: 2 * time.Minute}}
}

var errNoAddress = errors.New("no catalogue address in this build")

// Sync checks for a newer version and applies it. The copy changes only when everything downloaded and checked
// out, in one transaction; a failure is stored on the state (shown on the page) and the copy stays as it was.
func (s *Syncer) Sync(ctx context.Context) (int, error) {
	v, err := s.sync(ctx)
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	_, _ = s.db.ExecContext(ctx, `UPDATE catalogue_state SET checked_at = now(), error = $1 WHERE id = 1`, msg)
	return v, err
}

func (s *Syncer) sync(ctx context.Context) (int, error) {
	if s.base == "" {
		return 0, errNoAddress
	}
	var idx index
	if err := s.getJSON(ctx, "index.json", "", false, &idx); err != nil {
		return 0, fmt.Errorf("reading the catalogue's index: %w", err)
	}
	var have int
	if err := s.db.GetContext(ctx, &have, `SELECT version FROM catalogue_state WHERE id = 1`); err != nil {
		return 0, err
	}
	switch {
	case idx.Version == have:
		return have, nil
	case idx.Version < have: // only forward (CTL-3): an older index is a mistake somewhere, never a reason to go back
		return have, fmt.Errorf("the published catalogue (version %d) is older than this copy (%d)", idx.Version, have)
	}
	// The change files after our version, in order; any gap (or no copy yet) means the whole version.
	var steps []changes
	byTo := map[int]changeRef{}
	for _, c := range idx.Changes {
		byTo[c.To] = c
	}
	whole := have == 0
	for v := have + 1; !whole && v <= idx.Version; v++ {
		ref, ok := byTo[v]
		if !ok {
			whole = true
			break
		}
		var c changes
		if err := s.getJSON(ctx, ref.File, ref.Sha256, true, &c); errors.Is(err, errNotThere) {
			whole = true // listed but gone: the whole version instead (CTL-3)
			break
		} else if err != nil {
			return have, fmt.Errorf("reading %s: %w", ref.File, err)
		}
		if c.From != v-1 || c.To != v {
			return have, fmt.Errorf("%s is from %d to %d, not %d to %d", ref.File, c.From, c.To, v-1, v)
		}
		steps = append(steps, c)
	}
	var full version
	if whole {
		if err := s.getJSON(ctx, idx.Full.File, idx.Full.Sha256, true, &full); err != nil {
			return have, fmt.Errorf("reading %s: %w", idx.Full.File, err)
		}
		if full.Version != idx.Version || len(full.Products) != idx.Count {
			return have, fmt.Errorf("%s is version %d with %d products, the index says %d with %d", idx.Full.File, full.Version, len(full.Products), idx.Version, idx.Count)
		}
	}
	err := appdb.WithTx(ctx, s.db, func(tx *sqlx.Tx) error {
		// One sync at a time; one that waited finds the work done.
		if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext('catalogue_sync'))`); err != nil {
			return err
		}
		var now int
		if err := tx.GetContext(ctx, &now, `SELECT version FROM catalogue_state WHERE id = 1`); err != nil {
			return err
		}
		if now != have {
			return errRaced
		}
		changed, goneAll := map[int64]Item{}, []gone{}
		if whole {
			old, err := loadItems(ctx, tx)
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `DELETE FROM catalogue_items`); err != nil {
				return err
			}
			if err := putItems(ctx, tx, full.Products); err != nil {
				return err
			}
			seen := map[int64]bool{}
			for _, it := range full.Products {
				seen[it.ID] = true
				if o, ok := old[it.ID]; ok && !sameItem(o, it) {
					changed[it.ID] = it
				}
			}
			for id := range old {
				if !seen[id] {
					goneAll = append(goneAll, gone{ID: id})
				}
			}
		} else {
			for _, c := range steps {
				if err := putItems(ctx, tx, append(append([]Item{}, c.Added...), c.Changed...)); err != nil {
					return err
				}
				ids := make([]int64, len(c.Gone))
				for i, g := range c.Gone {
					ids[i] = g.ID
					delete(changed, g.ID)
				}
				if _, err := tx.ExecContext(ctx, `DELETE FROM catalogue_items WHERE id = ANY($1)`, pq.Array(ids)); err != nil {
					return err
				}
				for _, it := range c.Changed {
					changed[it.ID] = it
				}
				goneAll = append(goneAll, c.Gone...)
			}
		}
		if s.afterChange != nil {
			if err := s.afterChange(ctx, tx, changed, goneAll); err != nil {
				return err
			}
		}
		fields, _ := json.Marshal(idx.Fields)
		_, err := tx.ExecContext(ctx, `UPDATE catalogue_state SET version = $1, fields = $2, synced_at = now() WHERE id = 1`, idx.Version, fields)
		return err
	})
	if errors.Is(err, errRaced) {
		return idx.Version, nil
	}
	if err != nil {
		return have, err
	}
	return idx.Version, nil
}

var (
	errRaced    = errors.New("another sync moved the copy")
	errNotThere = errors.New("not found")
)

// getJSON downloads a published file, checks its SHA-256 (when given) and decodes it.
func (s *Syncer) getJSON(ctx context.Context, name, sha string, gz bool, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.base+"/"+name, nil)
	if err != nil {
		return err
	}
	res, err := s.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return errNotThere
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("%s", res.Status)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, 200<<20))
	if err != nil {
		return err
	}
	if sha != "" {
		if sum := sha256.Sum256(body); hex.EncodeToString(sum[:]) != sha {
			return errors.New("the file doesn't match its checksum")
		}
	}
	if gz {
		r, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			return err
		}
		if body, err = io.ReadAll(io.LimitReader(r, 1<<30)); err != nil {
			return err
		}
	}
	return json.Unmarshal(body, out)
}

// searchText is what search looks in: the name and every field value, lower-case.
func searchText(it Item) string {
	parts := []string{it.Name}
	for _, v := range it.Fields {
		parts = append(parts, v)
	}
	return strings.ToLower(strings.Join(parts, " "))
}

// putItems inserts or replaces items, a few thousand at a time.
func putItems(ctx context.Context, tx *sqlx.Tx, items []Item) error {
	for len(items) > 0 {
		n := min(len(items), 5000)
		ids, names, units, kinds, fields, search := make([]int64, n), make([]string, n), make([]string, n), make([]string, n), make([]string, n), make([]string, n)
		for i, it := range items[:n] {
			f, _ := json.Marshal(it.Fields)
			if it.Fields == nil {
				f = []byte("{}")
			}
			ids[i], names[i], units[i], kinds[i], fields[i], search[i] = it.ID, it.Name, it.Unit, it.Kinds, string(f), searchText(it)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO catalogue_items (id, name, unit, kinds, fields, search)
			SELECT * FROM unnest($1::bigint[], $2::text[], $3::text[], $4::text[], $5::jsonb[], $6::text[])
			ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, unit = EXCLUDED.unit, kinds = EXCLUDED.kinds,
				fields = EXCLUDED.fields, search = EXCLUDED.search`,
			pq.Array(ids), pq.Array(names), pq.Array(units), pq.Array(kinds), pq.Array(fields), pq.Array(search)); err != nil {
			return err
		}
		items = items[n:]
	}
	return nil
}

type itemRow struct {
	ID     int64  `db:"id"`
	Name   string `db:"name"`
	Unit   string `db:"unit"`
	Kinds  string `db:"kinds"`
	Fields []byte `db:"fields"`
}

func (r itemRow) item() Item {
	it := Item{ID: r.ID, Name: r.Name, Unit: r.Unit, Kinds: r.Kinds}
	_ = json.Unmarshal(r.Fields, &it.Fields)
	return it
}

func loadItems(ctx context.Context, q sqlx.QueryerContext) (map[int64]Item, error) {
	var rows []itemRow
	if err := sqlx.SelectContext(ctx, q, &rows, `SELECT id, name, unit, kinds, fields FROM catalogue_items`); err != nil {
		return nil, err
	}
	out := make(map[int64]Item, len(rows))
	for _, r := range rows {
		out[r.ID] = r.item()
	}
	return out, nil
}

func sameItem(a, b Item) bool {
	if a.Name != b.Name || a.Unit != b.Unit || a.Kinds != b.Kinds || len(a.Fields) != len(b.Fields) {
		return false
	}
	for k, v := range a.Fields {
		if b.Fields[k] != v {
			return false
		}
	}
	return true
}
