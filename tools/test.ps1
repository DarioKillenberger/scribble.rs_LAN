param(
    [switch]$Race,
    [switch]$Count3
)

$ErrorActionPreference = "Stop"

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    throw "Go is not on PATH. Install Go 1.25.0 or later, then reopen the shell."
}

$env:GOCACHE = if ($env:GOCACHE) { $env:GOCACHE } else { Join-Path $PWD ".go-build" }
$env:GOMODCACHE = if ($env:GOMODCACHE) { $env:GOMODCACHE } else { Join-Path $PWD ".go-mod" }
New-Item -ItemType Directory -Path $env:GOCACHE -Force | Out-Null
New-Item -ItemType Directory -Path $env:GOMODCACHE -Force | Out-Null

if ($Race -and -not (Get-Command gcc -ErrorAction SilentlyContinue)) {
    throw "Race tests on Windows require CGO and gcc on PATH. Install a MinGW-w64 toolchain or run without -Race."
}

gofmt -w `
    internal/api/createparse.go `
    internal/api/createparse_test.go `
    internal/api/http.go `
    internal/api/lan.go `
    internal/api/v1.go `
    internal/api/ws.go `
    internal/config/config.go `
    internal/frontend/http.go `
    internal/frontend/index.go `
    internal/frontend/lobby.go `
    internal/game/data.go `
    internal/game/lan.go `
    internal/game/lan_test.go `
    internal/game/lobby.go `
    internal/game/shared.go `
    cmd/laninput/bridge.go `
    cmd/laninput/main.go `
    cmd/laninput/rawinput_other.go `
    cmd/laninput/rawinput_windows.go

$testArgs = @("test")
if ($Race) {
    $testArgs += "-race"
}
if ($Count3) {
    $testArgs += "-count=3"
}
$testArgs += "./..."

& go @testArgs
