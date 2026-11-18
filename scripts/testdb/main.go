// testdb gives every Go test package a database of its own, so `go test` can
// run packages side by side instead of one at a time on the shared suite_db.
//
//	testdb template            make test_tpl: migrated + seeded (as -reset -seed)
//	testdb exec <test binary>  run it with DATABASE_URL on a fresh copy of test_tpl
//
// make test passes it to `go test -exec`; nothing in the tests changes.
package main

import (
	"database/sql"
	"fmt"
	"math/rand/v2"
	"os"
	"os/exec"
	"strings"

	_ "github.com/lib/pq"
)

const tpl = "test_tpl"

func main() {
	base := os.Getenv("DATABASE_URL")
	if base == "" || len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: DATABASE_URL=… testdb template | exec <binary> [args]")
		os.Exit(2)
	}
	admin, err := sql.Open("postgres", base)
	if err != nil {
		fail(err)
	}
	switch os.Args[1] {
	case "template":
		must(admin.Exec(`DROP DATABASE IF EXISTS ` + tpl + ` WITH (FORCE)`))
		must(admin.Exec(`CREATE DATABASE ` + tpl))
		cmd := exec.Command("go", "run", "./cmd/server", "-reset", "-seed")
		cmd.Env = append(os.Environ(), "DATABASE_URL="+dsnFor(base, tpl))
		cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
		if err := cmd.Run(); err != nil {
			fail(fmt.Errorf("seeding the template: %w", err))
		}
	case "exec":
		name := fmt.Sprintf("t_%d", rand.Uint64()>>1)
		must(admin.Exec(`CREATE DATABASE ` + name + ` TEMPLATE ` + tpl))
		admin.Exec(`ALTER DATABASE ` + name + ` SET synchronous_commit = off`) //nolint:errcheck
		cmd := exec.Command(os.Args[2], os.Args[3:]...)
		cmd.Env = append(os.Environ(), "DATABASE_URL="+dsnFor(base, name))
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
		err := cmd.Run()
		admin.Exec(`DROP DATABASE IF EXISTS ` + name + ` WITH (FORCE)`) //nolint:errcheck
		if ee, ok := err.(*exec.ExitError); ok {
			os.Exit(ee.ExitCode())
		} else if err != nil {
			fail(err)
		}
	default:
		fail(fmt.Errorf("unknown command %q", os.Args[1]))
	}
}

func dsnFor(base, name string) string {
	i := strings.LastIndex(base, "/")
	q := ""
	if j := strings.Index(base[i:], "?"); j >= 0 {
		q = base[i+j:]
	}
	return base[:i+1] + name + q
}

func must(_ sql.Result, err error) {
	if err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "testdb:", err)
	os.Exit(1)
}
