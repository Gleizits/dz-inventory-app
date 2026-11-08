package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/Gleizits/dz-inventory-app/api/internal/store"
)

// reporteVentas devuelve unidades vendidas, costo de compra, venta, ganancia
// y porcentaje para un rango de fechas (es lo que la app exporta a PDF).
func (s *Server) reporteVentas(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	desde, hasta := q.Get("desde"), q.Get("hasta")

	clave := "reporte:" + desde + ":" + hasta
	var rep store.Reporte
	if s.cache.Obtener(r.Context(), s.cache.Gen(r.Context()), clave, &rep) {
		responderJSON(w, http.StatusOK, rep)
		return
	}

	resultado, err := s.store.ReporteVentas(r.Context(), desde, hasta)
	if err != nil {
		s.traducirError(w, err)
		return
	}
	s.cache.Guardar(r.Context(), s.cache.Gen(r.Context()), clave, resultado)
	responderJSON(w, http.StatusOK, resultado)
}

// salud es el health check: que la API esté viva y responda la base y Redis.
func (s *Server) salud(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	resp := map[string]any{
		"estado": "ok",
		"hora":   time.Now().Format(time.RFC3339),
	}

	if err := s.store.Ping(ctx); err != nil {
		resp["estado"] = "degradado"
		resp["postgres"] = "no responde"
	} else {
		resp["postgres"] = "ok"
	}
	resp["redis"] = "ok"
	if !s.cache.Disponible() {
		resp["redis"] = "no disponible (la API sigue funcionando sin caché)"
	}
	resp["cache_generacion"] = s.cache.Gen(ctx)

	status := http.StatusOK
	if resp["estado"] != "ok" {
		status = http.StatusServiceUnavailable
	}
	responderJSON(w, status, resp)
}
