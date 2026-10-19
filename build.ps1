<#
  build.ps1 - Windows counterpart of the Makefile (Windows has no make / bash).

  Run from the repo root in PowerShell:

      .\build.ps1                 # numbered menu
      .\build.ps1 <target> [args] # one target directly

  Targets:
      builder [args]     the Shop Builder (opens in the browser; flags pass through)
      shop <key|all>     build a shop from shops.json: dist\<key>\ = karots-setup, install.conf, the binary
      shops              list the shops in shops.json
      build              static karots-suite.exe in bin\ (builds the web app first)
      web                build the web app into internal\spa\dist
      types              regenerate web\src\api\types.gen.ts (tygo)
      run                run the server (reads .env)
      test | test-web    go test ./...  /  the web unit tests
      support-pin <ID>   print a shop's current support PIN

  Needs Go and bun on PATH (tygo for `types`). Shop builds need POS_SUPPORT_SECRET
  in .env — every shop gets its own support PIN.
#>
[CmdletBinding()]
param(
  [Parameter(Position = 0)][string]$Target = "",
  [Parameter(Position = 1, ValueFromRemainingArguments = $true)][string[]]$Rest
)

$ErrorActionPreference = "Stop"
if (-not (Test-Path "go.mod")) { Write-Host "Run from the repository root (go.mod not found)." -ForegroundColor Red; exit 1 }

function Build-Web {
  Push-Location web
  try { bun install --frozen-lockfile; bun run build } finally { Pop-Location }
  New-Item -ItemType File -Force -Path "internal\spa\dist\.gitkeep" | Out-Null   # vite empties the folder
}

$Menu = @(
  @{ t = "builder";     d = "Open the Shop Builder" }
  @{ t = "shop";        d = "Build a shop from shops.json (three files in dist\<key>)" }
  @{ t = "shops";       d = "List the shops" }
  @{ t = "build";       d = "Build bin\karots-suite.exe" }
  @{ t = "web";         d = "Build the web app" }
  @{ t = "types";       d = "Regenerate the API types" }
  @{ t = "run";         d = "Run the server locally" }
  @{ t = "test";        d = "go test ./..." }
  @{ t = "test-web";    d = "Web unit tests" }
  @{ t = "support-pin"; d = "Print a shop's current support PIN" }
)

function Invoke-Target($t, $rest) {
  switch ($t) {
    "builder"     { go run ./cmd/bootstrap @rest }
    "shop"        { go run ./cmd/bootstrap -shop @rest }
    "shops"       { go run ./cmd/bootstrap -list }
    "web"         { Build-Web }
    "types"       { tygo generate }
    "build" {
      Build-Web
      $env:CGO_ENABLED = "0"; $env:GOAMD64 = "v1"   # static, runs on old CPUs too
      go build -ldflags="-s -w" -o bin\karots-suite.exe ./cmd/server
      Write-Host "OK  bin\karots-suite.exe" -ForegroundColor Green
    }
    "run"         { go run ./cmd/server }
    "test"        { go test ./... }
    "test-web"    { Push-Location web; try { bun run test } finally { Pop-Location } }
    "support-pin" { go run ./cmd/server -support-pin @rest }
    default {
      Write-Host "Unknown target '$t'." -ForegroundColor Red
      Write-Host "Targets: $(($Menu | ForEach-Object { $_.t }) -join ' ')"
      exit 1
    }
  }
}

if ($Target) { Invoke-Target $Target $Rest; return }

Write-Host ""
Write-Host "Karots Suite build - pick a target:" -ForegroundColor Cyan
for ($i = 0; $i -lt $Menu.Count; $i++) {
  "{0,2}. {1,-12} {2}" -f ($i + 1), $Menu[$i].t, $Menu[$i].d | Write-Host
}
$sel = Read-Host "`nNumber (or blank to cancel)"
if (-not $sel) { return }
$n = 0
if (-not [int]::TryParse($sel, [ref]$n) -or $n -lt 1 -or $n -gt $Menu.Count) {
  Write-Host "Not a valid choice." -ForegroundColor Red; exit 1
}
$chosen = $Menu[$n - 1].t
$rest = @()
if ($chosen -eq "shop") {
  $key = Read-Host "Shop key (or all)"
  if (-not $key) { Write-Host "No shop given." -ForegroundColor Red; exit 1 }
  $rest = @($key)
}
if ($chosen -eq "support-pin") {
  $id = Read-Host "Shop install ID"
  if (-not $id) { Write-Host "No ID given." -ForegroundColor Red; exit 1 }
  $rest = @($id)
}
Invoke-Target $chosen $rest
