# Desplegar DZ Urban Style

Esta guía describe cómo pasar la API y la base de datos a producción. La
arquitectura prevista es:

```text
APK Android ──HTTPS──► API Go en Render Free
                       ├── PostgreSQL de Supabase
                       └── Supabase Storage (bucket privado `fotos`)
```

Despliegue actual:

- API: `https://dz-inventory-api.onrender.com`
- Servicio Render: `dz-inventory-api`
- Rama desplegada: `develop`
- Directorio raíz: `api`
- Redis: opcional; Render Free funciona sin caché
- Health check público: `/api/salud`

## 1. Preparar PostgreSQL de producción

1. Crear un proyecto en Supabase.
2. Copiar la cadena de conexión PostgreSQL de Supabase.
3. Confirmar la versión de PostgreSQL ofrecida por Supabase antes de ejecutar
   migraciones.
4. Aplicar las migraciones de `db/migrations` desde un equipo que tenga acceso
   a la base:

```powershell
$env:DZ_DATABASE_URL = "postgres://USUARIO:CONTRASENA@HOST:5432/BASE?sslmode=require"
Push-Location .\api
go run .\cmd\api
Pop-Location
```

Para aplicar Flyway directamente contra Supabase desde PowerShell:

```powershell
$SUPABASE_PASSWORD = Read-Host "Contraseña actual de Supabase"

podman run --rm `
  --volume "${PWD}\db\migrations:/flyway/sql:ro" `
  -e FLYWAY_URL="jdbc:postgresql://HOST_POOLER:5432/postgres?sslmode=require" `
  -e FLYWAY_USER="USUARIO_POOLER" `
  -e FLYWAY_PASSWORD="$SUPABASE_PASSWORD" `
  docker.io/flyway/flyway:11 migrate

Remove-Variable SUPABASE_PASSWORD
```

Usa el Session Pooler de Supabase. No ejecutes `reset` contra producción:
esa operación elimina el volumen local y está destinada únicamente a
desarrollo. La versión actual de producción es V3 `seguridad`.

La base debe empezar sin datos de ejemplo. Las categorías, lotes y productos
se registran desde la aplicación.

## 2. Compilar la API Go

Desde la raíz del repositorio:

```powershell
Push-Location .\api
go test ./...
go vet ./...
go build -o .\dz-inventory-api.exe .\cmd\api
Pop-Location
```

En Linux, compila el binario para el sistema de destino:

```bash
cd api
go test ./...
go vet ./...
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o dz-inventory-api ./cmd/api
```

El binario necesita acceso de red a PostgreSQL y Redis. Redis es opcional: si
no está disponible, la API funciona sin caché.

## 3. Configurar la API en Render

En Render, abre `dz-inventory-api` → **Environment** y define las variables
sin subirlas al repositorio:

```text
DZ_API_ADDR=:8080
DZ_DATABASE_URL=postgres://USUARIO:CONTRASENA@HOST:5432/BASE?sslmode=require
DZ_REDIS_URL=redis://HOST_REDIS:6379/0
DZ_FOTOS_DIR=/tmp/dz-inventory-fotos
DZ_SUPABASE_URL=https://TU-PROYECTO.supabase.co
DZ_SUPABASE_SERVICE_ROLE_KEY=CLAVE_SERVICE_ROLE_SOLO_EN_RENDER
DZ_SUPABASE_FOTOS_BUCKET=fotos
DZ_API_KEY=CAMBIA_ESTA_CLAVE
DZ_STOCK_UMBRAL=0
DZ_CACHE_TTL=60s
DZ_FOTO_ANCHO_MAX=1600
DZ_FOTO_ANCHO_THUMB=320
```

Recomendaciones:

- Usa una cuenta de base de datos con los permisos mínimos necesarios.
- Define `DZ_API_KEY` en producción y configura el cliente Android para
  enviarla.
- No expongas `DZ_SUPABASE_SERVICE_ROLE_KEY` en Android ni en el repositorio.
- Crea en Supabase Storage un bucket privado llamado `fotos`. La API usa la
  clave `service_role` para subir y servir las imágenes mediante sus propios
  endpoints.
- Si no configuras Supabase Storage, crea `DZ_FOTOS_DIR` en un volumen
  persistente. Render Free no ofrece ese volumen, por lo que las fotos locales
  se perderían al reiniciar.
- No copies `infra/.env` del entorno local al servidor.
- Render usa `DZ_API_ADDR=0.0.0.0:10000`, build command
  `go build -o dz-inventory-api ./cmd/api`, start command
  `./dz-inventory-api` y health check `/api/salud`.
- Render Free no conserva `/tmp` entre reinicios; Storage debe estar
  configurado para producción.

Ejemplo de ejecución manual:

```bash
export DZ_API_ADDR=:8080
export DZ_DATABASE_URL='postgres://...'
export DZ_REDIS_URL='redis://...'
export DZ_FOTOS_DIR='/var/lib/dz-inventory/fotos'
export DZ_SUPABASE_URL='https://TU-PROYECTO.supabase.co'
export DZ_SUPABASE_SERVICE_ROLE_KEY='...'
export DZ_SUPABASE_FOTOS_BUCKET='fotos'
export DZ_API_KEY='...'
./dz-inventory-api
```

## 4. HTTPS y proxy inverso

La API Go puede escuchar en un puerto interno, pero la aplicación Android de
producción debe consumir una URL HTTPS pública. Configura un proxy inverso
con un certificado válido y reenvía hacia `127.0.0.1:8080`.

El permiso de HTTP claro de Android está limitado a la variante `debug`.
Nunca habilites `usesCleartextTraffic` ni `networkSecurityConfig` de desarrollo
en la variante `release`.

Antes de publicar una APK de producción, verifica que el cliente Android use
la URL HTTPS del host y no `http://127.0.0.1:8080`. La versión actual usa esa
URL para desarrollo con `adb reverse`; la parametrización de la URL de
producción debe quedar definida antes de publicar la APK.

