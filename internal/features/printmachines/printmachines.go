// Package printmachines lets a cashier PC's USB printers print. A small helper on
// that PC calls the main PC (nothing listens on the cashier PC, and its address may
// change), reports its printers and collects jobs. An admin approves the machine
// and picks its printers; a job from that machine's browser then goes to them.
package printmachines

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"

	"karots-suite/internal/apperr"
)

const (
	// Scheme starts a print target that goes through a machine's helper.
	Scheme = "machine://"
	// PollWait is how long a helper's call waits for a job before answering empty.
	PollWait = 20 * time.Second
	// A helper not heard from for this long is off.
	onlineWithin = 2*PollWait + 5*time.Second
	jobTimeout   = 30 * time.Second
	maxWaiting   = 20 // machines not yet approved: anyone on the network can ask
)

// Machine is a cashier PC with the helper.
type Machine struct {
	ID             int64           `db:"id"`
	TokenHash      string          `db:"token_hash"`
	Name           string          `db:"name"`
	IP             string          `db:"ip"`
	Printers       json.RawMessage `db:"printers"`
	Approved       bool            `db:"approved"`
	ReceiptPrinter string          `db:"receipt_printer"`
	LabelPrinter   string          `db:"label_printer"`
	LastSeen       time.Time       `db:"last_seen"`
}

// PrinterNames is what its helper last reported.
func (m Machine) PrinterNames() []string {
	names := []string{}
	json.Unmarshal(m.Printers, &names) //nolint:errcheck // written by Poll
	return names
}

// Online reports whether its helper has called lately.
func (m Machine) Online(now time.Time) bool { return now.Sub(m.LastSeen) < onlineWithin }

// Job is one print job on its way to a helper.
type Job struct {
	ID    string `json:"id"`
	Queue string `json:"queue"`
	Data  []byte `json:"data"`
}

type waiting struct {
	machine int64
	done    chan string // the helper's error, "" = printed
}

// Service holds the machines and the jobs in flight (jobs are memory only: one
// that can't be delivered fails at once, like a printer that is off).
type Service struct {
	db *sqlx.DB

	mu      sync.Mutex
	queues  map[int64]chan Job
	pending map[string]waiting
	byIP    map[string]Machine
	loaded  bool
}

func NewService(db *sqlx.DB) *Service {
	return &Service{db: db, queues: map[int64]chan Job{}, pending: map[string]waiting{}}
}

func hash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (s *Service) queue(id int64) chan Job {
	s.mu.Lock()
	defer s.mu.Unlock()
	q, ok := s.queues[id]
	if !ok {
		q = make(chan Job, 16)
		s.queues[id] = q
	}
	return q
}

// refresh rebuilds the address → machine map from the approved machines.
func (s *Service) refresh(ctx context.Context) error {
	var rows []Machine
	if err := s.db.SelectContext(ctx, &rows, `SELECT * FROM print_machines WHERE approved`); err != nil {
		return err
	}
	byIP := make(map[string]Machine, len(rows))
	for _, m := range rows {
		byIP[m.IP] = m
	}
	s.mu.Lock()
	s.byIP, s.loaded = byIP, true
	s.mu.Unlock()
	return nil
}

