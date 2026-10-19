package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"karots-suite/internal/shopconf"
)

// Role is what this PC is: the shop's main PC (the till server) or a cashier
// PC that only opens the till from the main PC.
type Role string

const (
	RoleServer  Role = "server"
	RoleCashier Role = "cashier"
)

// Job is one install: the shop's install.conf plus what was chosen on this PC.
type Job struct {
	Conf          shopconf.Conf
	Role          Role
	Address       string // cashier PC: the main PC's host[:port]
	User          string // the shop user: Linux login / Windows DOMAIN\user
	Dir           string // the folder holding karots-setup, install.conf and the binary
	Updating      bool   // InstallDir already holds the binary
	SuperPassword string // typed on the page when an existing PostgreSQL's password is unknown
	Run           Runner
	Log           io.Writer

	Kept          []string // an Update: settings kept from this PC (shown on the page)
	launcherReady bool     // Windows: karots-start.exe was placed this run
}

// Runner runs the OS tools; tests record the commands instead.
type Runner interface {
	Run(name string, args ...string) (string, error)
	Script(ps string) (string, error) // PowerShell (Windows only)
}

// Step is one modular piece of the install. Applies says whether it is part of
// this job (role, mode, OS); the page lists only the ones that are.
type Step struct {
	ID, Title string
	Detail    func(*Job) string // the plain line on the page's "What will happen"
	Applies   func(*Job) bool
	Do        func(*Job) error
}

func (j *Job) server() bool   { return j.Role == RoleServer }
func (j *Job) serverDB() bool { return j.server() && j.Conf.Mode == "server" }
func (j *Job) windows() bool  { return j.Conf.OS == "windows" }
func (j *Job) localDB() bool  { return j.serverDB() && j.Conf.DB.Local }
func always(*Job) bool        { return true }

// backupDir is where automatic backups go: the chosen folder, else a one-PC
// shop's own data\backups.
func (j *Job) backupDir() string {
	if d := j.Conf.Env["BACKUP_DIR"]; d != "" {
		return d
	}
	return joinOS(j, j.Conf.InstallDir, "data", "backups")
}
func (j *Job) binPath() string { return joinOS(j, j.Conf.InstallDir, j.Conf.Binary) }
func (j *Job) tillURL() string {
	if j.Role == RoleCashier {
		return "http://" + j.Address
	}
	return fmt.Sprintf("http://localhost:%d", j.Conf.Port)
}

// joinOS joins with the target's separator (tests build Windows paths on Linux).
func joinOS(j *Job, parts ...string) string {
	sep := "/"
	if j.windows() {
		sep = `\`
	}
	out := parts[0]
	for _, p := range parts[1:] {
		if len(out) > 0 && out[len(out)-1] != sep[0] {
			out += sep
		}
		out += p
	}
	return out
}

// pick chooses the Linux or Windows body of a step.
func pick(linux, windows func(*Job) error) func(*Job) error {
	return func(j *Job) error {
		if j.windows() {
			return windows(j)
		}
		return linux(j)
	}
}

// Steps is the install, in order, filtered to this job.
func Steps(j *Job) []Step {
	all := []Step{
		{ID: "checks", Title: "Check this PC", Applies: always, Do: checks,
			Detail: func(*Job) string { return "Make sure this PC can run Karots (admin rights, the port is free)" }},
		{ID: "database", Title: "Database", Applies: (*Job).serverDB, Do: pick(linuxDatabase, windowsDatabase),
			Detail: func(j *Job) string {
				if j.Conf.DB.Local {
					return fmt.Sprintf("Install PostgreSQL on this PC and make the shop's database (port %d)", j.Conf.DB.Port)
				}
				return fmt.Sprintf("Use the shop's database on %s:%d", j.Conf.DB.Host, j.Conf.DB.Port)
			}},
		{ID: "program", Title: "Program", Applies: (*Job).server, Do: pick(linuxProgram, windowsProgram),
			Detail: func(j *Job) string {
				if j.Conf.Mode == "file" {
					return "Put Karots in " + j.Conf.InstallDir + " — the shop's data stays on this PC, beside it"
				}
				return "Put Karots in " + j.Conf.InstallDir
			}},
		{ID: "settings", Title: "Settings", Applies: (*Job).server, Do: pick(linuxSettings, windowsSettings),
			Detail: func(j *Job) string { return fmt.Sprintf("Write the shop's settings (port %d)", j.Conf.Port) }},
		{ID: "backups", Title: "Backup folder", Applies: func(j *Job) bool { return j.server() && j.Conf.Env["BACKUP_DIR"] != "" }, Do: pick(linuxBackups, windowsBackups),
			Detail: func(j *Job) string { return "Keep automatic backups in " + j.backupDir() }},
		{ID: "startup", Title: "Start with the PC", Applies: (*Job).server, Do: pick(linuxStartup, windowsStartup),
			Detail: func(*Job) string { return "Start Karots when this PC starts" }},
		{ID: "network", Title: "Network", Applies: (*Job).server, Do: pick(linuxNetwork, windowsNetwork),
			Detail: func(j *Job) string {
				return fmt.Sprintf("Let cashier PCs on the shop's network reach port %d", j.Conf.Port)
			}},
		{ID: "printing", Title: "Printing", Applies: func(j *Job) bool { return j.server() && !j.windows() }, Do: linuxPrinting,
			Detail: func(*Job) string { return "Install printing (CUPS)" }},
		{ID: "open", Title: "Open the till", Applies: always, Do: pick(linuxOpen, windowsOpen),
			Detail: func(j *Job) string {
				if j.Role == RoleCashier || j.Conf.Open == "kiosk" {
					return "Open the till full-screen when someone signs in to this PC"
				}
				return "Run in the background, with a Karots Suite shortcut on the desktop"
			}},
		{ID: "health", Title: "Check the till answers", Applies: always, Do: health,
			Detail: func(j *Job) string { return "Check the till answers at " + j.tillURL() }},
	}
	var out []Step
	for _, s := range all {
		if s.Applies(j) {
			out = append(out, s)
		}
	}
	return out
}

// RunAll runs the job's steps in order and stops at the first failure.
func RunAll(j *Job, mark func(id string, err error)) error { return runSteps(j, Steps(j), mark) }

func runSteps(j *Job, steps []Step, mark func(id string, err error)) error {
	for _, s := range steps {
		fmt.Fprintf(j.Log, "\n== %s\n", s.Title)
		err := s.Do(j)
		mark(s.ID, err)
		if err != nil {
			return fmt.Errorf("%s: %w", s.Title, err)
		}
	}
	return nil
}

// loadConf reads install.conf from the folder holding the three files.
func loadConf(dir string) (shopconf.Conf, error) {
	p := filepath.Join(dir, shopconf.FileName)
	if _, err := os.Stat(p); err != nil {
		return shopconf.Conf{}, fmt.Errorf("%s isn't next to karots-setup — build this shop in the Shop Builder and copy all three files", shopconf.FileName)
	}
	return shopconf.Read(p)
}
