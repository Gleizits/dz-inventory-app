# DZ Urban Style · App de inventario

Aplicación Android en Jetpack Compose para controlar categorías, lotes,
productos, stock por talla, ventas, anulaciones y reportes de DZ Urban Style.
La app consume una API REST en Go, que usa PostgreSQL y Redis.

## Arquitectura

```text
Android Compose ──HTTP local (debug)──► API Go :8080
                                      ├── PostgreSQL 17
                                      └── Redis 7
```

- **Android:** Kotlin, Jetpack Compose y Material 3.
- **API:** Go, router chi y PostgreSQL.
- **Base de datos:** migraciones SQL versionadas con Flyway.
- **Fotos:** la API genera la foto principal y una miniatura.
- **Flujo de inventario:** categoría → lote → uno o varios productos dentro del
  lote → stock por talla.
- **Reportes:** ventas, costo promedio ponderado, ganancia y porcentaje.

## Requisitos

- Android Studio con SDK Android instalado.
- JDK 11.
- Go 1.27 o superior.
- Podman Desktop o Podman CLI.
- Un dispositivo Android o emulador.
- Para probar desde un dispositivo físico: `adb` y depuración USB.

## Inicio rápido en desarrollo

Desde PowerShell, en la raíz del proyecto:

```powershell
# 1. Levantar PostgreSQL 17 y Redis 7
.\infra\dev.ps1 up

# 2. Aplicar las migraciones
.\infra\dev.ps1 migrate

# 3. Iniciar la API
Push-Location .\api
go run .\cmd\api
Pop-Location
```

La API queda disponible en `http://127.0.0.1:8080`.

Para un dispositivo conectado por USB:

```powershell
$adb = "$env:LOCALAPPDATA\Android\Sdk\platform-tools\adb.exe"
& $adb reverse tcp:8080 tcp:8080
```

Después, ejecuta la variante `debug` desde Android Studio o instala el APK:

```powershell
.\gradlew.bat assembleDebug
& $adb install -r .\app\build\outputs\apk\debug\app-debug.apk
& $adb shell monkey -p store.dz.dz_inventory_app 1
```

### HTTP claro en desarrollo

La variante `debug` permite HTTP para trabajar con `adb reverse` y
`127.0.0.1`. La configuración está únicamente en:

- `app/src/debug/AndroidManifest.xml`
- `app/src/debug/res/xml/network_security_config.xml`

La variante `release` no incluye esta excepción. No uses la variante `debug`
como APK de producción.

## Comandos útiles

| Comando | Uso |
|---|---|
| `.\infra\dev.ps1 status` | Estado de contenedores y migraciones |
| `.\infra\dev.ps1 logs` | Logs de PostgreSQL y Redis |
| `.\infra\dev.ps1 migrate` | Aplicar migraciones Flyway |
| `.\infra\dev.ps1 down` | Detener la infraestructura local |
| `.\infra\dev.ps1 reset` | Borrar la base local y migrar desde cero |
| `.\gradlew.bat assembleDebug` | Compilar APK de desarrollo |
| `.\gradlew.bat assembleRelease` | Compilar APK de producción |
| `.\gradlew.bat test` | Ejecutar tests Android |
| `go test ./...` | Ejecutar tests de la API |
| `pwsh -File .\api\smoke.ps1` | Prueba punta a punta de la API |

Para probar la API:

```powershell
Push-Location .\api
go test ./...
Pop-Location
```

La prueba de integración requiere una base separada configurada mediante
`DZ_TEST_DATABASE_URL`. No apuntes esa variable a datos reales: la prueba
restablece su esquema.

## Flujo de uso

1. Crear una categoría.
2. Seleccionar la categoría.
3. Crear un lote con unidades totales, precio de compra y envío.
4. Seleccionar el lote.
5. Crear uno o varios productos dentro del lote.
6. Registrar foto, unidades y tallas de cada producto.
7. Registrar ventas y anularlas cuando corresponda.
8. Consultar reportes y exportarlos a PDF.

La suma de las tallas de cada producto debe coincidir con sus unidades
asignadas y no puede superar las unidades disponibles del lote.

## API

La documentación detallada de endpoints está en [`api/README.md`](api/README.md).

Endpoints principales:

- `GET /api/salud`
- `GET|POST /api/categorias`
- `GET|POST /api/productos`
- `GET|POST /api/lotes`
- `GET|POST /api/ventas`
- `POST /api/ventas/{id}/anular`
- `GET /api/reportes/ventas`
- `GET /api/stock/bajos`

La API devuelve errores con la forma:

```json
{
  "error": "mensaje",
  "codigo": "validacion",
  "detalles": {}
}
```

## Configuración

No subas `infra/.env`. Usa [`infra/.env.example`](infra/.env.example) como
plantilla para desarrollo. Las variables de entorno tienen prioridad sobre
ese archivo.

Variables principales:

- `DZ_DATABASE_URL`
- `DZ_REDIS_URL`
- `DZ_API_ADDR`
- `DZ_FOTOS_DIR`
- `DZ_API_KEY`
- `DZ_STOCK_UMBRAL`
- `DZ_CACHE_TTL`

## Documentación de producción

La guía de despliegue está en [`DESPLEGAR.md`](DESPLEGAR.md). Incluye
PostgreSQL administrado, el binario Go, fotos, variables de entorno, HTTPS y
la configuración necesaria para una APK de producción.

## Estado del proyecto

- Fases 0 a 5 completadas.
- La base de desarrollo usa PostgreSQL 17 y Redis 7 en Podman.
- La base de desarrollo no incluye seed obligatorio.
- La rama de trabajo actual es `develop`.

