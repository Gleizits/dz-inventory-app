# Prueba punta a punta de la API de DZ Urban Style (20 checks).
#
# Uso: levantar la API en otra terminal con `go run ./cmd/api` y luego
#   pwsh -File api/smoke.ps1
#
# OJO: crea categorías, productos, ventas y fotos de prueba en la base de
# desarrollo. Para dejarla como al inicio (sin seed):
#   podman exec dz-postgres psql -U dz -d dz_inventory -c \
#     "TRUNCATE detalle_venta, venta, producto_talla, lote, producto, categoria RESTART IDENTITY CASCADE;"
$ErrorActionPreference = "Stop"
$base = "http://127.0.0.1:8080"

function Pedir([string]$Metodo, [string]$Ruta, $Cuerpo = $null, [string]$Tipo = "application/json") {
    # SkipHttpErrorCheck: los 4xx/5xx vuelven como respuesta normal y se lee
    # el cuerpo del error sin depender de excepciones desechadas.
    $params = @{ Method = $Metodo; Uri = "$base$Ruta"; TimeoutSec = 15; SkipHttpErrorCheck = $true }
    if ($null -ne $Cuerpo) {
        $params.Body = if ($Cuerpo -is [string]) { $Cuerpo } else { $Cuerpo | ConvertTo-Json -Depth 6 }
        $params.ContentType = $Tipo
    }
    try {
        $r = Invoke-WebRequest @params
        return @{ Status = [int]$r.StatusCode; Body = $r.Content }
    } catch {
        return @{ Status = 0; Body = "$_" }
    }
}

function Ver([string]$Nombre, $Resultado, $Esperado) {
    $ok = $Resultado.Status -eq $Esperado
    $marca = if ($ok) { "OK  " } else { "FALLA" }
    $cuerpo = "$($Resultado.Body)"
    if ($cuerpo.Length -gt 220) { $cuerpo = $cuerpo.Substring(0, 220) + "..." }
    Write-Output ("[{0}] {1} -> {2}  {3}" -f $marca, $Nombre, $Resultado.Status, $cuerpo)
    if (-not $ok) { $script:fallas++ }
}

$script:fallas = 0

# esperar a que la API esté viva
$disponible = $false
for ($i = 0; $i -lt 20; $i++) {
    $salud = Pedir GET "/api/salud"
    if ($salud.Status -ne 0) { $disponible = $true; break }
    Start-Sleep -Milliseconds 500
}
if (-not $disponible) { Write-Output "La API no respondió en 10 s"; exit 1 }
Ver "GET /api/salud" $salud 200

# categorías
$c = Pedir POST "/api/categorias" @{ nombre = "Polos oversize"; descripcion = "Prueba Fase 1" }
Ver "POST /api/categorias" $c 201
$catId = ($c.Body | ConvertFrom-Json).id

$c2 = Pedir POST "/api/categorias" @{ nombre = "Polos oversize" }
Ver "POST /api/categorias duplicada (409)" $c2 409

# producto con stock y lote (costo 120 + envio 20 en 10 unidades = 14 c/u)
$p = Pedir POST "/api/productos" @{
    categoria_id = $catId
    nombre       = "Polo negro oversize"
    stock        = @{ S = 4; M = 6; XL = 0 }
    lote         = @{ unidades = 10; costo_total = 120; costo_envio = 20 }
}
Ver "POST /api/productos (con lote)" $p 201
$prod = $p.Body | ConvertFrom-Json
$prodId = $prod.producto.id
Write-Output ("       unidades={0} costo_unitario={1}" -f $prod.producto.unidades, $prod.lotes[0].costo_unitario)

$pg = Pedir GET "/api/productos/$prodId"
Ver "GET /api/productos/{id}" $pg 200

# venta válida: 2 unidades M a 18
$v = Pedir POST "/api/ventas" @{ lineas = @(@{ producto_id = $prodId; talla = "M"; cantidad = 2; precio_unitario = 18 }) }
Ver "POST /api/ventas" $v 201
$venta = $v.Body | ConvertFrom-Json
$ventaId = $venta.id
Write-Output ("       total={0} unidades={1}" -f $venta.total, $venta.unidades)

# venta que supera el stock
$vs = Pedir POST "/api/ventas" @{ lineas = @(@{ producto_id = $prodId; talla = "M"; cantidad = 99; precio_unitario = 18 }) }
Ver "POST /api/ventas sin stock (409)" $vs 409

