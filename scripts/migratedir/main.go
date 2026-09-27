// Command migratedir applies a directory of goose migrations to a database.
// Used by squash-migrations.sh to rebuild the "originals" DB straight from
// karots-pos's migration files, so the identity check never depends on a
// leftover database. Usage: go run ./scripts/migratedir <database-url> <dir>
package main

import (
	"log"
	"os"

	appdb "karots-suite/internal/db"
)

func main() {
	if len(os.Args) != 3 {
		log.Fatal("usage: migratedir <database-url> <migrations-dir>")
	}
	conn, err := appdb.Connect(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	if err := appdb.RunMigrations(conn.DB, os.DirFS(os.Args[2])); err != nil {
		log.Fatal(err)
	}
}
