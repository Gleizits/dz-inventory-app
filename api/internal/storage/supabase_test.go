package storage

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSupabase_SubirYDescargar(t *testing.T) {
	const contenido = "jpeg"
	var subido bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/storage/v1/object/fotos/20260101.jpg" {
			t.Errorf("ruta = %q", r.URL.Path)
		}
		if r.Method == http.MethodPost {
			subido = true
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(contenido))
			return
		}
		t.Errorf("método inesperado: %s", r.Method)
	}))
	defer srv.Close()

	s := NewSupabase(srv.URL, "secreta", "fotos")
	if err := s.Subir(context.Background(), "20260101.jpg", []byte(contenido)); err != nil {
		t.Fatalf("Subir: %v", err)
	}
	got, err := s.Descargar(context.Background(), "20260101.jpg")
	if err != nil {
		t.Fatalf("Descargar: %v", err)
	}
	if string(got) != contenido {
		t.Errorf("contenido = %q, quiero %q", got, contenido)
	}
	if !subido {
		t.Error("no se recibió la subida")
	}
}

func TestNewSupabase_RequiereConfiguracionCompleta(t *testing.T) {
	if NewSupabase("", "clave", "fotos") != nil {
		t.Error("URL vacía debió desactivar Storage")
	}
	if NewSupabase("https://ejemplo.supabase.co", "", "fotos") != nil {
		t.Error("clave vacía debió desactivar Storage")
	}
	if NewSupabase("https://ejemplo.supabase.co", "clave", "") != nil {
		t.Error("bucket vacío debió desactivar Storage")
	}
}
