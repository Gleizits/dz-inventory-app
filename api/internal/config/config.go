// Package config carga la configuración de la API desde variables de entorno
// y, si existe, desde infra/.env (que nunca se sube a git).
package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Root        string        // raíz del repositorio
	Addr        string        // ej. ":8080"
	DBURL       string        // postgres://...
	RedisURL    string        // redis://...
	FotosDir    string        // donde se guardan las fotos
	SupabaseURL string        // https://<proyecto>.supabase.co
	SupabaseKey string        // service_role, solo en el servidor
	FotosBucket string        // bucket de Supabase Storage
	AnchoMax    int           // ancho máximo de la foto completa
	AnchoThumb  int           // ancho de la miniatura
	APIKey      string        // vacío = sin autenticación (dev local)
	CacheTTL    time.Duration // TTL del caché en Redis
	StockUmbral int           // stock mínimo para avisos
}

// Load lee infra/.env (si existe) y luego las variables de entorno, que tienen
// prioridad sobre el archivo.
func Load() (Config, error) {
	root := findRoot()
	if err := cargarEnv(filepath.Join(root, "infra", ".env")); err != nil {
		return Config{}, err
	}

	c := Config{
		Root:        root,
		Addr:        env("DZ_API_ADDR", ":8080"),
		RedisURL:    env("DZ_REDIS_URL", "redis://127.0.0.1:6379/0"),
		FotosDir:    env("DZ_FOTOS_DIR", filepath.Join(root, "storage", "fotos")),
		SupabaseURL: env("DZ_SUPABASE_URL", ""),
		SupabaseKey: env("DZ_SUPABASE_SERVICE_ROLE_KEY", ""),
		FotosBucket: env("DZ_SUPABASE_FOTOS_BUCKET", "fotos"),
		APIKey:      env("DZ_API_KEY", ""),
		StockUmbral: envInt("DZ_STOCK_UMBRAL", 0),
	}
	c.CacheTTL = envDur("DZ_CACHE_TTL", 60*time.Second)
	c.AnchoMax = envInt("DZ_FOTO_ANCHO_MAX", 1600)
	c.AnchoThumb = envInt("DZ_FOTO_ANCHO_THUMB", 320)

	c.DBURL = os.Getenv("DZ_DATABASE_URL")
	if c.DBURL == "" {
		c.DBURL = urlPostgres()
	}
	if c.DBURL == "" {
		return Config{}, fmt.Errorf("falta DZ_DATABASE_URL (o POSTGRES_USER/POSTGRES_PASSWORD/POSTGRES_DB en infra/.env)")
	}
	if err := os.MkdirAll(c.FotosDir, 0o755); err != nil {
		return Config{}, fmt.Errorf("no se pudo crear el directorio de fotos %s: %w", c.FotosDir, err)
	}
	return c, nil
}

// urlPostgres arma la URL de conexión a partir de las claves POSTGRES_* de .env.
func urlPostgres() string {
	user := os.Getenv("POSTGRES_USER")
	pass := os.Getenv("POSTGRES_PASSWORD")
	db := os.Getenv("POSTGRES_DB")
	if user == "" || pass == "" || db == "" {
		return ""
	}
	puerto := os.Getenv("POSTGRES_PORT")
	if puerto == "" {
		puerto = "5432"
	}
	return fmt.Sprintf("postgres://%s:%s@127.0.0.1:%s/%s", user, pass, puerto, db)
}

// findRoot sube desde el directorio de trabajo hasta encontrar la raíz del
// proyecto (donde está infra/.settings.gradle.kts).
func findRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "settings.gradle.kts")); err == nil {
			return dir
		}
		padre := filepath.Dir(dir)
		if padre == dir {
			break
		}
		dir = padre
	}
	dir, _ = os.Getwd()
	return dir
}

// cargarEnv parsea un archivo .env simple: LINEA=VALOR, # comentarios.
// No pisa las variables ya definidas en el entorno.
func cargarEnv(ruta string) error {
	f, err := os.Open(ruta)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		linea := strings.TrimSpace(sc.Text())
		if linea == "" || strings.HasPrefix(linea, "#") {
			continue
		}
		i := strings.Index(linea, "=")
		if i <= 0 {
			continue
		}
		clave := strings.TrimSpace(linea[:i])
		valor := strings.Trim(strings.TrimSpace(linea[i+1:]), `"'`)
		if os.Getenv(clave) == "" {
			os.Setenv(clave, valor)
		}
	}
	return sc.Err()
}

func env(clave, def string) string {
	if v := os.Getenv(clave); v != "" {
		return v
	}
	return def
}

func envInt(clave string, def int) int {
	if v := os.Getenv(clave); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envDur(clave string, def time.Duration) time.Duration {
	if v := os.Getenv(clave); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}
