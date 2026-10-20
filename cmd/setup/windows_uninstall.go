package main

import (
	"fmt"
)

func windowsRemoveStartup(j *Job) error {
	_, err := ps(j, fmt.Sprintf(`# karots:remove-startup
Stop-ScheduledTask -TaskName %s -ErrorAction SilentlyContinue
Unregister-ScheduledTask -TaskName %s -Confirm:$false -ErrorAction SilentlyContinue
%s
$p = Get-Process | Where-Object { $_.Path -eq %s }
$p | Stop-Process -Force -ErrorAction SilentlyContinue
$p | Wait-Process -Timeout 30 -ErrorAction SilentlyContinue`, windowsTask, windowsTask, stopLauncherPS(j), psq(j.binPath())))
	return err
}

// windowsRemoveOpen removes the kiosk (and closes its browser) and the
// background shortcuts.
func windowsRemoveOpen(j *Job) error {
	_, err := ps(j, `# karots:remove-open
`+noKiosk+`
Get-CimInstance Win32_Process | Where-Object { $_.CommandLine -like '*--kiosk*' } | ForEach-Object { Invoke-CimMethod -InputObject $_ -MethodName Terminate | Out-Null }
foreach ($d in @([Environment]::GetFolderPath('CommonDesktopDirectory'), [Environment]::GetFolderPath('CommonPrograms'))) {
  Remove-Item -Path (Join-Path $d 'Karots Suite.url') -ErrorAction SilentlyContinue
}`)
	return err
}

func windowsRemoveNetwork(j *Job) error {
	_, err := ps(j, `# karots:remove-firewall
Remove-NetFirewallRule -DisplayName 'Karots Suite' -ErrorAction SilentlyContinue`)
	return err
}

// windowsRemoveDatabase drops the shop's database and role as postgres (the
// profile's superuser password, else the one typed on the page); with
// RemovePostgres it runs PostgreSQL's own uninstaller.
func windowsRemoveDatabase(j *Job) error {
	bin := findPSQL(j)
	if bin == "" {
		fmt.Fprintln(j.Log, "PostgreSQL isn't on this PC — nothing to delete")
		return nil
	}
	db := j.Conf.DB
	tag, super := "", ""
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
	if _, err := winPSQL(j, bin, tag, super, "DROP DATABASE IF EXISTS "+ident(db.Name)+" WITH (FORCE)"); err != nil {
		return err
	}
	winPSQL(j, bin, tag, super, "DROP ROLE IF EXISTS "+ident(db.User)) //nolint:errcheck // the database is what matters
	if !j.RemovePostgres {
		return nil
	}
	j.note("Removing PostgreSQL — a few minutes")
	_, err := ps(j, `# karots:remove-postgresql
$u = Get-ChildItem 'C:\Program Files\PostgreSQL\*\uninstall-postgresql.exe' -ErrorAction SilentlyContinue | Select-Object -First 1
if ($u) { Start-Process -FilePath $u.FullName -ArgumentList '--mode unattended' -Wait }`)
	return err
}
