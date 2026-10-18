package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata" // embed the timezone database so local-time formatting works on any host

	"karots-suite/internal/app"
	"karots-suite/internal/backup"
	"karots-suite/internal/config"
	"karots-suite/internal/db"
	"karots-suite/internal/filedb"
)

func main() {
	migrateOnly := flag.Bool("migrate", false, "run migrations and exit")
	seedOnly := flag.Bool("seed", false, "seed development/demo data and exit")
	demoOnly := flag.Bool("demo", false, "seed entities + sample transactions (purchases, sales, expenses…) and exit")
	initOnly := flag.Bool("init", false, "create the initial admin account for a fresh shop and exit")
	resetFlag := flag.Bool("reset", false, "drop ALL data and re-run migrations (optionally combine with -seed/-demo)")
	forceFlag := flag.Bool("force", false, "allow -reset against a production database")
	supportFor := flag.String("support-pin", "", "print the support PIN for an install id (read it off the shop's login screen) and exit")
	dataDir := flag.String("data", "", "file mode: the data folder (default: data/ next to the binary)")
	flag.Parse()

	// Needs no database: the shop reads their Install ID down the phone and this
	// derives the PIN for it.
	if *supportFor != "" {
		printSupportPIN(*supportFor)
		return
	}

	// Pick up the .env shipped beside the binary before anything reads config, so
	// a shop can just run the executable. Anything already exported wins.
	config.LoadDotEnv()

	// File mode: no DATABASE_URL and a build that carries its own PostgreSQL.
	var fileDB *filedb.DB
	var ln net.Listener
	if os.Getenv("DATABASE_URL") == "" && filedb.Available() {
		fileDB, ln = startFileMode(*dataDir)
		defer func() {
			log.Println("file mode: stopping PostgreSQL…")
			if err := fileDB.Stop(); err != nil {
				log.Printf("stop PostgreSQL: %v", err)
			}
		}()
	}

	cfg := config.Load()
	if fileDB != nil {
		cfg.FileDataDir = fileDB.Dir
		log.Printf("file mode: data in %s", fileDB.Dir)
	} else {
		log.Printf("server mode (DATABASE_URL)")
	}

	sqlxDB, err := db.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer sqlxDB.Close()

	// Destructive wipe: drop the schema BEFORE migrating so the migration runner
	// rebuilds it (and its seeded reference data) from scratch.
	if *resetFlag {
		if cfg.IsProd() && !*forceFlag {
			log.Fatal("refusing -reset on a production database; pass -force to override")
		}
		log.Println("reset: dropping all data…")
		if err := resetSchema(sqlxDB); err != nil {
			log.Fatal(err)
		}
	}

	// Core migrations then each enabled plugin's own set (idempotent late-enable
	// path). Runs on every boot, including -migrate and after -reset.
	if err := app.Migrate(sqlxDB); err != nil {
		log.Fatal(err)
	}
	if ln != nil && (*migrateOnly || *resetFlag || *seedOnly || *demoOnly || *initOnly) {
		ln.Close() // a one-shot command doesn't serve
	}
	if *migrateOnly {
		log.Println("migrations applied")
		return
	}
	// -reset on its own leaves a clean, empty migrated database; combined with
	// -seed/-demo it falls through to those blocks to repopulate it.
	if *resetFlag && !*seedOnly && !*demoOnly {
		log.Println("reset complete: empty migrated database")
		return
	}
	if *seedOnly {
		if err := seed(sqlxDB); err != nil {
			log.Fatal(err)
		}
		log.Println("seed complete")
		return
	}
	if *demoOnly {
		if err := demo(sqlxDB); err != nil {
			log.Fatal(err)
		}
		log.Println("demo seed complete")
		return
	}
	if *initOnly {
		if err := initShop(sqlxDB); err != nil {
			log.Fatal(err)
		}
		return
	}

	// Hidden developer-only recovery admin — re-ensured on every boot so the
	// install can always be unlocked. Invisible to the shop.
	if err := ensureSystemAdmin(sqlxDB); err != nil {
		log.Fatal(err)
	}

	// The hidden System account's PIN rotates hourly, so it is validated by a
	// time-based check rather than a stored hash. Resolve this build's per-shop
	// seed once; the derived validator is handed to the shared server assembly.
	supportSeed, supportSource, err := resolveSupportSeed(sqlxDB)
	if err != nil {
		log.Fatalf("support seed: %v", err)
	}
	if supportSource == "" {
		log.Println("SECURITY: this build has no support seed of its own, so its support " +
			"PIN is the fixed fallback 2273. Build shop binaries with `make bootstrap` " +
			"(POS_SUPPORT_SECRET set) to give each shop its own rotating PIN.")
	} else {
		log.Printf("system admin: support PIN rotates hourly, %s", supportSource)
	}
	log.Printf("server time is %s (the support PIN is derived from this clock)",
		time.Now().UTC().Format(time.RFC3339))

	// Assemble the fully-wired server (static, health, every feature API with its
	// core seams, and the HTMX/Templ UI) — shared with cmd/demo via internal/app.
	e := app.BuildServer(sqlxDB, cfg, systemPINValidator(supportSeed))

	// Background context cancelled on shutdown so long-lived workers stop cleanly.
	bgCtx, bgCancel := context.WithCancel(context.Background())
	defer bgCancel()

	// Automatic time-based backups (in-process, same pure-Go path as the manual
	// Settings backup). Disabled unless BACKUP_DIR is set.
	if cfg.BackupDir != "" {
		log.Printf("auto-backup: enabled (dir=%s, every %s, keep %d)", cfg.BackupDir, cfg.BackupInterval, cfg.BackupKeep)
		go backup.RunScheduler(bgCtx, sqlxDB, cfg.BackupDir, cfg.BackupInterval, cfg.BackupKeep)
	} else {
		log.Printf("auto-backup: disabled (set BACKUP_DIR to enable)")
	}

	// Start with graceful shutdown.
	e.Listener = ln // file mode claimed the port already; nil = Echo listens itself
	go func() {
		addr := fmt.Sprintf(":%d", cfg.ServerPort)
		log.Printf("POS server listening on %s (env=%s)", addr, cfg.Env)
		if err := e.Start(addr); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("shutting down…")
	bgCancel()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := e.Shutdown(ctx); err != nil {
		log.Printf("forced shutdown: %v", err)
	}
}
