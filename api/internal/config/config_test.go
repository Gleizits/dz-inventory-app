package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCargarEnv(t *testing.T) {
	dir := t.TempDir()
	ruta := filepath.Join(dir, ".env")
	contenido := `# comentario
POSTGRES_USER=dz

POSTGRES_PASSWORD="clave con espacios"
DZ_CACHE_TTL=90s
clave_sin_igual
=valor_sin_clave
`
	if err := os.WriteFile(ruta, []byte(contenido), 0o600); err != nil {
		t.Fatal(err)
	}

	os.Unsetenv("POSTGRES_USER")
	os.Unsetenv("POSTGRES_PASSWORD")
	os.Unsetenv("DZ_CACHE_TTL")
	// una variable ya existente NO debe pisarse
	os.Setenv("POSTGRES_USER", "del_entorno")
	t.Cleanup(func() {
		for _, k := range []string{"POSTGRES_USER", "POSTGRES_PASSWORD", "DZ_CACHE_TTL"} {
			os.Unsetenv(k)
		}
	})

	if err := cargarEnv(ruta); err != nil {
		t.Fatalf("cargarEnv: %v", err)
	}
	if os.Getenv("POSTGRES_USER") != "del_entorno" {
		t.Errorf("POSTGRES_USER = %q, no debe pisar el entorno", os.Getenv("POSTGRES_USER"))
	}
	if os.Getenv("POSTGRES_PASSWORD") != "clave con espacios" {
		t.Errorf("POSTGRES_PASSWORD = %q, debe quitar las comillas", os.Getenv("POSTGRES_PASSWORD"))
	}
	if os.Getenv("DZ_CACHE_TTL") != "90s" {
		t.Errorf("DZ_CACHE_TTL = %q", os.Getenv("DZ_CACHE_TTL"))
	}
}

func TestCargarEnv_ArchivoInexistente_NoFalla(t *testing.T) {
	if err := cargarEnv(filepath.Join(t.TempDir(), "no-existe.env")); err != nil {
		t.Errorf("un .env que no existe no debe ser error: %v", err)
	}
}

func TestDefaults(t *testing.T) {
	os.Unsetenv("DZ_API_ADDR")
	if got := env("DZ_API_ADDR", ":8080"); got != ":8080" {
		t.Errorf("env default = %q", got)
	}
	if got := envInt("DZ_FOTO_ANCHO_MAX", 1600); got != 1600 {
		t.Errorf("envInt default = %d", got)
	}
	if got := envDur("DZ_CACHE_TTL", time.Minute); got != time.Minute {
		t.Errorf("envDur default = %v", got)
	}
	if got := envInt("DZ_FOTO_ANCHO_MAX", 1600); got != 1600 {
		t.Errorf("envInt con variable ausente = %d", got)
	}
}

func TestURLPostgres_FaltaClave(t *testing.T) {
	for _, k := range []string{"POSTGRES_USER", "POSTGRES_PASSWORD", "POSTGRES_DB", "DZ_DATABASE_URL"} {
		os.Unsetenv(k)
	}
	if url := urlPostgres(); url != "" {
		t.Errorf("sin claves debe devolver vacío, llegó %q", url)
	}
}
