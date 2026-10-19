package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const windowsTask = "KarotsSuite"

// ps runs one PowerShell script; its first line "# karots:<name>" names it in the log.
func ps(j *Job, script string) (string, error) {
	name := strings.TrimPrefix(strings.SplitN(script, "\n", 2)[0], "# karots:")
	fmt.Fprintf(j.Log, "→ PowerShell: %s\n", j.redact(name))
	out, err := j.Run.Script(script)
	if s := strings.TrimSpace(out); s != "" {
		fmt.Fprintln(j.Log, j.redact(s))
	}
	if err != nil {
		return out, fmt.Errorf("%s: %w", name, err)
	}
	return strings.TrimSpace(out), nil
}

// psq is a PowerShell single-quoted string.
func psq(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

func windowsProgram(j *Job) error {
	if j.Updating {
		// Stop the running till so its exe can be replaced (a file-mode PostgreSQL
		// left running is stopped by Karots itself on its next start).
		if _, err := ps(j, fmt.Sprintf(`# karots:stop-karots
Stop-ScheduledTask -TaskName %s -ErrorAction SilentlyContinue
$p = Get-Process | Where-Object { $_.Path -eq %s }
$p | Stop-Process -Force -ErrorAction SilentlyContinue
$p | Wait-Process -Timeout 30 -ErrorAction SilentlyContinue`, windowsTask, psq(j.binPath()))); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(at(j.Conf.InstallDir), 0o755); err != nil {
		return err
	}
	if err := copyFile(filepath.Join(j.Dir, j.Conf.Binary), at(j.binPath()), 0o755); err != nil {
		return fmt.Errorf("copy %s: %w (is Karots still running? close it and Try again)", j.Conf.Binary, err)
	}
	return nil
}

func windowsSettings(j *Job) error {
	env := joinOS(j, j.Conf.InstallDir, ".env")
	if err := os.WriteFile(at(env), j.Conf.EnvFile(), 0o600); err != nil {
		return err
	}
	// Owner-only: the shop user, SYSTEM and Administrators.
	_, err := ps(j, fmt.Sprintf("# karots:lock-settings\nicacls %s /inheritance:r /grant:r %s %s %s | Out-Null",
		psq(env), psq(j.User+":F"), psq("SYSTEM:F"), psq("Administrators:F")))
	return err
}

func windowsBackups(j *Job) error {
	return os.MkdirAll(at(joinOS(j, j.Conf.InstallDir, "backups")), 0o755)
}

// windowsStartup runs Karots at the shop user's sign-in, with no window (a
// till runs signed in, and in the user's session it sees their USB printers).
// Sign-out / shutdown still reach it as SIGTERM, so PostgreSQL stops cleanly.
func windowsStartup(j *Job) error {
	logs := joinOS(j, j.Conf.InstallDir, "logs")
	if err := os.MkdirAll(at(logs), 0o755); err != nil {
		return err
	}
	cmd := fmt.Sprintf(`-WindowStyle Hidden -NoProfile -ExecutionPolicy Bypass -Command "& '%s' *>> '%s'"`, j.binPath(), joinOS(j, logs, "karots.log"))
	_, err := ps(j, fmt.Sprintf(`# karots:startup-task
$action = New-ScheduledTaskAction -Execute 'powershell.exe' -Argument %s -WorkingDirectory %s
$trigger = New-ScheduledTaskTrigger -AtLogOn -User %s
$principal = New-ScheduledTaskPrincipal -UserId %s -LogonType Interactive -RunLevel Highest
$settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -RestartCount 999 -RestartInterval (New-TimeSpan -Minutes 1) -ExecutionTimeLimit ([TimeSpan]::Zero) -MultipleInstances IgnoreNew
Register-ScheduledTask -TaskName %s -Action $action -Trigger $trigger -Principal $principal -Settings $settings -Force | Out-Null
Start-ScheduledTask -TaskName %s`, psq(cmd), psq(j.Conf.InstallDir), psq(j.User), psq(j.User), windowsTask, windowsTask))
	return err
}

func windowsNetwork(j *Job) error {
	_, err := ps(j, fmt.Sprintf(`# karots:firewall
if (-not (Get-NetFirewallRule -DisplayName 'Karots Suite' -ErrorAction SilentlyContinue)) {
  New-NetFirewallRule -DisplayName 'Karots Suite' -Direction Inbound -Protocol TCP -LocalPort %d -Action Allow | Out-Null
} else {
  Set-NetFirewallRule -DisplayName 'Karots Suite' -LocalPort %d
}`, j.Conf.Port, j.Conf.Port))
	return err
}
