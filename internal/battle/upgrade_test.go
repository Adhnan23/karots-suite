package battle

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"karots-suite/internal/ledgercheck/ltest"
)

// migrationsChecked: migrations up to these were in a Release or read by hand
// for UPD-6 (2026-11-22); every newer one is held to the guard below.
var migrationsChecked = map[string]int{
	"migrations": 42, "aicatalog": 10, "alternatives": 2, "clearance": 3, "combos": 4, "documents": 9,
	"pharmacy": 9, "pocketcashier": 5, "productplus": 5, "recharge": 21, "repairs": 7, "requests": 2,
}

// stricter is what a migration may not do (UPD-6): the previous version, put
// back by BootGuard, must still read and write the migrated database.
var stricter = regexp.MustCompile(`(?i)\b(DROP\s+(COLUMN|TABLE)|RENAME|SET\s+NOT\s+NULL|ALTER\s+COLUMN\s+\S+\s+(SET\s+DATA\s+)?TYPE|ADD\s+CONSTRAINT|CREATE\s+UNIQUE)\b`)
var notNullNoDefault = regexp.MustCompile(`(?i)ADD\s+COLUMN\s+[^,;]*\bNOT\s+NULL\b`)

// UPD-6: a release's migrations only add. A migration that must do one of these
// says why the previous version is unaffected on a line "-- UPD-6: …".
func TestUPD6_MigrationsOnlyAdd(t *testing.T) {
	root := filepath.Join("..", "..")
	dirs := map[string]string{"migrations": filepath.Join(root, "migrations")}
	plugins, _ := filepath.Glob(filepath.Join(root, "plugins", "*", "migrations"))
	for _, d := range plugins {
		dirs[filepath.Base(filepath.Dir(d))] = d
	}
	num := regexp.MustCompile(`^(\d+)_`)
	for key, dir := range dirs {
		max, ok := migrationsChecked[key]
		if !ok {
			max = 0 // a new plugin: its first migrations make its own tables, the guard allows them below
		}
		files, _ := filepath.Glob(filepath.Join(dir, "*.sql"))
		for _, f := range files {
			m := num.FindStringSubmatch(filepath.Base(f))
			if m == nil {
				continue
			}
			if n, _ := strconv.Atoi(m[1]); n <= max {
				continue
			}
			b, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			src := string(b)
			if strings.Contains(src, "-- UPD-6:") {
				continue
			}
			up, _, _ := strings.Cut(src, "-- +goose Down")
			var code []string
			for _, l := range strings.Split(up, "\n") {
				if l, _, _ = strings.Cut(l, "--"); strings.TrimSpace(l) != "" {
					code = append(code, l)
				}
			}
			body := strings.Join(code, "\n")
			for _, stmt := range strings.Split(body, ";") {
				if strings.Contains(strings.ToUpper(stmt), "CREATE TABLE") {
					continue // a new table: the previous version never touches it
				}
				if s := stricter.FindString(stmt); s != "" {
					t.Errorf("UPD-6: %s does %q — the previous version may break on it after a rollback", f, s)
				}
				if s := notNullNoDefault.FindString(stmt); s != "" && !strings.Contains(strings.ToUpper(stmt), "DEFAULT") {
					t.Errorf("UPD-6: %s adds a NOT NULL column with no default — the previous version's inserts fail", f)
				}
			}
		}
	}
}

// strictReads are releases from before reads skipped unknown columns
// (db.Connect's Unsafe): any column a newer build adds makes their SELECT *
// fail, so they can't run on a newer schema at all. Once no shop runs one,
// drop it from here.
var strictReads = map[string]string{
	"48a7e04": "2026.11.17.2028 refuses unknown columns (missing destination name session_id); Iman, still onboarding (no real sales), moves past it with the next Release",
}

