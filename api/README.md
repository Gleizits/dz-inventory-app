# API REST — DZ Urban Style

API en Go que sirve el inventario de la app Android: categorías, productos con
foto, lotes (compras), ventas con validación de stock, reportes con % de
ganancia y avisos de stock en cero.

## Requisitos

- Go 1.27+
- PostgreSQL 17 y Redis 7 levantados (ver `../infra/dev.ps1`)

## Ejecutar

```powershell
# 1) base + Redis (una vez por sesión)
pwsh -File ../infra/dev.ps1 up
pwsh -File ../infra/dev.ps1 migrate

# 2) la API (lee ../infra/.env automáticamente)
go run ./cmd/api
# -> API escuchando addr=:8080
```

Desde el celular conectado por USB:

```powershell
adb reverse tcp:8080 tcp:8080
```

## Configuración

Todo por variables de entorno (con prioridad sobre `infra/.env`):

| Variable | Defecto | Descripción |
|---|---|---|
| `DZ_API_ADDR` | `:8080` | dirección de escucha |
| `DZ_DATABASE_URL` | armada de `POSTGRES_*` | PostgreSQL |
| `DZ_REDIS_URL` | `redis://127.0.0.1:6379/0` | Redis (opcional: sin él la API funciona sin caché) |
| `DZ_FOTOS_DIR` | `../storage/fotos` | dónde se guardan las fotos |
| `DZ_API_KEY` | vacío | si tiene valor, exige la cabecera `X-API-Key` |
| `DZ_STOCK_UMBRAL` | `0` | umbral de `/api/stock/bajos` |
| `DZ_CACHE_TTL` | `60s` | TTL del caché |

## Endpoints

Errores: siempre `{"error": "...", "codigo": "...", "detalles": {...}}`
(`validacion` 400, `no_encontrado` 404, `conflicto` / `stock_insuficiente` 409).

| Método | Ruta | Descripción |
|---|---|---|
| GET | `/api/salud` | health check (Postgres + Redis) |
| GET | `/api/categorias?incluir_inactivas=` | lista |
| POST | `/api/categorias` | crea `{nombre, descripcion?}` |
| PUT | `/api/categorias/{id}` | actualiza |
| DELETE | `/api/categorias/{id}` | desactiva (409 si tiene productos activos) |
| GET | `/api/productos?q=&categoria_id=&con_stock=&incluir_inactivos=` | catálogo con stock por talla |
| POST | `/api/productos` | crea `{categoria_id, lote_id, unidades_lote, nombre, descripcion?, stock:{S,M,L,XL}}` |
| GET | `/api/productos/{id}` | producto + últimos 5 lotes |
| PUT | `/api/productos/{id}` | actualiza nombre/descripción/categoría |
| DELETE | `/api/productos/{id}` | borrado lógico |
| POST | `/api/productos/{id}/foto` | multipart campo `foto` → devuelve `foto_url` y `foto_thumb_url` |
| GET | `/api/fotos/{archivo}` | sirve foto o miniatura (sin API key) |
| POST | `/api/lotes` | crea un lote por categoría `{categoria_id, fecha?, unidades, costo_total, costo_envio}` o agrega stock a un producto existente con `{producto_id, fecha?, unidades, costo_total, costo_envio, stock?}` |
| GET | `/api/lotes?categoria_id=` | lista lotes y unidades disponibles para asignar productos |
| GET | `/api/ventas?desde=&hasta=&limite=&offset=` | ventas con líneas |
| POST | `/api/ventas` | registra venta (valida stock en la API) |
| POST | `/api/ventas/{id}/anular` | anula y repone stock |
| GET | `/api/reportes/ventas?desde=&hasta=` | unidades, costo, venta, ganancia y % |
| GET | `/api/stock/bajos?umbral=` | tallas en cero (para la notificación) |

Notas de negocio:

- **Costo unitario del lote** = `(costo_total + costo_envio) / unidades`.
- **Costo de compra en reportes** = costo promedio ponderado de los lotes con
  fecha ≤ fin del rango (no se guarda snapshot por venta: mantiene la 3FN).
- Las tallas válidas son `S`, `M`, `L`, `XL`.
- El stock se valida con `FOR UPDATE`: dos ventas simultáneas no pueden vender
  las mismas unidades.

## Tests

```powershell
# preparar la base de tests (una vez)
podman exec dz-postgres psql -U dz -d postgres -c "CREATE DATABASE dz_inventory_test OWNER dz;"

$env:DZ_TEST_DATABASE_URL = "postgres://USER:PASS@127.0.0.1:5432/dz_inventory_test"
go test ./...
```

- Sin `DZ_TEST_DATABASE_URL` la prueba de integración se omite (el resto pasa
  igual).
- La integración **resetea el esquema** de `dz_inventory_test`: no apuntes ahí
  datos reales.
- Las pruebas de caché usan la base Redis 15 y se omiten si Redis no corre.

## Estructura

```
cmd/api/            entrada (config, pool, Redis, apagado ordenado)
internal/config/    .env + variables de entorno
internal/store/     acceso a PostgreSQL y reglas de negocio (transacciones)
internal/httpapi/   router chi, handlers, errores, API key, logs
internal/cache/     Redis con invalidación por generación (INCR)
internal/fotos/     redimensionado + miniatura JPEG
internal/report/    cálculos de ganancia y %
```
