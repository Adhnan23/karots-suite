package config

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all runtime configuration, parsed once at startup from the
// environment. Missing required values are fatal so the process never starts
// in a half-configured state.
type Config struct {
	Env           string
	DatabaseURL   string
	ServerPort    int
	JWTSecret     string
	JWTAccessTTL  time.Duration
	JWTRefreshTTL time.Duration
	CORSOrigins   []string
	// CookieSecure controls the Secure flag on the login cookie.
	//
	// "auto" (the default) follows the actual connection: Secure when the request
	// arrived over HTTPS, off when it did not. Tying it to APP_ENV instead made a
	// plain-HTTP shop network unusable — the browser drops a Secure cookie on
	// http://, so every till logged in and bounced straight back to the login
	// page, while the server's own http://localhost kept working because browsers
	// treat localhost as a secure context. "always" forces it on for an install
	// genuinely served over HTTPS; "never" pins it off.
	CookieSecure CookieSecureMode
	// BackupDir is where automatic time-based backups are written. Empty disables
	// the scheduler (manual backup/restore in the UI is unaffected). It should
	// point at a mounted volume or off-site-synced path — a backup on the DB's own
	// disk does not survive disk/host loss.
	BackupDir string
	// BackupInterval is how often an automatic backup runs (when BackupDir is set).
	BackupInterval time.Duration
	// BackupKeep is how many automatic backup files to retain (oldest pruned).
	BackupKeep int
	// BackupCopyDir is the copy folder preset (USB / share, M8c2): seeded once
	// into backup_copies, after which the Data page owns it.
	BackupCopyDir string
	// Cloud is the vendor's Karots cloud backup — set only in paying shops' builds.
	Cloud CloudBackup
	// FileDataDir is the data folder of a file-mode shop (its own PostgreSQL);
	// empty in server mode.
	FileDataDir string
	// Version is this binary's release version ("dev" for an unstamped build);
	// every response carries it as X-Karots-Version.
	Version string
}

func Load() *Config {
	c := &Config{
		Env:         getEnv("APP_ENV", "development"),
		DatabaseURL: mustEnv("DATABASE_URL"),
		ServerPort:  mustInt("SERVER_PORT", 3000),
		JWTSecret:   mustEnv("JWT_SECRET"),
		// Default to a full shift: the UI session is a single access-token cookie
		// with no sliding refresh, so a short TTL silently logs cashiers out mid-task
		// (e.g. losing a long stock-take) when JWT_EXPIRES_IN isn't set. 12h matches
		// a working day; lower it via the env var if stricter sessions are needed.
		JWTAccessTTL:   mustDuration("JWT_EXPIRES_IN", 12*time.Hour),
		JWTRefreshTTL:  mustDuration("JWT_REFRESH_EXPIRES_IN", 7*24*time.Hour),
		CORSOrigins:    strings.Split(getEnv("CORS_ORIGINS", "http://localhost:3000"), ","),
		BackupDir:      getEnv("BACKUP_DIR", ""),
		BackupInterval: mustDuration("BACKUP_INTERVAL", 6*time.Hour),
		BackupKeep:     mustInt("BACKUP_KEEP", 28),
		BackupCopyDir:  getEnv("BACKUP_COPY_DIR", ""),
		Cloud: CloudBackup{URL: getEnv("CLOUD_BACKUP_URL", ""), KeyID: getEnv("CLOUD_BACKUP_KEY_ID", ""),
			Secret: getEnv("CLOUD_BACKUP_SECRET", ""), Folder: getEnv("CLOUD_BACKUP_FOLDER", ""),
			Crypt: getEnv("CLOUD_BACKUP_CRYPT", ""), On: getEnv("CLOUD_BACKUP", "") == "on"},
	}
	c.CookieSecure = ParseCookieSecure(getEnv("COOKIE_SECURE", "auto"))
	if len(c.JWTSecret) < 32 {
		log.Fatal("JWT_SECRET must be at least 32 characters")
	}
	return c
}

// CloudBackup is Karots cloud: the vendor's bucket (URL = https://host/bucket),
// this shop's folder in it and its encryption key (64 hex). On is the switch a
// fresh install starts with; after that the Data page's switch counts.
type CloudBackup struct {
	URL, KeyID, Secret, Folder, Crypt string
	On                                bool
}

// Set reports whether this build carries Karots cloud.
func (c CloudBackup) Set() bool {
	return c.URL != "" && c.KeyID != "" && c.Secret != "" && c.Folder != "" && c.Crypt != ""
}

func (c *Config) IsProd() bool { return c.Env == "production" }

// BackupFolder is where backup files live for the Data page: BACKUP_DIR, or
// ./backups when it is unset — the safety copy taken before a restore must land
// somewhere even on an install without automatic backups.
func (c *Config) BackupFolder() string {
	if c.BackupDir != "" {
		return c.BackupDir
	}
	return "backups"
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("required env var %s is not set", key)
	}
	return v
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func mustInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		log.Fatalf("%s must be an integer", key)
	}
	return n
}

func mustDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		log.Fatalf("%s must be a duration (e.g. 15m, 168h)", key)
	}
	return d
}

// CookieSecureMode is how the Secure flag on the login cookie is decided.
type CookieSecureMode int

const (
	// CookieSecureAuto sets Secure only when the request itself came over HTTPS.
	CookieSecureAuto CookieSecureMode = iota
	CookieSecureAlways
	CookieSecureNever
)

// ParseCookieSecure maps the COOKIE_SECURE setting to a mode.
func ParseCookieSecure(v string) CookieSecureMode {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "always", "true", "1", "yes":
		return CookieSecureAlways
	case "never", "false", "0", "no":
		return CookieSecureNever
	default:
		return CookieSecureAuto
	}
}
