// Package fotos guarda las fotos de las prendas: recorta el tamaño y genera
// una miniatura para que la app cargue liviana.
package fotos

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	_ "image/gif"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

const (
	calidadJPEG = 82
)

// Guardar recibe la imagen subida por la app, la redimensiona a anchoMax,
// genera la miniatura de anchoThumb y escribe ambas en dir.
// Devuelve el nombre del archivo principal (ej. "20261108-a1b2c3.jpg").
func Guardar(dir string, r io.Reader, anchoMax, anchoThumb int) (string, error) {
	img, _, err := image.Decode(r)
	if err != nil {
		return "", fmt.Errorf("no se pudo leer la imagen (formato no soportado): %w", err)
	}
	if anchoMax <= 0 {
		anchoMax = 1600
	}
	if anchoThumb <= 0 {
		anchoThumb = 320
	}

	base := nombreArchivo()
	principal := redimensionar(img, anchoMax)
	miniatura := redimensionar(img, anchoThumb)

	if err := escribirJPEG(filepath.Join(dir, base), principal); err != nil {
		return "", err
	}
	if err := escribirJPEG(filepath.Join(dir, Miniatura(base)), miniatura); err != nil {
		return "", err
	}
	return base, nil
}

// Miniatura devuelve el nombre de la miniatura a partir del nombre principal.
func Miniatura(nombre string) string {
	ext := filepath.Ext(nombre)
	return strings.TrimSuffix(nombre, ext) + "_thumb" + ext
}

// RutaSegura valida que el nombre no permita salirse del directorio de fotos
// y devuelve la ruta completa.
func RutaSegura(dir, nombre string) (string, error) {
	if nombre == "" || strings.ContainsAny(nombre, `/\`) || strings.Contains(nombre, "..") {
		return "", fmt.Errorf("nombre de archivo inválido")
	}
	ruta := filepath.Join(dir, nombre)
	if filepath.Dir(ruta) != filepath.Clean(dir) {
		return "", fmt.Errorf("nombre de archivo inválido")
	}
	if _, err := os.Stat(ruta); err != nil {
		return "", os.ErrNotExist
	}
	return ruta, nil
}

// redimensionar escala la imagen manteniendo la proporción; si ya es más
// chica no la toca.
func redimensionar(img image.Image, anchoMax int) image.Image {
	b := img.Bounds()
	if b.Dx() <= anchoMax {
		return img
	}
	alto := b.Dy() * anchoMax / b.Dx()
	if alto < 1 {
		alto = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, anchoMax, alto))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), img, b, xdraw.Over, nil)
	return dst
}

func escribirJPEG(ruta string, img image.Image) error {
	f, err := os.Create(ruta)
	if err != nil {
		return fmt.Errorf("no se pudo escribir la foto: %w", err)
	}
	defer f.Close()
	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: calidadJPEG}); err != nil {
		return fmt.Errorf("no se pudo convertir a JPEG: %w", err)
	}
	return nil
}

// nombreArchivo = fecha + aleatorio, para que no haya colisiones ni se adivine.
func nombreArchivo() string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("%s-%s.jpg", time.Now().Format("20060102-150405"), hex.EncodeToString(b[:]))
}

// NuevaImagenPrueba crea una imagen sólida para los tests.
func NuevaImagenPrueba(ancho, alto int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, ancho, alto))
	for y := 0; y < alto; y++ {
		for x := 0; x < ancho; x++ {
			img.Set(x, y, color.RGBA{R: 244, G: 98, B: 61, A: 255})
		}
	}
	return img
}
