package main

import (
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"

	"karots-suite/internal/config"
	"karots-suite/internal/filedb"
)

// startFileMode runs the shop's own PostgreSQL (a file-mode build with no
// DATABASE_URL) and points the app at it. The app's port is claimed first, so a
// second copy stops there — before it could touch the running shop's database.
// JWT_SECRET and BACKUP_DIR get file-mode defaults; .env still overrides both.
func startFileMode(dir string) (*filedb.DB, net.Listener) {
	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "3000"
	}
	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("port %s is in use — is the shop already running? (%v)", port, err)
	}
	if dir == "" {
		dir = filepath.Join(config.ExeDir(), "data")
	}
	dbPort := filedb.DefaultPort
	if v := os.Getenv("FILE_DB_PORT"); v != "" {
		if dbPort, err = strconv.Atoi(v); err != nil {
			log.Fatalf("FILE_DB_PORT=%q is not a port number", v)
		}
	}
	log.Printf("file mode: starting PostgreSQL %s in %s (port %d)…", filedb.PGVersion, dir, dbPort)
	d, err := filedb.Open(filedb.Options{Dir: dir, Port: dbPort})
	if err != nil {
		log.Fatalf("file mode: %v", err)
	}
	os.Setenv("DATABASE_URL", d.URL)
	setDefaultEnv("JWT_SECRET", d.JWTSecret)
	setDefaultEnv("BACKUP_DIR", filepath.Join(d.Dir, "backups"))
	return d, ln
}

func setDefaultEnv(key, val string) {
	if os.Getenv(key) == "" {
		os.Setenv(key, val)
	}
}
