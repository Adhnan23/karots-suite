package main

import (
	"fmt"
	"os"
	"strings"
)

// windowsOpen: kiosk = the till full-screen at sign-in (karots-pos's relaunch
// loop, run with no window); background = a Karots Suite shortcut on every
// desktop and in the Start menu, nothing locked.
func windowsOpen(j *Job) error {
	if j.Role == RoleServer && j.Conf.Open == "background" {
		_, err := ps(j, fmt.Sprintf(`# karots:shortcuts
$url = "[InternetShortcut]`+"`r`n"+`URL=%s`+"`r`n"+`"
foreach ($d in @([Environment]::GetFolderPath('CommonDesktopDirectory'), [Environment]::GetFolderPath('CommonPrograms'))) {
  Set-Content -Path (Join-Path $d 'Karots Suite.url') -Value $url -Encoding ASCII
}
%s`, j.tillURL(), noKiosk))
		return err
	}
	// The default browser, then the usual install places. Edge is on every
	// Windows 10 / 11, so nothing is ever downloaded.
	out, _ := ps(j, `# karots:find-browser
try {
  $id = (Get-ItemProperty 'HKCU:\Software\Microsoft\Windows\Shell\Associations\UrlAssociations\http\UserChoice' -ErrorAction Stop).ProgId
  $cmd = (Get-ItemProperty "Registry::HKEY_CLASSES_ROOT\$id\shell\open\command" -ErrorAction Stop).'(default)'
  if ($cmd -match '^\s*"([^"]+)"') { "DEFAULT=" + $Matches[1] } elseif ($cmd -match '^\s*(\S+)') { "DEFAULT=" + $Matches[1] }
} catch {}
foreach ($p in @("$env:ProgramFiles\Google\Chrome\Application\chrome.exe", "${env:ProgramFiles(x86)}\Google\Chrome\Application\chrome.exe",
  "$env:LOCALAPPDATA\Google\Chrome\Application\chrome.exe", "$env:LOCALAPPDATA\Chromium\Application\chrome.exe", "$env:ProgramFiles\Chromium\Application\chrome.exe",
  "$env:ProgramFiles\Mozilla Firefox\firefox.exe", "${env:ProgramFiles(x86)}\Mozilla Firefox\firefox.exe", "$env:ProgramFiles\LibreWolf\librewolf.exe",
  "${env:ProgramFiles(x86)}\Microsoft\Edge\Application\msedge.exe", "$env:ProgramFiles\Microsoft\Edge\Application\msedge.exe")) {
  if (Test-Path $p) { "FOUND=$p" }
}`)
	def, found := "", []string(nil)
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if p, ok := strings.CutPrefix(line, "DEFAULT="); ok {
			def = p
		} else if p, ok := strings.CutPrefix(line, "FOUND="); ok {
			found = append(found, p)
		}
	}
	browser, family := pickBrowser(def, found)
	if browser == "" {
		return fmt.Errorf("no Chrome, Edge or Firefox on this PC — install one, then Try again")
	}
	fmt.Fprintf(j.Log, "kiosk browser: %s\n", browser)
	dir := j.Conf.InstallDir
	if err := os.MkdirAll(at(dir), 0o755); err != nil {
		return err
	}
	profile := joinOS(j, dir, "kiosk-profile")
	kiosk := joinOS(j, dir, "kiosk.cmd")
	// Relaunch loop: Alt+F4 or a crash reopens the till; only the admin / support
	// kiosk-exit hotkey (it writes %SENTINEL%) lets it stop on the desktop.
	script := strings.ReplaceAll(fmt.Sprintf(`@echo off
set SENTINEL=%%TEMP%%\karots-kiosk-exit
if exist "%%SENTINEL%%" del "%%SENTINEL%%"
if not exist "%[3]s" mkdir "%[3]s"
:wait
curl -s -o NUL %[1]s/health && goto launch
timeout /t 2 >NUL
goto wait
:launch
start "" /wait "%[2]s" %[4]s
if exist "%%SENTINEL%%" ( del "%%SENTINEL%%" & goto end )
timeout /t 1 >NUL
goto launch
:end
exit /b 0
`, j.tillURL(), browser, profile, kioskArgs(family, j.tillURL(), profile)), "\n", "\r\n")
	if err := os.WriteFile(at(kiosk), []byte(script), 0o644); err != nil {
		return err
	}
	// karots-start.exe runs the loop with no window: nothing pops up at sign-in.
	launcher, err := ensureLauncher(j)
	if err != nil {
		return err
	}
	// A sign-in task, not a Startup-folder shortcut: Windows holds Startup items
	// back ~10 s after sign-in. Every user who signs in gets the till (BUILTIN\Users,
	// by SID so a non-English Windows matches), not elevated. The shortcut older
	// installs made goes, or the till would open twice.
	_, err = ps(j, fmt.Sprintf(`# karots:kiosk-task
Remove-Item -Path (Join-Path ([Environment]::GetFolderPath('CommonStartup')) 'Karots Suite Kiosk.lnk') -ErrorAction SilentlyContinue
$action = New-ScheduledTaskAction -Execute %s -Argument %s -WorkingDirectory %s
$trigger = New-ScheduledTaskTrigger -AtLogOn
$principal = New-ScheduledTaskPrincipal -GroupId 'S-1-5-32-545' -RunLevel Limited
$settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -ExecutionTimeLimit ([TimeSpan]::Zero) -MultipleInstances Parallel
Register-ScheduledTask -TaskName %s -Action $action -Trigger $trigger -Principal $principal -Settings $settings -Force | Out-Null`,
		psq(launcher), psq(`-hidden cmd.exe /c "`+kiosk+`"`), psq(dir), psq(windowsKioskTask)))
	return err
}

const windowsKioskTask = "KarotsSuiteKiosk"

// noKiosk removes a kiosk set up before (a shop switched to background).
var noKiosk = `Unregister-ScheduledTask -TaskName '` + windowsKioskTask + `' -Confirm:$false -ErrorAction SilentlyContinue
Remove-Item -Path (Join-Path ([Environment]::GetFolderPath('CommonStartup')) 'Karots Suite Kiosk.lnk') -ErrorAction SilentlyContinue`
