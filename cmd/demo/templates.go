package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"karots-suite/internal/backup"
	"karots-suite/internal/db"
	"karots-suite/internal/demo/gen"
	"karots-suite/internal/demo/shops"
	"karots-suite/internal/filedb"
)

// buildTemplate is a child in template mode: the shop's past up to yesterday,
// kept as a suite backup at a.Template.
func buildTemplate(ctx context.Context, d gen.Deps, a childArgs, progress func(int, string)) error {
	if err := gen.Past(ctx, d, a.Shop, time.Now(), 400, progress); err != nil {
		return err
	}
	f, err := os.Create(a.Template)
	if err != nil {
		return err
	}
	if err := backup.Dump(ctx, d.DB, time.Now().UTC().Format(time.RFC3339), f, backup.DumpOpts{}); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// runTemplates builds every built-in shop's past into dir (make demo), one child
// process per shop — plugins register once per process — all at once.
func runTemplates(dir string) int {
	tmp, err := os.MkdirTemp("", "karots-demo-build-*")
	if err != nil {
		log.Print(err)
		return 1
	}
	defer os.RemoveAll(tmp)
	fdb, err := filedb.Open(filedb.Options{Dir: tmp, Port: freePort()})
	if err != nil {
		log.Printf("templates: %v", err)
		return 1
	}
	defer fdb.Stop()
	pg, err := db.Connect(fdb.URL)
	if err != nil {
		log.Print(err)
		return 1
	}
	defer pg.Close()
	exe, _ := os.Executable()
	var wg sync.WaitGroup
	failed := false
	var mu sync.Mutex
	for _, p := range shops.BuiltIn() {
		wg.Add(1)
		go func(p shops.Profile) {
			defer wg.Done()
			began := time.Now()
			name := "tpl_" + p.Key
			if _, err := pg.Exec(`CREATE DATABASE ` + name); err == nil {
				_, _ = pg.Exec(`ALTER DATABASE ` + name + ` SET synchronous_commit = off`)
			}
			args, _ := json.Marshal(childArgs{DB: siblingURL(fdb.URL, name), JWTSecret: fdb.JWTSecret, Shop: p, Staff: p.Staff,
				ReceiptPrinter: "127.0.0.1:9", LabelPrinter: "127.0.0.1:9", Template: filepath.Join(dir, p.Key+".ksb")})
			cmd := exec.Command(exe, "-child", string(args))
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				mu.Lock()
				failed = true
				mu.Unlock()
				log.Printf("templates: %s failed: %v", p.Name, err)
				return
			}
			fmt.Printf("✓ %s: a year built in %s\n", p.Name, time.Since(began).Round(time.Second))
		}(p)
	}
	wg.Wait()
	if failed {
		return 1
	}
	return 0
}