Verifica el proxy:

```powershell
Invoke-WebRequest https://api.ejemplo.com/api/salud
```

Debe responder con el estado de salud de PostgreSQL y Redis. No expongas
PostgreSQL ni Redis directamente a Internet.

## 5. Fotos con Supabase Storage

1. En Supabase abre **Storage** y crea un bucket privado llamado `fotos`.
2. En **Project Settings → API**, copia la clave secreta `service_role`.
3. En Render añade `DZ_SUPABASE_URL`, `DZ_SUPABASE_SERVICE_ROLE_KEY` y
   `DZ_SUPABASE_FOTOS_BUCKET=fotos`.
4. Guarda los cambios y espera el redeploy.

La API redimensiona cada imagen, sube `{nombre}.jpg` y
`{nombre}_thumb.jpg` al bucket, y mantiene `/api/fotos/{archivo}` para que la
app no cambie. La clave `service_role` nunca debe entrar en la APK.

Si las variables de Supabase Storage no están configuradas, la API usa el
disco indicado por `DZ_FOTOS_DIR` como fallback.

## 6. Fotos sin almacenamiento externo

El fallback local guarda fotos en `DZ_FOTOS_DIR` y sirve:

- `/api/fotos/{archivo}`
- `/api/productos/{id}/foto`

No uses un disco efímero en producción si no has configurado Supabase Storage:
las fotos se perderían al reiniciar o reemplazar el host.

## 7. Configurar y firmar la APK

La variante `release` usa:

- API: `https://dz-inventory-api.onrender.com`
- Cabecera `X-API-Key`
- HTTP claro desactivado
- Nombre visible: `DZ Inventario`

La API key se proporciona solo durante la compilación mediante una variable
local. No la guardes en Git:

```powershell
$env:DZ_API_KEY = (Get-Content .\infra\.dz-api-key -Raw).Trim()
.\gradlew.bat clean assembleRelease
Remove-Item Env:DZ_API_KEY
```

El keystore de release y `infra/release-signing.properties` son locales y
están excluidos por `.gitignore`. Conserva ambos para futuras actualizaciones.
La APK queda en:

```text
app\build\outputs\apk\release\app-release.apk
```

## 8. Verificación antes de publicar

Ejecuta estas comprobaciones:

```powershell
Push-Location .\api
go test ./...
go vet ./...
Pop-Location

.\gradlew.bat assembleRelease test
```

Después verifica manualmente:

1. `GET /api/salud`.
2. Crear una categoría, lote y dos productos dentro del mismo lote.
3. Confirmar que no se puedan asignar más unidades que las del lote.
4. Registrar una venta y anularla.
5. Confirmar que los reportes no incluyan productos desactivados.
6. Subir una foto y descargar la foto principal y la miniatura.
7. Probar la APK contra HTTPS, sin `adb reverse`.

## 9. Operación y mantenimiento

- Conserva copias de seguridad de PostgreSQL según la política de Supabase.
- Conserva el volumen de fotos y respáldalo junto con la base.
- Revisa los logs del proceso Go y del proxy inverso.
- Aplica nuevas migraciones Flyway antes de iniciar una versión que dependa
  de ellas.
- No ejecutes migraciones destructivas sobre producción sin una copia de
  seguridad y una revisión previa.

