package fotos

import (
	"bytes"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGuardar_GeneraFotoYMiniatura(t *testing.T) {
	dir := t.TempDir()

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, NuevaImagenPrueba(4000, 2000), nil); err != nil {
		t.Fatal(err)
	}

	nombre, err := Guardar(dir, &buf, 1600, 320)
	if err != nil {
		t.Fatalf("Guardar: %v", err)
	}
	if !strings.HasSuffix(nombre, ".jpg") {
		t.Errorf("el nombre debe terminar en .jpg, llegó %q", nombre)
	}

	// foto principal
	ruta := filepath.Join(dir, nombre)
	abrirYValidar(t, ruta, 1600, 800)

	// miniatura
	rutaThumb := filepath.Join(dir, Miniatura(nombre))
	if _, err := os.Stat(rutaThumb); err != nil {
		t.Fatalf("falta la miniatura: %v", err)
	}
	abrirYValidar(t, rutaThumb, 320, 160)
}

func TestGuardar_ImagenChicaNoSeAgranda(t *testing.T) {
	dir := t.TempDir()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, NuevaImagenPrueba(200, 100), nil); err != nil {
		t.Fatal(err)
	}
	nombre, err := Guardar(dir, &buf, 1600, 320)
	if err != nil {
		t.Fatalf("Guardar: %v", err)
	}
	abrirYValidar(t, filepath.Join(dir, nombre), 200, 100)
}

func TestGuardar_ImagenInvalida(t *testing.T) {
	dir := t.TempDir()
	_, err := Guardar(dir, strings.NewReader("esto no es una imagen"), 1600, 320)
	if err == nil {
		t.Error("debió rechazar contenido que no es imagen")
	}
}

func TestRutaSegura_BloqueaTraversals(t *testing.T) {
	dir := t.TempDir()
	archivo := filepath.Join(dir, "ok.jpg")
	if err := os.WriteFile(archivo, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := RutaSegura(dir, "ok.jpg"); err != nil {
		t.Errorf("debió aceptar un archivo normal: %v", err)
	}
	for _, malo := range []string{"", "..\\..\\windows\\system32", "../../etc/passwd", "sub/ok.jpg"} {
		if _, err := RutaSegura(dir, malo); err == nil {
			t.Errorf("debió rechazar %q", malo)
		}
	}
	if _, err := RutaSegura(dir, "no-existe.jpg"); err == nil {
		t.Error("debió devolver error de archivo inexistente")
	}
}

func abrirYValidar(t *testing.T, ruta string, anchoEsperado, altoEsperado int) {
	t.Helper()
	f, err := os.Open(ruta)
	if err != nil {
		t.Fatalf("no abrió %s: %v", ruta, err)
	}
	defer f.Close()
	img, err := jpeg.Decode(f)
	if err != nil {
		t.Fatalf("no decodificó %s: %v", ruta, err)
	}
	if img.Bounds().Dx() != anchoEsperado || img.Bounds().Dy() != altoEsperado {
		t.Errorf("%s = %dx%d, quiero %dx%d",
			filepath.Base(ruta), img.Bounds().Dx(), img.Bounds().Dy(), anchoEsperado, altoEsperado)
	}
}
