<#
.SYNOPSIS
  Infraestructura local de DZ Urban Style (PostgreSQL + Redis en Podman).

.DESCRIPTION
  No usa docker-compose (no hay proveedor instalado). Levanta los contenedores
  con "podman run" de forma idempotente. Todo lo que instala queda en Podman,
  nada a nivel global del sistema.

.EXAMPLE
  ./dev.ps1 up        # levanta PostgreSQL y Redis
  ./dev.ps1 migrate   # aplica db/migrations con Flyway (contenedor efímero)
  ./dev.ps1 status    # estado de contenedores y esquema
  ./dev.ps1 down      # para los contenedores (conserva los datos)
  ./dev.ps1 reset     # borra los datos y vuelve a migrar desde cero
  ./dev.ps1 logs      # últimas líneas de PostgreSQL y Redis
#>
param(
    [Parameter(Position = 0)]
    [ValidateSet("up", "down", "status", "migrate", "reset", "logs", "sql")]
    [string]$Command = "status"
)

$ErrorActionPreference = "Stop"

$InfraDir = $PSScriptRoot
$RootDir  = Split-Path -Parent $InfraDir
$EnvFile  = Join-Path $InfraDir ".env"
$MigrationsDir = Join-Path $RootDir "db\migrations"

$NetName    = "dz-net"
$PgName     = "dz-postgres"
$RedisName  = "dz-redis"
$PgVolume   = "dz-pgdata"

$PgImage    = "docker.io/library/postgres:17-alpine"
$RedisImage = "docker.io/library/redis:7-alpine"
$FlywayImage = "docker.io/flyway/flyway:11"

function Read-EnvFile {
    if (-not (Test-Path $EnvFile)) {
        # Genera un .env la primera vez con una password aleatoria
        $bytes = New-Object byte[] 24
        [System.Security.Cryptography.RandomNumberGenerator]::Fill($bytes)
        $pw = ($bytes | ForEach-Object { "{0:x2}" -f $_ }) -join ""
        $example = Get-Content (Join-Path $InfraDir ".env.example") -Raw
        $example = $example -replace "pon-aqui-una-password-larga", $pw
        Set-Content -Path $EnvFile -Value $example -Encoding UTF8
        Write-Host "[dev] Se creo infra/.env con una password aleatoria (no se sube a git)." -ForegroundColor Yellow
    }
    Get-Content $EnvFile | ForEach-Object {
        if ($_ -match '^\s*([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.*)\s*$') {
            Set-Item -Path "env:$($matches[1])" -Value $matches[2]
        }
    }
    if (-not $env:POSTGRES_USER)     { $env:POSTGRES_USER = "dz" }
    if (-not $env:POSTGRES_DB)       { $env:POSTGRES_DB = "dz_inventory" }
    if (-not $env:POSTGRES_PORT)     { $env:POSTGRES_PORT = "5432" }
    if (-not $env:REDIS_PORT)        { $env:REDIS_PORT = "6379" }
    if (-not $env:POSTGRES_PASSWORD) { throw "Falta POSTGRES_PASSWORD en infra/.env" }
}

function Test-Podman {
    if (-not (Get-Command podman -ErrorAction SilentlyContinue)) {
        throw "No se encontro 'podman' en el PATH. Instala Podman Desktop o Podman CLI."
    }
}

function Invoke-Checked([string]$File, [string[]]$Arguments, [switch]$Quiet) {
    & $File @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "Fallo el comando: $File $($Arguments -join ' ') (exit $LASTEXITCODE)"
    }
    if (-not $Quiet) { }
}

function Ensure-Network {
    & podman network exists $NetName 2>$null
    if ($LASTEXITCODE -ne 0) {
        Invoke-Checked "podman" @("network", "create", $NetName) -Quiet
        Write-Host "[dev] Red '$NetName' creada." -ForegroundColor DarkGray
    }
}

function Test-Container([string]$Name) {
    & podman container exists $Name 2>$null
    return ($LASTEXITCODE -eq 0)
}

function Start-Postgres {
    if (Test-Container $PgName) {
        & podman start $PgName | Out-Null
        Write-Host "[dev] PostgreSQL ya existia: reanudado." -ForegroundColor DarkGray
        return
    }
    Write-Host "[dev] Creando contenedor PostgreSQL 17..." -ForegroundColor Cyan
    Invoke-Checked "podman" @(
        "run", "-d",
        "--name", $PgName,
        "--network", $NetName,
        "--network-alias", "postgres",
        "-p", "127.0.0.1:${env:POSTGRES_PORT}:5432",
        "-e", "POSTGRES_USER=$($env:POSTGRES_USER)",
        "-e", "POSTGRES_PASSWORD=$($env:POSTGRES_PASSWORD)",
        "-e", "POSTGRES_DB=$($env:POSTGRES_DB)",
        "-e", "TZ=America/Lima",
        "--volume", "${PgVolume}:/var/lib/postgresql/data",
        "--label", "dz.project=inventory",
        $PgImage
    ) -Quiet
}

function Start-Redis {
    if (Test-Container $RedisName) {
        & podman start $RedisName | Out-Null
        Write-Host "[dev] Redis ya existia: reanudado." -ForegroundColor DarkGray
        return
    }
    Write-Host "[dev] Creando contenedor Redis 7..." -ForegroundColor Cyan
    # Solo cache: sin persistencia, limite de memoria y politica LRU
    Invoke-Checked "podman" @(
        "run", "-d",
        "--name", $RedisName,
        "--network", $NetName,
        "--network-alias", "redis",
        "-p", "127.0.0.1:$($env:REDIS_PORT):6379",
        "--label", "dz.project=inventory",
        "--volume", "$(Join-Path $InfraDir 'redis.conf'):/usr/local/etc/redis/redis.conf:ro",
        $RedisImage,
        "redis-server", "/usr/local/etc/redis/redis.conf"
    ) -Quiet
}