// Targets gives the print targets of the machine at this address ("" = none set).
func (s *Service) Targets(ctx context.Context, ip string) (receipt, label string) {
	s.mu.Lock()
	loaded := s.loaded
	s.mu.Unlock()
	if !loaded && s.refresh(ctx) != nil {
		return "", ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.byIP[ip]
	if !ok {
		return "", ""
	}
	return Target(m.ID, m.ReceiptPrinter), Target(m.ID, m.LabelPrinter)
}

// Target is the print target of a queue on a machine ("" for no queue).
func Target(id int64, queue string) string {
	if queue == "" {
		return ""
	}
	return Scheme + strconv.FormatInt(id, 10) + "/" + queue
}

// Poll is a helper's call: it says who it is and what it has, and waits for a job.
func (s *Service) Poll(ctx context.Context, token, ip, name string, printers []string) (approved bool, jobs []Job, err error) {
	if len(token) < 32 {
		return false, nil, apperr.Unauthorized("bad helper token")
	}
	if printers == nil {
		printers = []string{}
	}
	list, _ := json.Marshal(printers)
	var was Machine
	err = s.db.GetContext(ctx, &was, `SELECT * FROM print_machines WHERE token_hash = $1`, hash(token))
	switch {
	case errors.Is(err, sql.ErrNoRows):
		var n int
		if err := s.db.GetContext(ctx, &n, `SELECT count(*) FROM print_machines WHERE NOT approved`); err != nil {
			return false, nil, apperr.Internal("failed to read machines", err)
		}
		if n >= maxWaiting {
			return false, nil, apperr.Forbidden("too many machines are waiting for approval")
		}
	case err != nil:
		return false, nil, apperr.Internal("failed to read machines", err)
	}
	var m Machine
	if err := s.db.GetContext(ctx, &m, `INSERT INTO print_machines (token_hash, name, ip, printers) VALUES ($1, $2, $3, $4)
		ON CONFLICT (token_hash) DO UPDATE SET name = $2, ip = $3, printers = $4, last_seen = now() RETURNING *`,
		hash(token), name, ip, string(list)); err != nil {
		return false, nil, apperr.Internal("failed to save the machine", err)
	}
	if !m.Approved {
		return false, nil, nil
	}
	if was.IP != ip {
		if err := s.refresh(ctx); err != nil {
			return true, nil, apperr.Internal("failed to read machines", err)
		}
	}
	t := time.NewTimer(PollWait)
	defer t.Stop()
	select {
	case j := <-s.queue(m.ID):
		return true, []Job{j}, nil
	case <-t.C:
	case <-ctx.Done():
	}
	return true, nil, nil
}

// Done is a helper's answer for a job.
func (s *Service) Done(ctx context.Context, token, jobID, failure string) error {
	var id int64
	if err := s.db.GetContext(ctx, &id, `SELECT id FROM print_machines WHERE token_hash = $1 AND approved`, hash(token)); err != nil {
		return apperr.Unauthorized("bad helper token")
	}
	s.mu.Lock()
	w, ok := s.pending[jobID]
	if ok && w.machine == id {
		delete(s.pending, jobID)
	}
	s.mu.Unlock()
	if ok && w.machine == id {
		w.done <- failure
	}
	return nil
}

// Send is printing's way to a machine: the job goes to its helper and this waits
// for the answer, so a failure reads the same as a printer on this PC failing.
func (s *Service) Send(ctx context.Context, tgt string, data []byte) error {
	idText, queue, ok := strings.Cut(strings.TrimPrefix(tgt, Scheme), "/")
	id, err := strconv.ParseInt(idText, 10, 64)
	if !ok || err != nil {
		return fmt.Errorf("bad machine printer %q", tgt)
	}
	var m Machine
	if err := s.db.GetContext(ctx, &m, `SELECT * FROM print_machines WHERE id = $1 AND approved`, id); err != nil {
		return fmt.Errorf("that machine isn't set up for printing")
	}
	if !m.Online(time.Now()) {
		return fmt.Errorf("%s isn't connected: check that it is on and on the shop's network", m.Name)
	}
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return err
	}
	j := Job{ID: hex.EncodeToString(b[:]), Queue: queue, Data: data}
	w := waiting{machine: id, done: make(chan string, 1)}
	s.mu.Lock()
	s.pending[j.ID] = w
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.pending, j.ID)
		s.mu.Unlock()
	}()
	select {
	case s.queue(id) <- j:
	default:
		return fmt.Errorf("%s has too many jobs waiting", m.Name)
	}
	t := time.NewTimer(jobTimeout)
	defer t.Stop()
	select {
	case failure := <-w.done:
		if failure != "" {
			return fmt.Errorf("%s: %s", m.Name, failure)
		}
		return nil
	case <-t.C:
		return fmt.Errorf("%s didn't answer", m.Name)
	case <-ctx.Done():
		return ctx.Err()
	}
}

// List gives every machine, the ones waiting for approval first.
func (s *Service) List(ctx context.Context) ([]Machine, error) {
	rows := []Machine{}
	if err := s.db.SelectContext(ctx, &rows, `SELECT * FROM print_machines ORDER BY approved, name, id`); err != nil {
		return nil, apperr.Internal("failed to read machines", err)
	}
	return rows, nil
}

// Save approves a machine and sets its printers.
func (s *Service) Save(ctx context.Context, id int64, receipt, label string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE print_machines SET approved = true, receipt_printer = $2, label_printer = $3 WHERE id = $1`, id, receipt, label)
	if err != nil {
		return apperr.Internal("failed to save the machine", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return apperr.NotFound("machine")
	}
	return s.refresh(ctx)
}

// Remove forgets a machine; its helper asks again and waits for approval.
func (s *Service) Remove(ctx context.Context, id int64) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM print_machines WHERE id = $1`, id); err != nil {
		return apperr.Internal("failed to remove the machine", err)
	}
	return s.refresh(ctx)
}
