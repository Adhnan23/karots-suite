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
}

// RestoreReport says what a restore loaded and what this schema could not take.
type RestoreReport struct {
	Tables         int      `json:"tables"`
	Rows           int      `json:"rows"`
	SkippedTables  []string `json:"skippedTables"`
	DroppedColumns []string `json:"droppedColumns"`
}

// CheckFindingRow is one example row of a data-check finding.
type CheckFindingRow struct {
	Ref    string `json:"ref"`
	Detail string `json:"detail"`
}

// CheckFinding is one kind of suspicious stored data (read-only report).
type CheckFinding struct {
	Key     string            `json:"key"`
	Title   string            `json:"title"`
	Explain string            `json:"explain"`
	Count   int               `json:"count"`
	Sample  []CheckFindingRow `json:"sample"`
}

// RestoreResult is POST /admin/data/restore and /admin/data/backups/:name/restore.
// The session is ended — the SPA sends the user to sign in again.
type RestoreResult struct {
	Report     RestoreReport  `json:"report"`
	SafetyCopy string         `json:"safetyCopy"` // file name of the copy taken first
	Check      []CheckFinding `json:"check"`
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
