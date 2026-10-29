package api

import "time"

// ---- Setup → Data (admin only) ----

// DataGroup is one plugin's tables, offered as a "choose what to include" box.
type DataGroup struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

// BackupFileRow is one backup file in the backup folder.
type BackupFileRow struct {
	Name          string    `json:"name"`
	Size          int64     `json:"size"`
	ModifiedAt    time.Time `json:"modifiedAt"`
	BeforeRestore bool      `json:"beforeRestore"` // the safety copy taken before a restore
}

// DataPage is GET /admin/data.
type DataPage struct {
	Folder        string          `json:"folder"`
	AutoBackups   bool            `json:"autoBackups"` // BACKUP_DIR set → the scheduler runs
	IntervalHours int             `json:"intervalHours"`
	Keep          int             `json:"keep"`
	Groups        []DataGroup     `json:"groups"`
	Backups       []BackupFileRow `json:"backups"`
	FileDataDir   string          `json:"fileDataDir"` // file mode: the shop's own data folder; "" = server mode
}

// RestoreReport says what a restore loaded and what this schema could not take.
type RestoreReport struct {
	Tables         int      `json:"tables"`
	Rows           int      `json:"rows"`
	SkippedTables  []string `json:"skippedTables"`
	DroppedColumns []string `json:"droppedColumns"`
}

// RestoreResult is POST /admin/data/restore and /admin/data/backups/:name/restore.
// The session is ended — the SPA sends the user to sign in again.
type RestoreResult struct {
	Report     RestoreReport  `json:"report"`
	SafetyCopy string `json:"safetyCopy"` // file name of the copy taken first
}

// SettingsImportResult is POST /admin/data/settings/import.
type SettingsImportResult struct {
	Applied int      `json:"applied"`
	Skipped []string `json:"skipped"` // this install's own (identity, printers, default locker)
	Unknown []string `json:"unknown"` // not a setting in this version
}

// EntityImportResult answers a product / customer / supplier import: the good
// rows land, each bad one is reported by its line in the sheet.
type EntityImportResult struct {
	Created int              `json:"created"`
	Updated int              `json:"updated"`
	Skipped int              `json:"skipped"`
	Errors  []ImportRowError `json:"errors"`
	Notes   []string         `json:"notes"`
}

// SetupStep is one go-live step on the dashboard's "Get the shop ready" card.
type SetupStep struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Hint  string `json:"hint"`
	To    string `json:"to"` // the SPA page that finishes it
	Done  bool   `json:"done"`
}

// UpdateStatusDTO is the Data page's Updates card (GET /admin/data/update).
// Enabled is false for a build without an update channel (dev, Docker): the
// card then shows the version only.
type UpdateStatusDTO struct {
	Version        string     `json:"version"`
	Kind           string     `json:"kind"` // "test" | "release" | ""
	Enabled        bool       `json:"enabled"`
	State          string     `json:"state"` // unchecked | up-to-date | available | not-published | offline | bad-signature | installing | failed
	Latest         string     `json:"latest"`
	Notes          string     `json:"notes"`
	Message        string     `json:"message"`
	Progress       string     `json:"progress"`
	RolledBackFrom string     `json:"rolledBackFrom"`
	CheckedAt      *time.Time `json:"checkedAt"`
}

// BackupCopyRow is one place the automatic backups are copied to (GET
// /admin/data/copies): the owner's copy folder or the vendor's Karots cloud.
type BackupCopyRow struct {
	Kind         string     `json:"kind"` // "folder" | "cloud"
	Enabled      bool       `json:"enabled"`
	Path         string     `json:"path"` // folder only
	LastCopiedAt *time.Time `json:"lastCopiedAt"`
	LastTriedAt  *time.Time `json:"lastTriedAt"`
	LastFile     string     `json:"lastFile"`
	LastError    string     `json:"lastError"`
	Waiting      int        `json:"waiting"` // folder: backups it lacks; cloud: 1 when a copy is due
}

// CopyFolderRequest is PUT /admin/data/copies/folder.
type CopyFolderRequest struct {
	Path    string `json:"path"`
	Enabled bool   `json:"enabled"`
}

// CopyCloudRequest is PUT /admin/data/copies/cloud (only the system user may switch it on).
type CopyCloudRequest struct {
	Enabled bool `json:"enabled"`
}

// FetchedBackup is a backup fetched from a copy into the backups folder, ready
// for POST /admin/data/backups/:name/restore.
type FetchedBackup struct {
	Name string `json:"name"`
}

// FolderRoot is a place the folder browser starts from (a drive, Home, USB drives).
type FolderRoot struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// FolderListing is GET /admin/data/folders: one folder on the shop's main PC
// (the server's PC — a browser's own picker can't give a path).
type FolderListing struct {
	Path    string       `json:"path"`
	Parent  string       `json:"parent"`
	Folders []string     `json:"folders"`
	Roots   []FolderRoot `json:"roots"`
}

// NewFolderRequest is POST /admin/data/folders: make Name inside Path.
type NewFolderRequest struct {
	Path string `json:"path"`
	Name string `json:"name"`
}
