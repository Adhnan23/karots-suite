package main

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"
)

// ErrNeedSuperPassword: PostgreSQL was already on this PC and the profile's
// superuser password isn't its password. The page asks for it and tries again.
var ErrNeedSuperPassword = errors.New("PostgreSQL is already on this PC — enter its 'postgres' password")

// The PostgreSQL 17 installer winget's PostgreSQL.PostgreSQL.17 (17.11-5) points
// at — used when this PC has no winget. Bump both together from the winget-pkgs
// manifest.
const (
	pgInstallerURL    = "https://get.enterprisedb.com/postgresql/postgresql-17.11-5-windows-x64.exe"
	pgInstallerSHA256 = "612C7F5400A003AAFFA0372CB273EED52278F93C680AB911EB9CB2229E329072"
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
	super, tag := "", ""
	for _, try := range []struct{ pw, tag string }{{db.SuperPassword, "super"}, {j.SuperPassword, "typed"}} {
		if try.pw == "" {
			continue
		}
		if out, _ := winPSQL(j, bin, try.tag, try.pw, "SELECT 'up'"); out == "up" {
			super, tag = try.pw, try.tag
			break
		}
	}
	if super == "" {
		return ErrNeedSuperPassword
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

// installPostgres finds psql.exe, installing PostgreSQL 17 first when it isn't
// on this PC: winget, else the same EDB installer winget uses, downloaded and
// checked against its pinned SHA-256 (some Windows 10 builds have no winget).
// Each stage says on the page what it is doing — it can take ten minutes.
func installPostgres(j *Job) (string, error) {
	find := func() string {
		bin, _ := ps(j, `# karots:find-psql
(Get-ChildItem 'C:\Program Files\PostgreSQL\*\bin\psql.exe' -ErrorAction SilentlyContinue | Sort-Object FullName -Descending | Select-Object -First 1).FullName`)
		return bin
	}
	if bin := find(); bin != "" {
		return bin, nil
	}
	args := fmt.Sprintf("--superpassword %s --serverport %d", j.Conf.DB.SuperPassword, j.Conf.DB.Port)
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
if ((Get-FileHash $f -Algorithm SHA256).Hash -ne '%s') { Remove-Item $f; throw 'the PostgreSQL download is damaged (checksum differs)' }`, pgInstallerURL, pgInstallerSHA256)); err != nil {
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
