package main

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/jmoiron/sqlx"

	"karots-suite/internal/backup"
	"karots-suite/internal/config"
	"karots-suite/internal/update"
)

// Stamped by the Shop Builder (-X main.…). A build without a channel (dev,
// demo, Docker, `make build`) has no updater; the Data page shows the version only.
var (
	version       = ""
	buildKind     = "" // "test" | "release"
	updateBaseURL = ""
	updateChannel = ""
	updatePubKey  = ""
)

// selfUpdate: a shop build updates itself unless SELF_UPDATE=off — a Docker /
// VPS container, which is updated by rebuilding its image (deploy/docker).
func selfUpdate() bool {
	return updateChannel != "" && updateBaseURL != "" && os.Getenv("SELF_UPDATE") != "off"
}

// exePath is the running binary, symlinks resolved — what an update swaps.
func exePath() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if r, err := filepath.EvalSymlinks(exe); err == nil {
		return r
	}
	return exe
}

// bootGuard runs before anything that can crash a fresh update (migrations).
// It reports true when it put the previous binary back: the caller exits with
// update.RestartCode so the supervisor starts that one.
func bootGuard() bool {
	if !selfUpdate() {
		return false
	}
	rolledBack, err := update.BootGuard(exePath())
	if err != nil {
		log.Printf("update: boot guard: %v", err)
	}
	if rolledBack {
		log.Printf("update: the new version couldn't stay up — went back to the previous one, restarting")
	}
	return rolledBack
}

// startUpdater turns self-update on for a shop build: checks at start and every
// 6 h, and marks the binary healthy once it has stayed up 2 minutes. Restart
// sends on restart; main then shuts down and exits with update.RestartCode.
func startUpdater(ctx context.Context, db *sqlx.DB, cfg *config.Config, restart chan<- struct{}) *update.Updater {
	if !selfUpdate() {
		return nil
	}
	pub, err := base64.StdEncoding.DecodeString(updatePubKey)
	if err != nil || len(pub) != ed25519.PublicKeySize {
		log.Printf("update: off — this build's release key is missing or damaged")
		return nil
	}
	exe := exePath()
	u := &update.Updater{
		Current: cfg.Version, Kind: buildKind, BaseURL: updateBaseURL, Channel: updateChannel,
		PubKey: ed25519.PublicKey(pub), Exe: exe,
		Backup: func(ctx context.Context, v string) error {
			_, err := backup.WriteSnapshot(ctx, db, cfg.BackupFolder(), time.Now(), "before-update-"+v)
			return err
		},
		Restart: func() {
			select {
			case restart <- struct{}{}:
			default:
			}
		},
	}
	update.Enable(u)
	go u.Run(ctx, 6*time.Hour)
	time.AfterFunc(2*time.Minute, func() { update.MarkHealthy(exe, cfg.Version) })
	log.Printf("update: on (%s build %s)", buildKind, cfg.Version)
	return u
}
