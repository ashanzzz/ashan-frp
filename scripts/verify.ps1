[CmdletBinding()]
param(
    [switch]$SkipVet,
    [switch]$SkipBuild,
    [switch]$Docker
)

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
$backend = Join-Path $repoRoot 'frp-backend'

function Invoke-Step {
    param([string]$Name, [scriptblock]$Action)
    Write-Host "`n==> $Name" -ForegroundColor Cyan
    & $Action
    if ($LASTEXITCODE -ne 0) {
        throw "$Name failed with exit code $LASTEXITCODE"
    }
}

Push-Location $backend
try {
    Invoke-Step 'Embedded UI tests' { node --test internal/web/app.test.mjs }
    Invoke-Step 'Go tests' { go test ./... }
    if (-not $SkipVet) {
        Invoke-Step 'Go vet' { go vet ./... }
    }
    if (-not $SkipBuild) {
        $output = Join-Path ([IO.Path]::GetTempPath()) 'ashan-frp-verify.exe'
        Invoke-Step 'Production build' { go build -trimpath -o $output ./cmd/ashan-frp }
        Remove-Item -LiteralPath $output -Force -ErrorAction SilentlyContinue
    }
}
finally {
    Pop-Location
}

Invoke-Step 'Git whitespace check' { git -C $repoRoot diff --check }

if ($Docker) {
    Invoke-Step 'Docker smoke test' {
        & (Join-Path $PSScriptRoot 'docker-smoke.ps1')
    }
}

Write-Host "`nAll requested checks passed." -ForegroundColor Green
