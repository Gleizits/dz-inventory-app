package httpapi

import (
	"net/http"
	"strconv"

	"github.com/Gleizits/dz-inventory-app/api/internal/store"
)

// ---------------------------------------------------------------------------
// Lotes (compras de prendas)
// ---------------------------------------------------------------------------

type loteBody struct {
	CategoriaID int64          `json:"categoria_id"`
	ProductoID  int64          `json:"producto_id"`
	Fecha       string         `json:"fecha"`
	Unidades    int            `json:"unidades"`
	CostoTotal  float64        `json:"costo_total"`
	CostoEnvio  float64        `json:"costo_envio"`
	Stock       map[string]int `json:"stock"`
}

func (s *Server) crearLote(w http.ResponseWriter, r *http.Request) {
	var b loteBody
	if !leerJSON(w, r, &b) {
		return
	}
	if b.ProductoID <= 0 && b.CategoriaID <= 0 {
		responderError(w, http.StatusBadRequest, "validacion", "producto_id inválido", nil)
		return
	}
	if b.ProductoID <= 0 {
		l, err := s.store.CrearLoteCategoria(r.Context(), b.CategoriaID, store.LoteInput{
			Fecha: b.Fecha, Unidades: b.Unidades, CostoTotal: b.CostoTotal, CostoEnvio: b.CostoEnvio,
		})
		if err != nil {
			s.traducirError(w, err)
			return
		}
		s.cache.Invalidar(r.Context())
		responderJSON(w, http.StatusCreated, l)
		return
	}
	l, err := s.store.CrearLote(r.Context(), b.ProductoID, store.LoteInput{
		Fecha:      b.Fecha,
		Unidades:   b.Unidades,
		CostoTotal: b.CostoTotal,
		CostoEnvio: b.CostoEnvio,
		Stock:      b.Stock,
	})
	if err != nil {
		s.traducirError(w, err)
		return
	}
	s.cache.Invalidar(r.Context())
	responderJSON(w, http.StatusCreated, l)
}

func (s *Server) listarLotes(w http.ResponseWriter, r *http.Request) {
	var categoriaID *int64
	if value := r.URL.Query().Get("categoria_id"); value != "" {
		id, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			responderError(w, http.StatusBadRequest, "validacion", "categoria_id inválido", nil)
			return
		}
		categoriaID = &id
	}
	out, err := s.store.ListarLotesCategoria(r.Context(), categoriaID)
	if err != nil {
		s.traducirError(w, err)
		return
	}
	responderJSON(w, http.StatusOK, out)
}

// ---------------------------------------------------------------------------
// Ventas
// ---------------------------------------------------------------------------

func (s *Server) crearVenta(w http.ResponseWriter, r *http.Request) {
	var b store.VentaInput
	if !leerJSON(w, r, &b) {
		return
	}
	v, err := s.store.CrearVenta(r.Context(), b)
	if err != nil {
		s.traducirError(w, err)
		return
	}
	s.cache.Invalidar(r.Context())
	responderJSON(w, http.StatusCreated, v)
}

func (s *Server) listarVentas(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limite, _ := strconv.Atoi(q.Get("limite"))
	offset, _ := strconv.Atoi(q.Get("offset"))

	out, err := s.store.ListarVentas(r.Context(), q.Get("desde"), q.Get("hasta"), limite, offset)
	if err != nil {
		s.traducirError(w, err)
		return
	}
	responderJSON(w, http.StatusOK, out)
}

func (s *Server) anularVenta(w http.ResponseWriter, r *http.Request) {
	id, ok := s.idParam(w, r)
	if !ok {
		return
	}
	v, err := s.store.AnularVenta(r.Context(), id)
	if err != nil {
		s.traducirError(w, err)
		return
	}
	s.cache.Invalidar(r.Context())
	responderJSON(w, http.StatusOK, v)
}

// ---------------------------------------------------------------------------
// Stock
// ---------------------------------------------------------------------------

func (s *Server) stockBajos(w http.ResponseWriter, r *http.Request) {
	umbral := s.cfg.StockUmbral
	if v := r.URL.Query().Get("umbral"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			responderError(w, http.StatusBadRequest, "validacion", "umbral inválido", nil)
			return
		}
		umbral = n
	}

	clave := "stock:bajos:" + strconv.Itoa(umbral)
	var out []store.StockBajo
	if s.cache.Obtener(r.Context(), s.cache.Gen(r.Context()), clave, &out) {
		responderJSON(w, http.StatusOK, out)
		return
	}
	out, err := s.store.StockBajos(r.Context(), umbral)
	if err != nil {
		s.traducirError(w, err)
		return
	}
	s.cache.Guardar(r.Context(), s.cache.Gen(r.Context()), clave, out)
	responderJSON(w, http.StatusOK, out)
}
