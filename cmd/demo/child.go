package main

import (
	"bytes"
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
	"karots-suite/internal/backup"
	"karots-suite/internal/config"
	"karots-suite/internal/db"
	_ "karots-suite/internal/demo/allplugins"
	"karots-suite/internal/demo/gen"
	"karots-suite/internal/demo/shops"
	"karots-suite/internal/demo/templates"
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
	// Template, when set, builds the shop's past into this backup file and
	// exits (make demo) instead of serving.
	Template string `json:"template,omitempty"`
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
	if a.Template == "" { // a serving shop ends with its launcher (stdin closes)
		go func() { _, _ = io.Copy(io.Discard, os.Stdin); os.Exit(0) }()
	}

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
	cfg := &config.Config{
		Env: "production", DatabaseURL: a.DB, ServerPort: a.Port, JWTSecret: a.JWTSecret,
		JWTAccessTTL: 12 * time.Hour, JWTRefreshTTL: 7 * 24 * time.Hour, BackupInterval: 6 * time.Hour,
		CookieSecure: config.ParseCookieSecure("auto"),
	}
	validator := app.SystemPINValidator(seed)
	progress := func(pct int, step string) { say("progress %d %s", pct, step) }
	ctx := context.Background()
	past, hasPast := templates.Get(a.Shop.Key)
	if hasPast && a.Template == "" { // a built-in shop: its year comes from the binary
		progress(20, "Unpacking a year of trade")
		if _, err := backup.Restore(ctx, sqlxDB, bytes.NewReader(past)); err != nil {
			log.Printf("demo child: restoring the shop: %v", err)
			return 1
		}
	}
	if err := app.EnsureSystemAdmin(sqlxDB, installID); err != nil {
		log.Printf("demo child: system user: %v", err)
		return 1
	}
	e := app.BuildServer(sqlxDB, cfg, validator)
	deps := gen.Deps{DB: sqlxDB, Svc: app.NewServices(sqlxDB, cfg, validator), API: e,
		ReceiptPrinter: a.ReceiptPrinter, LabelPrinter: a.LabelPrinter}
	var err2 error
	switch {
	case a.Template != "":
		err2 = buildTemplate(ctx, deps, a, progress)
		if err2 == nil {
			return 0
		}
	case hasPast:
		err2 = gen.Today(ctx, deps, a.Shop, time.Now(), progress)
	default: // make-your-own: a shorter past, built live
		err2 = gen.Run(ctx, deps, a.Shop, time.Now(), 60, progress)
	}
	if err2 != nil {
		log.Printf("demo child: building the shop: %v", err2)
		return 1
	}

	sim, err := gen.NewSim(context.Background(), deps, a.Shop)
	if err != nil {
		log.Printf("demo child: the shop's own trade: %v", err)
		return 1
	}
	go sim.Run(context.Background())
	go func() {
		if err := http.ListenAndServe(fmt.Sprintf("127.0.0.1:%d", a.Control), newControl(sim)); err != nil {
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

// newControl is the child's private server for the launcher: pause the shop's
// own trade, and the cheat moves.
func newControl(sim *gen.Sim) http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("POST /pause", func(w http.ResponseWriter, r *http.Request) {
		sim.Pause(r.URL.Query().Get("on") == "true")
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	})
	m.HandleFunc("POST /cheat/{name}", func(w http.ResponseWriter, r *http.Request) {
		if err := sim.Cheat(r.PathValue("name")); err != nil {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	})
	return m
}
