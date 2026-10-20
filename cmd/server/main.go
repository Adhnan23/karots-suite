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
	"path/filepath"
	"syscall"
	"time"
	_ "time/tzdata" // embed the timezone database so local-time formatting works on any host

	"karots-suite/internal/app"
	"karots-suite/internal/backup"
	"karots-suite/internal/config"
	"karots-suite/internal/db"
	"karots-suite/internal/filedb"
	"karots-suite/internal/update"
)

func main() { os.Exit(run()) }

// run is the server; its result is the exit code (update.RestartCode after an
// update, so the supervisor starts the new binary). Returning — not os.Exit —
// lets file mode stop its PostgreSQL first.
func run() int {
	migrateOnly := flag.Bool("migrate", false, "run migrations and exit")
	seedOnly := flag.Bool("seed", false, "seed development/demo data and exit")
	demoOnly := flag.Bool("demo", false, "seed entities + sample transactions (purchases, sales, expenses…) and exit")
	initOnly := flag.Bool("init", false, "create the initial admin account for a fresh shop and exit")
	resetFlag := flag.Bool("reset", false, "drop ALL data and re-run migrations (optionally combine with -seed/-demo)")
	forceFlag := flag.Bool("force", false, "allow -reset against a production database")
	supportFor := flag.String("support-pin", "", "print the support PIN for an install id (read it off the shop's login screen) and exit")
	dataDir := flag.String("data", "", "file mode: the data folder (default: data/ next to the binary)")
	backupDir := flag.String("backup", "", "write one full backup into this folder, print \"backup: <path>\" and exit (Setup)")
	backupTag := flag.String("tag", "", "with -backup: a tag in the file name (final, before-move)")
	restoreFile := flag.String("restore", "", "migrate, bring this backup back (replaces all data), print \"restored: N rows\" and exit (Setup)")
	flag.Parse()

	// Pick up the .env shipped beside the binary before anything reads config, so
	// a shop can just run the executable. Anything already exported wins.
	config.LoadDotEnv()

	// Needs no database: the shop reads their Install ID down the phone and this
	// derives the PIN for it. The secret is only in the developer's .env (a shop's
	// .env never carries it — the Shop Builder refuses it).
	if *supportFor != "" {
		printSupportPIN(*supportFor)
		return 0
	}

	oneShot := *migrateOnly || *seedOnly || *demoOnly || *initOnly || *resetFlag || *backupDir != "" || *restoreFile != ""
	if !oneShot && bootGuard() {
		return update.RestartCode
	}

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

	if fileDB == nil && (*backupDir != "" || *restoreFile != "") {
		if err := tillStopped(); err != nil {
			log.Print(err)
			return 1
		}
	}

	cfg := config.Load()
	cfg.Version = version
	if cfg.Version == "" {
		cfg.Version = "dev"
	}
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

	// The installed program dumps its own schema: no migrations first.
	if *backupDir != "" {
		if ln != nil {
			ln.Close()
		}
		path, err := backupTo(context.Background(), sqlxDB, *backupDir, *backupTag)
		if err != nil {
			log.Printf("backup: %v", err)
			return 1
		}
		fmt.Printf("backup: %s\n", path)
		return 0
	}

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
	if ln != nil && (*migrateOnly || *resetFlag || *seedOnly || *demoOnly || *initOnly || *restoreFile != "") {
		ln.Close() // a one-shot command doesn't serve
	}
	if *restoreFile != "" {
		// A database that already holds a shop is kept first: beside the
		// automatic backups, else beside the file being brought back.
		dir := os.Getenv("BACKUP_DIR")
		if dir == "" {
			dir = filepath.Dir(*restoreFile)
		}
		if p, err := safetyCopy(context.Background(), sqlxDB, dir); err != nil {
			log.Printf("restore: couldn't keep a copy of the data already here, so nothing was changed: %v", err)
			return 1
		} else if p != "" {
			fmt.Printf("safety copy: %s\n", p)
		}
		rep, err := restoreFrom(context.Background(), sqlxDB, *restoreFile)
		if err != nil {
			log.Printf("restore: %v", err)
			return 1
		}
		printRestore(rep)
		return 0
	}
	if *migrateOnly {
		log.Println("migrations applied")
		return 0
	}
	// -reset on its own leaves a clean, empty migrated database; combined with
	// -seed/-demo it falls through to those blocks to repopulate it.
	if *resetFlag && !*seedOnly && !*demoOnly {
		log.Println("reset complete: empty migrated database")
		return 0
	}
	if *seedOnly {
		if err := seed(sqlxDB); err != nil {
			log.Fatal(err)
		}
		log.Println("seed complete")
		return 0
	}
	if *demoOnly {
		if err := demo(sqlxDB); err != nil {
			log.Fatal(err)
		}
		log.Println("demo seed complete")
		return 0
	}
	if *initOnly {
		if err := initShop(sqlxDB); err != nil {
			log.Fatal(err)
		}
		return 0
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

	restart := make(chan struct{}, 1)
	startUpdater(bgCtx, sqlxDB, cfg, restart)

	// Start with graceful shutdown.
	e.Listener = ln // file mode claimed the port already; nil = Echo listens itself
	go func() {
		addr := fmt.Sprintf(":%d", cfg.ServerPort)
		log.Printf("Karots %s listening on %s (env=%s)", cfg.Version, addr, cfg.Env)
		if err := e.Start(addr); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	code := 0
	select {
	case <-quit:
		log.Println("shutting down…")
	case <-restart:
		log.Println("update installed — restarting into the new version…")
		code = update.RestartCode
	}
	bgCancel()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := e.Shutdown(ctx); err != nil {
		log.Printf("forced shutdown: %v", err)
	}
	return code
}