// UPD-6: the version each shop runs now, put back by BootGuard after a bad
// update, starts and sells on the database this build migrated. Builds each
// published commit (shops.json) in a git worktree: long, so make battle only.
func TestBattlePreviousReleaseOnNewSchema(t *testing.T) {
	t.Parallel()
	if os.Getenv("KS_BATTLE_STEPS") == "" {
		t.Skip("the previous release is built in make battle")
	}
	root, _ := filepath.Abs(filepath.Join("..", ".."))
	raw, err := os.ReadFile(filepath.Join(root, "shops.json"))
	if err != nil {
		t.Skip("no shops.json")
	}
	var shops struct {
		Shops []struct {
			Key       string `json:"key"`
			Published *struct {
				Commit, Version string
			} `json:"published"`
		} `json:"shops"`
	}
	if err := json.Unmarshal(raw, &shops); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, sh := range shops.Shops {
		if sh.Published == nil || seen[sh.Published.Commit] {
			continue
		}
		seen[sh.Published.Commit] = true
		commit, version := sh.Published.Commit, sh.Published.Version
		t.Run(version, func(t *testing.T) {
			if why, ok := strictReads[commit]; ok {
				t.Skip(why)
			}
			wt := filepath.Join(t.TempDir(), "wt")
			if out, err := exec.Command("git", "-C", root, "worktree", "add", "--detach", wt, commit).CombinedOutput(); err != nil {
				t.Fatalf("worktree: %v %s", err, out)
			}
			defer exec.Command("git", "-C", root, "worktree", "remove", "--force", wt).Run() //nolint:errcheck
			bin := filepath.Join(t.TempDir(), "karots-prev")
			build := exec.Command("go", "build", "-o", bin, "./cmd/server")
			build.Dir = wt
			if out, err := build.CombinedOutput(); err != nil {
				t.Fatalf("build %s: %v %s", commit, err, out)
			}
			s := newShop(t) // migrated by this build
			srv := startServer(t, bin, s.dsn)
			defer func() { srv.cmd.Process.Kill(); srv.cmd.Wait() }() //nolint:errcheck
			c := httpClient(t, srv.url, "0770000002")
			s.must(t, c.post("/api/v1/till/drawer/open", map[string]any{"breakdown": map[string]int{"1000": 5}}))
			for _, key := range []string{"plain", "odd", "loose", "pack"} {
				var r saleResult
				if err := c.postInto("/api/v1/till/sales", map[string]any{"saleType": "retail", "discount": "0", "discountType": "fixed",
					"items": []line{{"productId": s.prod[key], "quantity": "1"}}, "payments": []pay{cash("500")}}, &r); err != nil {
					log, _ := os.ReadFile(serverLog)
					t.Fatalf("UPD-6: %s (shop %s) can't sell %s on this build's schema: %v\n%s", version, sh.Key, key, err, tailOf(log, 1500))
				}
				var v sale
				s.must(t, c.get(fmt.Sprintf("/api/v1/till/sales/%d", r.ID), &v))
				if key == "plain" {
					s.must(t, c.post(fmt.Sprintf("/api/v1/till/sales/%d/return", r.ID), map[string]any{
						"lines": []map[string]any{{"saleItemId": v.Lines[0].ID, "quantity": "1"}}}))
				}
			}
			var st struct{ Summary struct{ Expected string } }
			s.must(t, c.get("/api/v1/till/drawer", &st))
			s.must(t, c.post("/api/v1/till/drawer/close", map[string]any{"breakdown": map[string]int{st.Summary.Expected: 1}}))
			s.audit(t)
		})
	}
}

// newShopNextSchema is a shop whose every table has columns this build doesn't
// know — what the next release's migrations may add.
func newShopNextSchema(t ltest.T) *shop {
	if testing.Short() {
		t.Skip("battle: skipped with -short")
	}
	db, dsn := ltest.NewDBWithDSN(t)
	var tables []string
	if err := db.Select(&tables, `SELECT tablename FROM pg_tables WHERE schemaname = 'public' AND tablename NOT LIKE 'goose%'`); err != nil {
		t.Fatal(err)
	}
	for _, tb := range tables {
		if _, err := db.Exec(fmt.Sprintf(`ALTER TABLE %q ADD COLUMN zz_next text DEFAULT 'next', ADD COLUMN zz_next_n int NOT NULL DEFAULT 0`, tb)); err != nil {
			t.Fatal(err)
		}
	}
	return newShopOn(t, db, dsn)
}