# talla inválida
$vt = Pedir POST "/api/ventas" @{ lineas = @(@{ producto_id = $prodId; talla = "G"; cantidad = 1; precio_unitario = 18 }) }
Ver "POST /api/ventas talla invalida (400)" $vt 400

# lote posterior con reparto por talla
$l = Pedir POST "/api/lotes" @{ producto_id = $prodId; unidades = 6; costo_total = 78; costo_envio = 6; stock = @{ S = 3; M = 3 } }
Ver "POST /api/lotes" $l 201

# reporte
$r = Pedir GET "/api/reportes/ventas"
Ver "GET /api/reportes/ventas" $r 200
$rep = $r.Body | ConvertFrom-Json
Write-Output ("       unidades={0} venta={1} costo={2} ganancia={3} %={4}" -f `
    $rep.total.unidades_vendidas, $rep.total.venta_total, $rep.total.costo_compra, `
    $rep.total.ganancia, $rep.total.porcentaje_ganancia)

# stock bajos (XL quedó en 0)
$sb = Pedir GET "/api/stock/bajos"
Ver "GET /api/stock/bajos" $sb 200
Write-Output ("       " + $sb.Body)

# foto (usamos el logo de la marca como si fuera la prenda)
$logo = Join-Path $PSScriptRoot "..\imgs\DZ LOGO TIKTOK.png"
try {
    $fr = Invoke-WebRequest -Method Post -Uri "$base/api/productos/$prodId/foto" `
        -Form @{ foto = Get-Item $logo } -TimeoutSec 30 -SkipHttpErrorCheck
    Write-Output ("[OK  ] POST /api/productos/{{id}}/foto -> {0}  {1}" -f [int]$fr.StatusCode, $fr.Content)
    if ([int]$fr.StatusCode -ne 200) { $script:fallas++ }
    $foto = $fr.Content | ConvertFrom-Json
    $im = Invoke-WebRequest -Uri "$base$($foto.foto_thumb_url)" -TimeoutSec 15
    Write-Output ("[OK  ] GET {0} -> {1} ({2} bytes)" -f $foto.foto_thumb_url, $im.StatusCode, $im.RawContentLength)
    $im2 = Invoke-WebRequest -Uri "$base$($foto.foto_url)" -TimeoutSec 15
    Write-Output ("[OK  ] GET {0} -> {1} ({2} bytes)" -f $foto.foto_url, $im2.StatusCode, $im2.RawContentLength)
} catch {
    Write-Output ("[FALLA] foto: {0}" -f $_)
    $script:fallas++
}

# anulación repone stock
$an = Pedir POST "/api/ventas/$ventaId/anular"
Ver "POST /api/ventas/{id}/anular" $an 200
$an2 = Pedir POST "/api/ventas/$ventaId/anular"
Ver "POST anular dos veces (409)" $an2 409

$pg2 = Pedir GET "/api/productos/$prodId"
$p2 = $pg2.Body | ConvertFrom-Json
$stockM = ($p2.producto.stock | Where-Object { $_.talla -eq "M" }).cantidad
# M: 6 inicial - 2 vendidos + 3 del lote posterior + 2 repuestos al anular = 9
Write-Output ("       stock M tras anular = {0} (esperado 9 = 6 - 2 + 3 lote + 2 anulado)" -f $stockM)
if ($stockM -ne 9) { $script:fallas++ }

# caché: la generación debe haber creado por cada escritura
$s2 = Pedir GET "/api/salud"
$s2j = $s2.Body | ConvertFrom-Json
Write-Output ("       cache_generacion = {0} (Redis operativo: {1})" -f $s2j.cache_generacion, $s2j.redis)

# errores de formato
$mal = Pedir POST "/api/categorias" '{ "nombre": "x", "campo_raro": 1 }'
Ver "POST con campo desconocido (400)" $mal 400
$mal2 = Pedir GET "/api/productos/abc"
Ver "GET id invalido (400)" $mal2 400
$noExiste = Pedir GET "/api/productos/999999"
Ver "GET producto inexistente (404)" $noExiste 404

Write-Output ""
if ($script:fallas -eq 0) {
    Write-Output "TODO OK: $((Get-Date).ToString('HH:mm:ss'))"
} else {
    Write-Output "HUBO $script:fallas FALLAS"
    exit 1
}
