<#
.SYNOPSIS
  Regenera los diagramas de la base de datos en imgs/*.png

.DESCRIPTION
  Usa Mermaid CLI instalado localmente en diagrams/node_modules (no global).
  Requiere haber ejecutado: npm install  (dentro de diagrams/)

.EXAMPLE
  ./render.ps1            # regenera los dos PNG
  ./render.ps1 -Logico    # solo el lógico
#>
param(
    [switch]$Logico,
    [switch]$Fisico
)
$ErrorActionPreference = "Stop"

$here = $PSScriptRoot
$root = Split-Path -Parent $here
$imgs = Join-Path $root "imgs"
$mmdc = Join-Path $here "node_modules\.bin\mmdc.cmd"
$cfg  = Join-Path $here "puppeteer-config.json"

if (-not (Test-Path $mmdc)) {
    throw "No esta Mermaid CLI. Ejecuta 'npm install' dentro de diagrams/."
}
if (-not (Test-Path $imgs)) { New-Item -ItemType Directory -Path $imgs | Out-Null }

if (-not $Fisico) {
    Write-Host "Generando imgs/diagrama-logico.png ..." -ForegroundColor Cyan
    & $mmdc -i (Join-Path $here "diagrama-logico.mmd") `
            -o (Join-Path $imgs "diagrama-logico.png") `
            -c $cfg -w 2200 -s 2 -b white
    if ($LASTEXITCODE -ne 0) { throw "Fallo el diagrama logico" }
}
if (-not $Logico) {
    Write-Host "Generando imgs/diagrama-fisico.png ..." -ForegroundColor Cyan
    & $mmdc -i (Join-Path $here "diagrama-fisico.mmd") `
            -o (Join-Path $imgs "diagrama-fisico.png") `
            -c $cfg -w 2600 -s 2 -b white
    if ($LASTEXITCODE -ne 0) { throw "Fallo el diagrama fisico" }
}

Get-ChildItem (Join-Path $imgs "*.png") | ForEach-Object {
    Write-Host ("  {0}  {1} KB" -f $_.Name, [math]::Round($_.Length / 1KB))
}
