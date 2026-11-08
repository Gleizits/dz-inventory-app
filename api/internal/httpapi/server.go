// Package httpapi expone la API REST que consume la app Android.
package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"path/filepath"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/Gleizits/dz-inventory-app/api/internal/cache"
	"github.com/Gleizits/dz-inventory-app/api/internal/config"
	"github.com/Gleizits/dz-inventory-app/api/internal/fotos"
	"github.com/Gleizits/dz-inventory-app/api/internal/store"
)

// Server agrupa las dependencias de todos los handlers.
type Server struct {
	store *store.Store
	cache *cache.Cache
	cfg   config.Config
	log   *slog.Logger
}

// New arma el router con todas las rutas.
func New(st *store.Store, ca *cache.Cache, cfg config.Config, log *slog.Logger) http.Handler {
	s := &Server{store: st, cache: ca, cfg: cfg, log: log}

	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(s.logRequests)
	r.Use(middleware.Recoverer)

	// Las fotos se sirven sin API key para que la app (y el preview del PDF)
	// pueda cargarlas sin cabeceras especiales.
	r.Get("/api/fotos/{archivo}", s.servirFoto)

	r.Route("/api", func(r chi.Router) {
		r.Use(s.apiKey)

		r.Get("/salud", s.salud)

		r.Get("/categorias", s.listarCategorias)
		r.Post("/categorias", s.crearCategoria)
		r.Put("/categorias/{id}", s.actualizarCategoria)
		r.Delete("/categorias/{id}", s.desactivarCategoria)

		r.Get("/productos", s.listarProductos)
		r.Post("/productos", s.crearProducto)
		r.Get("/productos/{id}", s.obtenerProducto)
		r.Put("/productos/{id}", s.actualizarProducto)
		r.Delete("/productos/{id}", s.desactivarProducto)
		r.Post("/productos/{id}/foto", s.subirFoto)

		r.Post("/lotes", s.crearLote)
		r.Get("/lotes", s.listarLotes)

		r.Get("/ventas", s.listarVentas)
		r.Post("/ventas", s.crearVenta)
		r.Post("/ventas/{id}/anular", s.anularVenta)

		r.Get("/reportes/ventas", s.reporteVentas)
		r.Get("/stock/bajos", s.stockBajos)
	})

	return r
}

// ---------------------------------------------------------------------------
// Respuestas
// ---------------------------------------------------------------------------

type errorResp struct {
	Error    string `json:"error"`
	Codigo   string `json:"codigo"`
	Detalles any    `json:"detalles,omitempty"`
}

func responderJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func responderError(w http.ResponseWriter, status int, codigo, mensaje string, detalles any) {
	responderJSON(w, status, errorResp{Error: mensaje, Codigo: codigo, Detalles: detalles})
}

// traducirError convierte los errores de dominio en respuestas HTTP.
func (s *Server) traducirError(w http.ResponseWriter, err error) {
	var ev *store.ErrorValidacion
	var es *store.ErrorStock

	switch {
	case errors.As(err, &ev):
		responderError(w, http.StatusBadRequest, "validacion", ev.Error(), ev)
	case errors.As(err, &es):
		responderError(w, http.StatusConflict, "stock_insuficiente", es.Error(), es)
	case errors.Is(err, store.ErrNoEncontrado):
		responderError(w, http.StatusNotFound, "no_encontrado", err.Error(), nil)
	case errors.Is(err, store.ErrConflicto):
		responderError(w, http.StatusConflict, "conflicto", err.Error(), nil)
	default:
		s.log.Error("error interno", "error", err)
		responderError(w, http.StatusInternalServerError, "interno",
			"error interno al procesar la solicitud", nil)
	}
}

// leerJSON decodifica el cuerpo de la petición y rechaza campos desconocidos
// (ayuda a detectar errores de nombre entre la app y la API).
func leerJSON(w http.ResponseWriter, r *http.Request, destino any) bool {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(destino); err != nil {
		responderError(w, http.StatusBadRequest, "json_invalido",
			"cuerpo de la petición inválido: "+err.Error(), nil)
		return false
	}
	return true
}

// ---------------------------------------------------------------------------
// Middlewares
// ---------------------------------------------------------------------------

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inicio := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		s.log.Info("http",
			"metodo", r.Method,
			"ruta", r.URL.Path,
			"estado", sw.status,
			"ms", time.Since(inicio).Milliseconds())
	})
}

// apiKey exige la cabecera X-API-Key solo si está configurada (en desarrollo
// local no hay clave y todo pasa).
func (s *Server) apiKey(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.APIKey != "" && r.Header.Get("X-API-Key") != s.cfg.APIKey {
			responderError(w, http.StatusUnauthorized, "api_key",
				"API key inválida o faltante", nil)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// servirFoto entrega la foto o su miniatura validando que el nombre no se
// salga del directorio.
func (s *Server) servirFoto(w http.ResponseWriter, r *http.Request) {
	nombre := chi.URLParam(r, "archivo")
	ruta, err := fotos.RutaSegura(s.cfg.FotosDir, nombre)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	// cachea en el dispositivo: el nombre ya lleva fecha+aleatorio
	w.Header().Set("Cache-Control", "public, max-age=604800")
	http.ServeFile(w, r, filepath.Clean(ruta))
}
