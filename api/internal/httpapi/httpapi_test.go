package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/Gleizits/dz-inventory-app/api/internal/cache"
	"github.com/Gleizits/dz-inventory-app/api/internal/config"
	"github.com/Gleizits/dz-inventory-app/api/internal/store"
)

func nuevoServer(t *testing.T, apiKey, dirFotos string) *Server {
	t.Helper()
	return &Server{
		cfg: config.Config{APIKey: apiKey, FotosDir: dirFotos, StockUmbral: 0},
		log: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
		cache: &cache.Cache{},
	}
}

func TestAPIKey_SoloSiEstaConfigurada(t *testing.T) {
	// sin API key: todo pasa (desarrollo local)
	sinClave := nuevoServer(t, "", t.TempDir())
	h := sinClave.apiKey(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/categorias", nil))
	if rec.Code != http.StatusTeapot {
		t.Errorf("sin API key el status fue %d, quiero %d", rec.Code, http.StatusTeapot)
	}

	// con API key: exige la cabecera
	conClave := nuevoServer(t, "secreta", t.TempDir())
	h2 := conClave.apiKey(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))

	rec = httptest.NewRecorder()
	h2.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/categorias", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("sin cabecera el status fue %d, quiero 401", rec.Code)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/categorias", nil)
	req.Header.Set("X-API-Key", "secreta")
	rec = httptest.NewRecorder()
	h2.ServeHTTP(rec, req)
	if rec.Code != http.StatusTeapot {
		t.Errorf("con la clave correcta el status fue %d, quiero %d", rec.Code, http.StatusTeapot)
	}
}

func TestLeerJSON_RechazaDesconocidos(t *testing.T) {
	// campo que la app no debería mandar
	rec := httptest.NewRecorder()
	ok := leerJSON(rec, httptest.NewRequest(http.MethodPost, "/api/categorias",
		strings.NewReader(`{"nombre":"Polos","campo_raro":1}`)), &struct {
			Nombre string `json:"nombre"`
		}{})
	if ok {
		t.Error("debió rechazar un campo desconocido")
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, quiero 400", rec.Code)
	}

	// cuerpo válido
	rec = httptest.NewRecorder()
	var destino struct {
		Nombre string `json:"nombre"`
	}
	ok = leerJSON(rec, httptest.NewRequest(http.MethodPost, "/api/categorias",
		strings.NewReader(`{"nombre":"Polos oversize"}`)), &destino)
	if !ok {
		t.Fatalf("debió aceptar un JSON válido: %s", rec.Body.String())
	}
	if destino.Nombre != "Polos oversize" {
		t.Errorf("nombre = %q", destino.Nombre)
	}
}

func TestTraducirError_CodigosHTTP(t *testing.T) {
	s := nuevoServer(t, "", t.TempDir())
	casos := []struct {
		nombre string
		err    error
		status int
	}{
		{"validación", &store.ErrorValidacion{Campo: "nombre", Mensaje: "obligatorio"}, http.StatusBadRequest},
		{"stock", &store.ErrorStock{Producto: "Polo", Talla: "M", Disponible: 2, Solicitado: 5}, http.StatusConflict},
		{"no encontrado", store.ErrNoEncontrado, http.StatusNotFound},
		{"conflicto", store.ErrConflicto, http.StatusConflict},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			rec := httptest.NewRecorder()
			s.traducirError(rec, c.err)
			if rec.Code != c.status {
				t.Errorf("status = %d, quiero %d", rec.Code, c.status)
			}
			var resp errorResp
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil || resp.Codigo == "" {
				t.Errorf("la respuesta debe traer codigo: %s", rec.Body.String())
			}
		})
	}
}

func TestTraducirError_InternoNoFugaDetalle(t *testing.T) {
	s := nuevoServer(t, "", t.TempDir())
	rec := httptest.NewRecorder()
	s.traducirError(rec, os.ErrDeadlineExceeded)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, quiero 500", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "deadline") {
		t.Errorf("no debe filtrar el detalle del error interno: %s", rec.Body.String())
	}
}

func TestServirFoto_BloqueaTraversals(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/polo.jpg", []byte("imagen"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := nuevoServer(t, "", dir)

	// se monta sobre un router real: chi.URLParam necesita el contexto
	r := chi.NewRouter()
	r.Get("/api/fotos/{archivo}", s.servirFoto)

	// nombre normal
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/fotos/polo.jpg", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("archivo normal = %d, quiero 200", rec.Code)
	}

	// intento de salirse del directorio (chi recibe ".." como segmento)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/fotos/..", nil))
	if rec.Code == http.StatusOK {
		t.Error("debió rechazar la salida del directorio")
	}

	// nombre con ruta embebida
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/fotos/sub%2Fpolo.jpg", nil))
	if rec.Code == http.StatusOK {
		t.Error("debió rechazar un nombre con separador de ruta")
	}
}