function Wait-Postgres {
    Write-Host "[dev] Esperando a que PostgreSQL acepte conexiones..." -ForegroundColor DarkGray
    for ($i = 1; $i -le 40; $i++) {
        & podman exec $PgName pg_isready -U $env:POSTGRES_USER -d $env:POSTGRES_DB 2>$null | Out-Null
        if ($LASTEXITCODE -eq 0) { Write-Host "[dev] PostgreSQL listo." -ForegroundColor Green; return }
        Start-Sleep -Milliseconds 1500
    }
    throw "PostgreSQL no respondio en 60 segundos. Revisa: ./dev.ps1 logs"
}

function Invoke-Migrate {
    if (-not (Test-Container $PgName)) { throw "PostgreSQL no esta corriendo. Ejecuta: ./dev.ps1 up" }
    if (-not (Test-Path $MigrationsDir)) { throw "No existe $MigrationsDir" }
    Wait-Postgres
    Write-Host "[dev] Aplicando migraciones con Flyway..." -ForegroundColor Cyan
    # Contenedor efímero: solo existe durante la migración
    Invoke-Checked "podman" @(
        "run", "--rm",
        "--network", $NetName,
        "--volume", "${MigrationsDir}:/flyway/sql:ro",
        "-e", "FLYWAY_URL=jdbc:postgresql://postgres:5432/$($env:POSTGRES_DB)",
        "-e", "FLYWAY_USER=$($env:POSTGRES_USER)",
        "-e", "FLYWAY_PASSWORD=$($env:POSTGRES_PASSWORD)",
        "-e", "FLYWAY_CLEAN_DISABLED=false",
        $FlywayImage,
        "migrate"
    )
    Write-Host "[dev] Migraciones aplicadas." -ForegroundColor Green
}

function Show-Status {
    Write-Host ""
    Write-Host "Contenedores:" -ForegroundColor Cyan
    & podman ps -a --filter "label=dz.project=inventory" --format "table {{.Names}}`t{{.Status}}`t{{.Ports}}"
    if (Test-Container $PgName) {
        Write-Host ""
        Write-Host "Esquema (Flyway):" -ForegroundColor Cyan
        & podman exec $PgName psql -U $env:POSTGRES_USER -d $env:POSTGRES_DB -Atc `
            "select version || ' | ' || description from flyway_schema_history order by installed_rank;" 2>$null
        if ($LASTEXITCODE -ne 0) {
            Write-Host "  (aun sin migrar: ./dev.ps1 migrate)" -ForegroundColor DarkGray
        }
        Write-Host ""
        Write-Host "Tablas:" -ForegroundColor Cyan
        & podman exec $PgName psql -U $env:POSTGRES_USER -d $env:POSTGRES_DB -Atc `
            "select string_agg(tablename, ', ' order by tablename) from pg_tables where schemaname='public';" 2>$null
    }
    Write-Host ""
    Write-Host "API Go esperada en http://localhost:$($env:API_PORT) (arrancala con: go run ./api)" -ForegroundColor DarkGray
    Write-Host "Desde el celular via USB: adb reverse tcp:$($env:API_PORT) tcp:$($env:API_PORT)" -ForegroundColor DarkGray
    Write-Host ""
}

function Show-Logs {
    if (Test-Container $PgName)    { Write-Host "--- postgres ---" -ForegroundColor Cyan; & podman logs --tail 40 $PgName }
    if (Test-Container $RedisName) { Write-Host "--- redis ---" -ForegroundColor Cyan; & podman logs --tail 40 $RedisName }
}

function Stop-All {
    foreach ($n in @($RedisName, $PgName)) {
        if (Test-Container $n) {
            & podman stop $n | Out-Null
            Write-Host "[dev] '$n' detenido (datos conservados)." -ForegroundColor DarkGray
        }
    }
}

function Reset-All {
    Write-Host "[dev] BORRANDO datos (volumen $PgVolume)..." -ForegroundColor Yellow
    foreach ($n in @($RedisName, $PgName)) {
        if (Test-Container $n) { & podman rm -f $n | Out-Null }
    }
    & podman volume exists $PgVolume 2>$null
    if ($LASTEXITCODE -eq 0) { & podman volume rm $PgVolume | Out-Null }
    Start-Postgres
    Start-Redis
    Wait-Postgres
    Invoke-Migrate
}

Test-Podman
Read-EnvFile

switch ($Command) {
    "up" {
        Ensure-Network
        Start-Postgres
        Start-Redis
        Wait-Postgres
        Write-Host ""
        Write-Host "[dev] PostgreSQL 127.0.0.1:$($env:POSTGRES_PORT) | Redis 127.0.0.1:$($env:REDIS_PORT)" -ForegroundColor Green
        Write-Host "[dev] Siguiente: ./dev.ps1 migrate" -ForegroundColor Green
    }
    "migrate"  { Invoke-Migrate }
    "status"   { Show-Status }
    "logs"     { Show-Logs }
    "down"     { Stop-All }
    "reset"    { Reset-All }
}
