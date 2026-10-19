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
	// winget says "already installed" with a non-zero code: not an error. No
	// winget (some Windows 10 builds) or it failed: the same EDB installer winget
	// uses, downloaded and checked against its pinned SHA-256.
	args := fmt.Sprintf("--superpassword %s --serverport %d", db.SuperPassword, db.Port)
	ps(j, fmt.Sprintf(`# karots:install-postgresql
if (Get-Command winget -ErrorAction SilentlyContinue) {
  winget install --id PostgreSQL.PostgreSQL.17 -e --silent --accept-package-agreements --accept-source-agreements --custom "%[1]s"
}
if (-not (Test-Path 'C:\Program Files\PostgreSQL\*\bin\psql.exe')) {
  [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
  $ProgressPreference = 'SilentlyContinue'
  $f = Join-Path $env:TEMP 'karots-postgresql-17.exe'
  Write-Output 'downloading PostgreSQL 17 from get.enterprisedb.com…'
  Invoke-WebRequest -UseBasicParsing -Uri '%[2]s' -OutFile $f
  if ((Get-FileHash $f -Algorithm SHA256).Hash -ne '%[3]s') { Remove-Item $f; throw 'the PostgreSQL download is damaged (checksum differs)' }
  Start-Process -FilePath $f -ArgumentList '--mode unattended --unattendedmodeui none %[1]s' -Wait
  Remove-Item $f -ErrorAction SilentlyContinue
}`, args, pgInstallerURL, pgInstallerSHA256)) //nolint:errcheck
	bin, err := ps(j, `# karots:find-psql
(Get-ChildItem 'C:\Program Files\PostgreSQL\*\bin\psql.exe' -ErrorAction SilentlyContinue | Sort-Object FullName -Descending | Select-Object -First 1).FullName`)
	if err != nil || bin == "" {
		return fmt.Errorf("PostgreSQL didn't install (no psql.exe under C:\\Program Files\\PostgreSQL) — the log above says why; this PC must reach get.enterprisedb.com — then Try again")
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
