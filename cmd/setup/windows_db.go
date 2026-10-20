package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"karots-suite/internal/shopconf"
)

// windowsDatabase installs PostgreSQL 17 with the profile's superuser password
// and port (winget), then makes the role (the profile's password — an existing
// role gets it too) and the database. A database elsewhere is only checked.
func windowsDatabase(j *Job) error {
	db := j.Conf.DB
	if !db.Local {
		addr := net.JoinHostPort(db.Host, strconv.Itoa(db.Port))
		c, err := net.DialTimeout("tcp", addr, 5*time.Second)
		if err != nil {
			return fmt.Errorf("the database at %s doesn't answer — check that PC is on and its PostgreSQL allows this one", addr)
		}
		c.Close()
		return nil
	}
	bin, err := installPostgres(j)
	if err != nil {
		return err
	}
	tag, super, done, err := pgSuper(j, bin)
	defer done()
	if err != nil {
		return err
	}
	role := "CREATE ROLE"
	if out, _ := winPSQL(j, bin, tag, super, "SELECT 1 FROM pg_roles WHERE rolname = "+lit(db.User)); out == "1" {
		role = "ALTER ROLE"
	}
	if _, err := winPSQL(j, bin, tag, super, role+" "+ident(db.User)+" WITH LOGIN PASSWORD "+lit(db.Password)); err != nil {
		return err
	}
	if out, _ := winPSQL(j, bin, tag, super, "SELECT 1 FROM pg_database WHERE datname = "+lit(db.Name)); out != "1" {
		if _, err := winPSQL(j, bin, tag, super, "CREATE DATABASE "+ident(db.Name)+" OWNER "+ident(db.User)); err != nil {
			return err
		}
	}
	return nil
}

// winPSQL runs one statement as postgres over localhost on the profile's port.
func winPSQL(j *Job, bin, tag, password, sql string) (string, error) {
	return ps(j, fmt.Sprintf(`# karots:psql %s
$env:PGPASSWORD = %s
& %s -U postgres -h localhost -p %d -d postgres -tAc %s
$code = $LASTEXITCODE
Remove-Item Env:\PGPASSWORD
exit $code`, tag, psq(password), psq(bin), j.Conf.DB.Port, psq(sql)))
}

// pgSuper logs in as postgres with the profile's password; when PostgreSQL was
// already on this PC with another one, Setup (admin) lets postgres in from this
// PC only, without a password, for the step: two lines on top of pg_hba.conf,
// reloaded, and the file put back by done (call it, even after an error). Its
// password is never changed (other programs may use it) and never asked for.
func pgSuper(j *Job, bin string) (tag, password string, done func(), err error) {
	done = func() {}
	if out, _ := winPSQL(j, bin, "super", j.Conf.DB.SuperPassword, "SELECT 'up'"); out == "up" {
		return "super", j.Conf.DB.SuperPassword, done, nil
	}
	fmt.Fprintln(j.Log, "PostgreSQL's own password isn't this shop's — letting Setup in from this PC for this step")
	pgctl := psq(filepath.Join(filepath.Dir(bin), "pg_ctl.exe"))
	find := `$svc = Get-CimInstance Win32_Service | Where-Object { $_.Name -like 'postgresql*' } | Select-Object -First 1
if (-not $svc -or $svc.PathName -notmatch '-D\s+(?:"([^"]+)"|(\S+))') { 'no PostgreSQL service'; exit 1 }
$data = if ($Matches[1]) { $Matches[1] } else { $Matches[2] }; $hba = Join-Path $data 'pg_hba.conf'; $keep = "$hba.karots"`
	done = func() {
		ps(j, `# karots:pg-trust off
`+find+`
if (Test-Path $keep) { Move-Item -Force $keep $hba; & `+pgctl+` reload -D $data | Out-Null; if ($LASTEXITCODE -ne 0) { Restart-Service $svc.Name } }
exit 0`) //nolint:errcheck // nothing to put back = fine
	}
	ps(j, `# karots:pg-trust on
`+find+`
if ($svc.State -ne 'Running') { Start-Service $svc.Name; Start-Sleep 3 }
if (-not (Test-Path $keep)) { Copy-Item $hba $keep }
$lines = @('host all postgres 127.0.0.1/32 trust', 'host all postgres ::1/128 trust') + (Get-Content $keep)
[IO.File]::WriteAllLines($hba, [string[]]$lines)
& `+pgctl+` reload -D $data | Out-Null
if ($LASTEXITCODE -ne 0) { Restart-Service $svc.Name }
Start-Sleep 1`) //nolint:errcheck // the login below says whether it worked
	if out, _ := winPSQL(j, bin, "trust", "trust", "SELECT 'up'"); out == "up" {
		return "trust", "trust", done, nil
	}
	return "", "", done, fmt.Errorf("PostgreSQL on this PC doesn't answer — check its service (postgresql-x64-17) is running, then Try again")
}

