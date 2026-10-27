package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"karots-suite/internal/app"
	"karots-suite/internal/config"
	"karots-suite/internal/db"
	_ "karots-suite/internal/demo/allplugins"
	"karots-suite/internal/demo/gen"
	"karots-suite/internal/demo/shops"
	"karots-suite/internal/plugin"
)

// childArgs is how the launcher hands a shop to its child process.
type childArgs struct {
	DB             string        `json:"db"`
	Port           int           `json:"port"`
	Control        int           `json:"control"`
	JWTSecret      string        `json:"jwtSecret"`
	ReceiptPrinter string        `json:"receiptPrinter"`
	LabelPrinter   string        `json:"labelPrinter"`
	Shop           shops.Profile `json:"shop"`
	Staff          []shops.Login `json:"staff"` // Profile.Staff isn't in its JSON
}

// say is the child → launcher protocol on stdout: "progress <pct> <step>",
// then "ready" once the shop serves.
func say(format string, a ...any) { fmt.Fprintf(os.Stdout, format+"\n", a...) }

// runChild is one shop: only its plugins, migrated, its life generated through
// its own server, then served on 127.0.0.1 exactly as cmd/server serves a shop.
// It exits when the launcher goes away (stdin closes).
func runChild() int {
	var a childArgs
	if err := json.Unmarshal([]byte(flag.Arg(0)), &a); err != nil {
		log.Printf("demo child: bad arguments: %v", err)
		return 2
	}
	a.Shop.Staff = a.Staff
	go func() { _, _ = io.Copy(io.Discard, os.Stdin); os.Exit(0) }()

	keys := a.Shop.Plugins
	if keys == nil {
		keys = []string{} // nil would keep every compiled plugin
	}
	plugin.Only(keys)
	sqlxDB, err := db.Connect(a.DB)
	if err != nil {
		log.Printf("demo child: %v", err)
		return 1
	}
	defer sqlxDB.Close()
	if err := app.Migrate(sqlxDB); err != nil {
		log.Printf("demo child: migrate: %v", err)
		return 1
	}
	seed, installID, _ := demoSeed()
	if err := app.EnsureSystemAdmin(sqlxDB, installID); err != nil {
		log.Printf("demo child: system user: %v", err)
		return 1
	}
	cfg := &config.Config{
		Env: "production", DatabaseURL: a.DB, ServerPort: a.Port, JWTSecret: a.JWTSecret,
		JWTAccessTTL: 12 * time.Hour, JWTRefreshTTL: 7 * 24 * time.Hour, BackupInterval: 6 * time.Hour,
		CookieSecure: config.ParseCookieSecure("auto"),
	}
	validator := app.SystemPINValidator(seed)
	e := app.BuildServer(sqlxDB, cfg, validator)

	deps := gen.Deps{DB: sqlxDB, Svc: app.NewServices(sqlxDB, cfg, validator), API: e,
		ReceiptPrinter: a.ReceiptPrinter, LabelPrinter: a.LabelPrinter}
	if err := gen.Run(context.Background(), deps, a.Shop, time.Now(), 60, func(pct int, step string) {
		say("progress %d %s", pct, step)
	}); err != nil {
		log.Printf("demo child: building the shop: %v", err)
		return 1
	}

	ctl := newControl(deps, a.Shop)
	go func() {
		if err := http.ListenAndServe(fmt.Sprintf("127.0.0.1:%d", a.Control), ctl); err != nil {
			log.Printf("demo child: control: %v", err)
		}
	}()
	go func() {
		for i := 0; i < 100; i++ { // tell the launcher once the shop answers
			if r, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/health", a.Port)); err == nil {
				r.Body.Close()
				say("ready")
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
	}()
	if err := e.Start(fmt.Sprintf("127.0.0.1:%d", a.Port)); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Printf("demo child: %v", err)
		return 1
	}
	return 0
}

// newControl is the child's private server for the launcher: pause and cheats
// (filled in by the alive simulator, Task 8).
func newControl(d gen.Deps, p shops.Profile) http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("POST /pause", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, map[string]bool{"ok": true}) })
	return m
}
