// Package gen builds a demo shop's life — a year and more of it, ending now —
// through the suite's own services and API, so the data is exactly what the app
// would have written. Shop config (looks, printers, logins) is set directly.
package gen

import (
	"context"
	"net/http"
	"time"

	"github.com/jmoiron/sqlx"

	"karots-suite/internal/app"
	"karots-suite/internal/demo/shops"
)

// Deps is what a generator works through.
type Deps struct {
	DB  *sqlx.DB
	Svc *app.Services
	API http.Handler // the shop's real server, called in-process
	// The emulators' host:port, so every print lands on the paper strip.
	ReceiptPrinter, LabelPrinter string
}

// Run builds shop p as of now, reporting progress (0–100) as it goes.
func Run(ctx context.Context, d Deps, p shops.Profile, now time.Time, progress func(int, string)) error {
	progress(2, "Setting up the shop")
	if err := shopSettings(ctx, d, p); err != nil {
		return err
	}
	if err := people(ctx, d, p); err != nil {
		return err
	}
	progress(100, "Ready")
	return nil
}
