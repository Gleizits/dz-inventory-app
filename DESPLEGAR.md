# Desplegar DZ Urban Style

Esta guía describe cómo pasar la API y la base de datos a producción. La
arquitectura prevista es:

```text
APK Android ──HTTPS──► API Go (host de producción)
                       ├── PostgreSQL de Supabase
                       └── almacenamiento persistente de fotos
```

El host del binario Go todavía debe definirse. Puede ser una VM, servidor
dedicado o servicio equivalente sin costo adicional dentro de las
restricciones del proyecto.

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

Para aplicar Flyway directamente se debe usar la misma URL, usuario y
contraseña en el contenedor Flyway. No ejecutes `reset` contra producción:
esa operación elimina el volumen local y está destinada únicamente a
desarrollo.

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

## 3. Configurar la API

En el host de producción, define las variables sin subirlas al repositorio:

```text
DZ_API_ADDR=:8080
DZ_DATABASE_URL=postgres://USUARIO:CONTRASENA@HOST:5432/BASE?sslmode=require
DZ_REDIS_URL=redis://HOST_REDIS:6379/0
DZ_FOTOS_DIR=/var/lib/dz-inventory/fotos
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
- Configura reinicio automático del proceso mediante el servicio del host.

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

## 7. Verificación antes de publicar

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

## 8. Operación y mantenimiento

- Conserva copias de seguridad de PostgreSQL según la política de Supabase.
- Conserva el volumen de fotos y respáldalo junto con la base.
- Revisa los logs del proceso Go y del proxy inverso.
- Aplica nuevas migraciones Flyway antes de iniciar una versión que dependa
  de ellas.
- No ejecutes migraciones destructivas sobre producción sin una copia de
  seguridad y una revisión previa.

