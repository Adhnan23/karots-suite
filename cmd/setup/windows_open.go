package main

import (
	"fmt"
	"os"
	"strings"
)

// windowsOpen: kiosk = Chromium / Chrome full-screen at sign-in (karots-pos's
// relaunch loop); background = a Karots Suite shortcut on every desktop and in
// the Start menu, nothing locked.
func windowsOpen(j *Job) error {
	if j.Role == RoleServer && j.Conf.Open == "background" {
		_, err := ps(j, fmt.Sprintf(`# karots:shortcuts
$url = "[InternetShortcut]`+"`r`n"+`URL=%s`+"`r`n"+`"
foreach ($d in @([Environment]::GetFolderPath('CommonDesktopDirectory'), [Environment]::GetFolderPath('CommonPrograms'))) {
  Set-Content -Path (Join-Path $d 'Karots Suite.url') -Value $url -Encoding ASCII
}`, j.tillURL()))
		return err
	}
	browser, _ := ps(j, `# karots:find-browser
function Find { @("$env:ProgramFiles\Google\Chrome\Application\chrome.exe", "${env:ProgramFiles(x86)}\Google\Chrome\Application\chrome.exe", "$env:LOCALAPPDATA\Chromium\Application\chrome.exe", "$env:ProgramFiles\Chromium\Application\chrome.exe") | Where-Object { Test-Path $_ } | Select-Object -First 1 }
$b = Find
if (-not $b) { winget install --id Hibbiki.Chromium -e --silent --accept-package-agreements --accept-source-agreements | Out-Null; $b = Find }
if (-not $b) { winget install --id Google.Chrome -e --silent --accept-package-agreements --accept-source-agreements | Out-Null; $b = Find }
$b`)
	if browser == "" {
		return fmt.Errorf("couldn't install Chrome — install it from google.com/chrome, then Try again")
	}
	dir := j.Conf.InstallDir
	if err := os.MkdirAll(at(dir), 0o755); err != nil {
		return err
	}
	kiosk := joinOS(j, dir, "kiosk.cmd")
	// Relaunch loop: Alt+F4 or a crash reopens the till; only the admin / support
	// kiosk-exit hotkey (it writes %SENTINEL%) lets it stop on the desktop.
	script := strings.ReplaceAll(fmt.Sprintf(`@echo off
set SENTINEL=%%TEMP%%\karots-kiosk-exit
if exist "%%SENTINEL%%" del "%%SENTINEL%%"
:wait
curl -s -o NUL %[1]s/health && goto launch
timeout /t 2 >NUL
goto wait
:launch
start "" /wait "%[2]s" --kiosk --app=%[1]s --incognito --user-data-dir="%[3]s\kiosk-profile" --noerrdialogs --disable-infobars --disable-session-crashed-bubble --disable-features=TranslateUI --check-for-update-interval=31536000
if exist "%%SENTINEL%%" ( del "%%SENTINEL%%" & goto end )
timeout /t 1 >NUL
goto launch
:end
`, j.tillURL(), browser, dir), "\n", "\r\n")
	if err := os.WriteFile(at(kiosk), []byte(script), 0o644); err != nil {
		return err
	}
	_, err := ps(j, fmt.Sprintf(`# karots:kiosk-shortcut
$lnk = Join-Path ([Environment]::GetFolderPath('CommonStartup')) 'Karots Suite Kiosk.lnk'
$sc = (New-Object -ComObject WScript.Shell).CreateShortcut($lnk)
$sc.TargetPath = %s
$sc.WorkingDirectory = %s
$sc.WindowStyle = 7
$sc.Save()`, psq(kiosk), psq(dir)))
	return err
}
