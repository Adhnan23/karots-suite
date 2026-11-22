package battle

import (
	"context"
	"fmt"
	"math/rand/v2"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"karots-suite/internal/backup"
	"karots-suite/internal/ledgercheck/ltest"
)

// Power cuts while the shop's data is being saved or brought back: the real
// binary killed part way through Setup's -backup, an automatic backup and
// Setup's -restore. No *.json.gz is ever left that won't restore (BAK-3), and a
// restore is all or nothing (BAK-5).
func TestBattlePowerCutBackups(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	ctx := context.Background()
	s.open(t, s.till1)
	for range 10 {
		s.sell(t, s.till1, "0", []line{{"productId": s.prod["plain"], "quantity": "1"}}, cash("100"))
	}
	// Enough rows that a backup or a restore takes a while to cut into.
	s.must(t, sqlxExec(ctx, s.db, `INSERT INTO audit_log (action, entity, detail)
		SELECT 'bulk', 'battle', repeat('x', 200) FROM generate_series(1, 150000)`))
	bin := serverBinary(t)
	dir := t.TempDir()
	envFor := func(extra ...string) []string {
		l, err := net.Listen("tcp", "127.0.0.1:0")
		s.must(t, err)
		port := l.Addr().(*net.TCPAddr).Port
		l.Close()
		return append(os.Environ(), append([]string{"DATABASE_URL=" + s.dsn, fmt.Sprintf("SERVER_PORT=%d", port),
			"APP_ENV=development", "JWT_SECRET=battle-battle-battle-battle-battle-battle", "KAROTS_NO_SPOOL=1",
			"BACKUP_DIR=" + dir, "BACKUP_INTERVAL=6h", "CLOUD_BACKUP=", "BACKUP_COPY_DIR="}, extra...)...)
	}
	// cut runs the binary and kills it after d (it may finish first).
	cut := func(d time.Duration, env []string, args ...string) {
		cmd := exec.Command(bin, args...)
		cmd.Env = env
		s.must(t, cmd.Start())
		done := make(chan struct{})
		go func() { cmd.Wait(); close(done) }() //nolint:errcheck
		select {
		case <-done:
		case <-time.After(d):
			cmd.Process.Kill() //nolint:errcheck
			<-done
		}
		s.quiet(t)
	}
	checked := map[string]bool{}
	allRestore := func(when string) {
		files, _ := filepath.Glob(filepath.Join(dir, "*.json.gz"))
		for _, f := range files {
			if checked[f] {
				continue
			}
			checked[f] = true
			fh, err := os.Open(f)
			s.must(t, err)
			arc, err := backup.ReadArchive(fh)
			fh.Close()
			if err != nil {
				t.Fatalf("BAK-3: %s left %s, which isn't a backup: %v", when, filepath.Base(f), err)
			}
			cp := ltest.NewDB(t)
			if _, err := backup.RestoreArchive(ctx, cp, arc); err != nil {
				t.Fatalf("BAK-3: %s left %s, which won't restore: %v", when, filepath.Base(f), err)
			}
			ltest.Check(t, ctx, cp)
		}
	}

	cmd := exec.Command(bin, "-backup", dir, "-tag", "good")
	cmd.Env = envFor()
	out, err := cmd.CombinedOutput()
	s.must(t, err)
	good := ""
	for _, l := range strings.Split(string(out), "\n") {
		if p, ok := strings.CutPrefix(strings.TrimSpace(l), "backup: "); ok {
			good = p
		}
	}
	if good == "" {
		t.Fatalf("no backup: %s", out)
	}
	goodState := ltest.State(t, s.db)
	s.sell(t, s.till1, "0", []line{{"productId": s.prod["odd"], "quantity": "2"}}, cash("100"))
	was := ltest.State(t, s.db)

	cuts := max(steps()/2000, 1) // make test: one; make battle: ten
	for i := range cuts {
		ms := func(lo, hi int) time.Duration { return time.Duration(lo+rand.IntN(hi-lo)) * time.Millisecond }
		cut(ms(20, 1500), envFor(), "-backup", dir, "-tag", fmt.Sprintf("cut%d", i))
		allRestore("a cut -backup")

		cut(ms(300, 2500), envFor("BACKUP_INTERVAL=400ms")) // the server's automatic backups
		allRestore("a cut automatic backup")
		was = ltest.State(t, s.db) // the server's start touches users

		cut(ms(50, 3000), envFor(), "-restore", good)
		now := ltest.State(t, s.db)
		if len(ltest.Changed(now, goodState)) > 0 && len(ltest.Changed(now, was)) > 0 {
			t.Fatalf("BAK-5: a cut restore left half of it: %v differ from the backup, %v from before",
				ltest.Changed(now, goodState), ltest.Changed(now, was))
		}
		s.audit(t)
		was = now
		allRestore("a cut restore's safety copy")
		if t.Failed() {
			t.Fatalf("after cut %d", i+1)
		}
	}
	t.Logf("%d cuts, %d backups checked", cuts, len(checked))
}
