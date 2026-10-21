package db

import (
	"os"
	"testing"
)

// The app's connections run with JIT off: Karots runs small queries, and on a
// new shop's database (no statistics yet) PostgreSQL otherwise spends ~250 ms
// compiling each search.
func TestConnectTurnsJITOff(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	conn, err := Connect(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	var jit string
	if err := conn.Get(&jit, `SHOW jit`); err != nil {
		t.Fatal(err)
	}
	if jit != "off" {
		t.Fatalf("jit = %s, want off", jit)
	}
}

func TestWithJITOff(t *testing.T) {
	cases := map[string]string{
		"postgres://u:p@h:5433/d?sslmode=disable": "postgres://u:p@h:5433/d?jit=off&sslmode=disable",
		"postgres://u:p@h/d":                      "postgres://u:p@h/d?jit=off",
		"host=h port=5434 user=u dbname=d":        "host=h port=5434 user=u dbname=d jit=off",
		"postgres://u@h/d?jit=on":                 "postgres://u@h/d?jit=on", // an explicit choice stays
	}
	for in, want := range cases {
		if got := withJITOff(in); got != want {
			t.Errorf("withJITOff(%q) = %q, want %q", in, got, want)
		}
	}
}
