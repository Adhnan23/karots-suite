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
	// Chrome / Chromium first; winget only if this Windows has it; then Edge,
	// which every Windows 10 / 11 has. The path comes back on a BROWSER= line so
	// no error text is ever taken for it.
	out, _ := ps(j, `# karots:find-browser
function Find { @("$env:ProgramFiles\Google\Chrome\Application\chrome.exe", "${env:ProgramFiles(x86)}\Google\Chrome\Application\chrome.exe", "$env:LOCALAPPDATA\Chromium\Application\chrome.exe", "$env:ProgramFiles\Chromium\Application\chrome.exe") | Where-Object { Test-Path $_ } | Select-Object -First 1 }
$b = Find
if (-not $b -and (Get-Command winget -ErrorAction SilentlyContinue)) {
  winget install --id Google.Chrome -e --silent --accept-package-agreements --accept-source-agreements | Out-Null; $b = Find
}
if (-not $b) { $b = @("${env:ProgramFiles(x86)}\Microsoft\Edge\Application\msedge.exe", "$env:ProgramFiles\Microsoft\Edge\Application\msedge.exe") | Where-Object { Test-Path $_ } | Select-Object -First 1 }
if ($b) { "BROWSER=$b" }`)
	browser := ""
	for _, line := range strings.Split(out, "\n") {
		if p, ok := strings.CutPrefix(strings.TrimSpace(line), "BROWSER="); ok {
			browser = p
		}
	}
	if browser == "" {
		return fmt.Errorf("no Chrome or Edge on this PC — install Chrome from google.com/chrome, then Try again")
	}
	launch := "--kiosk --app=" + j.tillURL() + " --incognito"
	if strings.HasSuffix(strings.ToLower(browser), "msedge.exe") {
		launch = "--kiosk " + j.tillURL() + " --edge-kiosk-type=fullscreen --inprivate --no-first-run" // Edge's kiosk form
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
start "" /wait "%[2]s" %[4]s --user-data-dir="%[3]s\kiosk-profile" --noerrdialogs --disable-infobars --disable-session-crashed-bubble --disable-features=TranslateUI --check-for-update-interval=31536000
if exist "%%SENTINEL%%" ( del "%%SENTINEL%%" & goto end )
timeout /t 1 >NUL
goto launch
:end
`, j.tillURL(), browser, dir, launch), "\n", "\r\n")
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
