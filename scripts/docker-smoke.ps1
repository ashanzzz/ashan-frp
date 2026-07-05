param(
    [string]$ComposeFile = "compose.yaml",
    [string]$ServiceName = "ashan-frp",
    [string]$BaseUrl = "http://127.0.0.1:8080",
    [switch]$KeepRunning
)

$ErrorActionPreference = 'Stop'

function Assert-JsonEndpoint {
    param(
        [string]$Url,
        [int]$ExpectedStatus = 200
    )

    $response = Invoke-WebRequest -Uri $Url -UseBasicParsing -MaximumRedirection 0 -SkipHttpErrorCheck
    if ($response.StatusCode -ne $ExpectedStatus) {
        throw "Expected HTTP $ExpectedStatus for $Url but got $($response.StatusCode)"
    }
    return $response
}

function Assert-Redirect {
    param(
        [string]$Url,
        [string]$ExpectedLocation
    )

    $response = Invoke-WebRequest -Uri $Url -UseBasicParsing -MaximumRedirection 0 -SkipHttpErrorCheck
    if ($response.StatusCode -lt 300 -or $response.StatusCode -ge 400) {
        throw "Expected redirect for $Url but got HTTP $($response.StatusCode)"
    }
    $actual = $response.Headers.Location
    if ($actual -ne $ExpectedLocation) {
        throw "Expected redirect location $ExpectedLocation for $Url but got $actual"
    }
}

function Wait-ForHealthyContainer {
    param(
        [string]$ContainerName,
        [int]$TimeoutSeconds = 90
    )

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        $status = docker inspect --format "{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}" $ContainerName 2>$null
        if ($LASTEXITCODE -eq 0 -and ($status -eq 'healthy' -or $status -eq 'running')) {
            return
        }
        Start-Sleep -Seconds 2
    }

    throw "Container $ContainerName did not become healthy within $TimeoutSeconds seconds"
}

try {
    docker compose -f $ComposeFile up -d --build
    if ($LASTEXITCODE -ne 0) {
        throw "docker compose up failed"
    }

    Wait-ForHealthyContainer -ContainerName $ServiceName

    $health = Assert-JsonEndpoint -Url "$BaseUrl/api/v1/health"
    $version = Assert-JsonEndpoint -Url "$BaseUrl/api/v1/version"
    $openapi = Assert-JsonEndpoint -Url "$BaseUrl/api/v1/openapi.json"
    $docs = Assert-JsonEndpoint -Url "$BaseUrl/api/v1/docs"

    Assert-Redirect -Url "$BaseUrl/api/openapi.json" -ExpectedLocation "/api/v1/openapi.json"
    Assert-Redirect -Url "$BaseUrl/api/docs" -ExpectedLocation "/api/v1/docs"

    $summary = [ordered]@{
        health_status = $health.StatusCode
        version_status = $version.StatusCode
        openapi_status = $openapi.StatusCode
        docs_status = $docs.StatusCode
        compatibility_aliases = 'ok'
    } | ConvertTo-Json -Depth 3

    $summary
}
finally {
    if (-not $KeepRunning) {
        docker compose -f $ComposeFile down
    }
}
