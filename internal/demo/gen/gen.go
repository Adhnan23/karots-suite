// Package gen builds a demo shop's life — a year and more of it, ending now —
// through the suite's own services and API, so the data is exactly what the app
// would have written. Shop config (looks, printers, logins) is set directly.
//
// A full year takes minutes, so `make demo` builds each built-in shop's past
// once (Past) and keeps it as a suite backup; at launch the backup is restored
// and Today moves it forward to end yesterday and trades today. A make-your-own
// shop has no backup: Run builds a shorter past live.
package gen

import (
	"context"
	"fmt"
	"hash/fnv"
	"math/rand/v2"
	"net/http"
	"time"

	"github.com/jmoiron/sqlx"

	"karots-suite/internal/app"
	"karots-suite/internal/demo/shops"
	"karots-suite/internal/features/reorder"
)

// Deps is what a generator works through.
type Deps struct {
	DB  *sqlx.DB
	Svc *app.Services
	API http.Handler // the shop's real server, called in-process
	// The emulators' host:port, so every print lands on the paper strip.
	ReceiptPrinter, LabelPrinter string
	// OnDay, when set, hears each day of the past as it's finished (the
	// time machine on the building screens).
	OnDay func(Day)
}

// Day is one finished day of a shop's past: its till sales and their total,
// day N of the Of being built.
type Day struct {
	Date    string `json:"date"` // 2006-01-02
	Sales   int    `json:"sales"`
	Takings string `json:"takings"`
	N       int    `json:"n"`
	Of      int    `json:"of"`
}

func newGen(ctx context.Context, d Deps, p shops.Profile, now time.Time) (*gen, error) {
	tm, err := newTimeMachine(ctx, d.DB)
	if err != nil {
		return nil, err
	}
	return &gen{d: d, r: rand.New(rand.NewPCG(seedOf(p.Key), uint64(now.YearDay()))), tm: tm, stock: map[int64]float64{}, owed: map[int][]due{}, tills: map[int64]*Client{}, ctx: ctx}, nil
}

// Past builds shop p from its setup to the day before asOf: days of trade.
func Past(ctx context.Context, d Deps, p shops.Profile, asOf time.Time, days int, progress func(int, string)) error {
	g, err := newGen(ctx, d, p, asOf)
	if err != nil {
		return err
	}
	progress(2, "Setting up the shop")
	if err := g.tm.begin(ctx); err != nil { // set up just before its first day of trade
		return err
	}
	g.tm.at(at(localDay(asOf).AddDate(0, 0, -days-1), 10))
	if err := shopSettings(ctx, d, p); err != nil {
		return err
	}
	if err := people(ctx, d, p); err != nil {
		return err
	}
	if g.s, err = catalogue(ctx, d, p); err != nil {
		return err
	}
	if g.admin, err = NewClient(d.API, p.Logins[0].Phone, p.Logins[0].PIN, "demo-back-office"); err != nil {
		return err
	}
	if err := g.floorSetup(); err != nil {
		return err
	}
	if err := g.pluginsSetup(); err != nil {
		return err
	}
	if err := g.tm.shift(ctx); err != nil {
		return err
	}
	return g.history(asOf, days, progress)
}

// Today brings a shop whose past is built up to now: its whole past moves
// forward to end yesterday, the printers point at this run's emulators, today
// trades up to now with the tills open, and reorder levels are learned.
func Today(ctx context.Context, d Deps, p shops.Profile, now time.Time, progress func(int, string)) error {
	g, err := newGen(ctx, d, p, now)
	if err != nil {
		return err
	}
	if g.s, err = loadShop(ctx, d, p); err != nil {
		return err
	}
	progress(85, "Bringing it up to today")
	if err := g.tm.moveAll(ctx, now); err != nil {
		return err
	}
	if err := shopSettings(ctx, d, p); err != nil { // this run's printers
		return err
	}
	if err := g.loadStock(); err != nil {
		return err
	}
	if g.admin, err = NewClient(d.API, p.Logins[0].Phone, p.Logins[0].PIN, "demo-back-office"); err != nil {
		return err
	}
	if err := g.today(now); err != nil {
		return err
	}
	progress(95, "Learning reorder levels")
	if _, err := reorder.Run(ctx, d.DB, now); err != nil {
		return fmt.Errorf("reorder levels: %w", err)
	}
	_, _ = d.DB.ExecContext(ctx, `DROP TABLE IF EXISTS demo_anchors`)
	progress(100, "Ready")
	return nil
}

// Run builds shop p live: days of past, then today.
func Run(ctx context.Context, d Deps, p shops.Profile, now time.Time, days int, progress func(int, string)) error {
	if err := Past(ctx, d, p, now, days, progress); err != nil {
		return err
	}
	return Today(ctx, d, p, now, progress)
}

func seedOf(key string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(key))
	return h.Sum64()
}