// installPostgres finds psql.exe, installing PostgreSQL 17 first when it isn't
// on this PC: winget, else the same EDB installer winget uses, downloaded and
// checked against its pinned SHA-256 (some Windows 10 builds have no winget).
// Each stage says on the page what it is doing — it can take ten minutes.
func installPostgres(j *Job) (string, error) {
	find := func() string { return findPSQL(j) }
	if bin := find(); bin != "" {
		return bin, nil
	}
	// The server and psql only: pgAdmin (thousands of files) and StackBuilder
	// took most of a ~7 min install on a till, and the shop never opens them.
	args := fmt.Sprintf("--superpassword %s --serverport %d --disable-components pgAdmin,stackbuilder", j.Conf.DB.SuperPassword, j.Conf.DB.Port)
	// The copy the Builder put beside the three files: no download at the shop.
	if local := filepath.Join(j.Dir, shopconf.PGInstallerName); fileExists(local) {
		j.note("Installing PostgreSQL 17 from the copy beside Setup — a few minutes, nothing to click")
		ps(j, fmt.Sprintf(`# karots:local-postgresql
$f = %s
if ((Get-FileHash $f -Algorithm SHA256).Hash -ne '%s') { 'the PostgreSQL installer beside Setup is damaged — downloading it instead'; exit 0 }
Start-Process -FilePath $f -ArgumentList '--mode unattended --unattendedmodeui none %s' -Wait`, psq(local), shopconf.PGInstallerSHA256, args)) //nolint:errcheck
		if bin := find(); bin != "" {
			j.note("PostgreSQL 17 installed")
			return bin, nil
		}
	}
	j.note("Installing PostgreSQL 17 with winget (it downloads about 400 MB) — several minutes")
	// winget says "already installed" with a non-zero code: not an error.
	ps(j, fmt.Sprintf(`# karots:winget-postgresql
if (-not (Get-Command winget -ErrorAction SilentlyContinue)) { 'no-winget'; exit 0 }
winget install --id PostgreSQL.PostgreSQL.17 -e --silent --accept-package-agreements --accept-source-agreements --custom "%s"`, args)) //nolint:errcheck
	if bin := find(); bin != "" {
		return bin, nil
	}
	j.note("Downloading PostgreSQL 17 (413 MB) from get.enterprisedb.com — several minutes on a slow line")
	if _, err := ps(j, fmt.Sprintf(`# karots:download-postgresql
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
$ProgressPreference = 'SilentlyContinue'
$f = Join-Path $env:TEMP 'karots-postgresql-17.exe'
Invoke-WebRequest -UseBasicParsing -Uri '%s' -OutFile $f
if ((Get-FileHash $f -Algorithm SHA256).Hash -ne '%s') { Remove-Item $f; throw 'the PostgreSQL download is damaged (checksum differs)' }`, shopconf.PGInstallerURL, shopconf.PGInstallerSHA256)); err != nil {
		return "", fmt.Errorf("couldn't download PostgreSQL — this PC must reach get.enterprisedb.com: %w", err)
	}
	j.note("Installing PostgreSQL 17 — a few minutes, nothing to click")
	ps(j, fmt.Sprintf(`# karots:install-postgresql
$f = Join-Path $env:TEMP 'karots-postgresql-17.exe'
Start-Process -FilePath $f -ArgumentList '--mode unattended --unattendedmodeui none %s' -Wait
Remove-Item $f -ErrorAction SilentlyContinue`, args)) //nolint:errcheck
	if bin := find(); bin != "" {
		j.note("PostgreSQL 17 installed")
		return bin, nil
	}
	return "", fmt.Errorf("PostgreSQL didn't install (no psql.exe under C:\\Program Files\\PostgreSQL) — the log above says why; this PC must reach get.enterprisedb.com — then Try again")
}

// findPSQL is the newest psql.exe on this PC ("" when PostgreSQL isn't here).
func findPSQL(j *Job) string {
	bin, _ := ps(j, `# karots:find-psql
(Get-ChildItem 'C:\Program Files\PostgreSQL\*\bin\psql.exe' -ErrorAction SilentlyContinue | Sort-Object FullName -Descending | Select-Object -First 1).FullName`)
	return bin
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
