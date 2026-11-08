package httpapi

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/Gleizits/dz-inventory-app/api/internal/fotos"
	"github.com/Gleizits/dz-inventory-app/api/internal/store"
)

// ---------------------------------------------------------------------------
// Categorías
// ---------------------------------------------------------------------------

type categoriaBody struct {
	Nombre      string  `json:"nombre"`
	Descripcion *string `json:"descripcion"`
}

func (s *Server) listarCategorias(w http.ResponseWriter, r *http.Request) {
	incluir := r.URL.Query().Get("incluir_inactivas") == "true"
	clave := "categorias:" + strconv.FormatBool(incluir)

	var out []store.Categoria
	if s.cache.Obtener(r.Context(), s.cache.Gen(r.Context()), clave, &out) {
		responderJSON(w, http.StatusOK, out)
		return
	}
	out, err := s.store.ListarCategorias(r.Context(), incluir)
	if err != nil {
		s.traducirError(w, err)
		return
	}
	s.cache.Guardar(r.Context(), s.cache.Gen(r.Context()), clave, out)
	responderJSON(w, http.StatusOK, out)
}

func (s *Server) crearCategoria(w http.ResponseWriter, r *http.Request) {
	var b categoriaBody
	if !leerJSON(w, r, &b) {
		return
	}
	c, err := s.store.CrearCategoria(r.Context(), b.Nombre, b.Descripcion)
	if err != nil {
		s.traducirError(w, err)
		return
	}
	s.cache.Invalidar(r.Context())
	responderJSON(w, http.StatusCreated, c)
}

func (s *Server) actualizarCategoria(w http.ResponseWriter, r *http.Request) {
	id, ok := s.idParam(w, r)
	if !ok {
		return
	}
	var b categoriaBody
	if !leerJSON(w, r, &b) {
		return
	}
	c, err := s.store.ActualizarCategoria(r.Context(), id, b.Nombre, b.Descripcion)
	if err != nil {
		s.traducirError(w, err)
		return
	}
	s.cache.Invalidar(r.Context())
	responderJSON(w, http.StatusOK, c)
}

func (s *Server) desactivarCategoria(w http.ResponseWriter, r *http.Request) {
	id, ok := s.idParam(w, r)
	if !ok {
		return
	}
	if err := s.store.DesactivarCategoria(r.Context(), id); err != nil {
		s.traducirError(w, err)
		return
	}
	s.cache.Invalidar(r.Context())
	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// Productos
// ---------------------------------------------------------------------------

type productoBody struct {
	CategoriaID int64           `json:"categoria_id"`
	Nombre      string          `json:"nombre"`
	Descripcion *string         `json:"descripcion"`
	Stock       map[string]int  `json:"stock"`
	Lote        *store.LoteInput `json:"lote"`
}

type productoResp struct {
	Producto store.Producto `json:"producto"`
	Lotes    []store.Lote   `json:"lotes"`
}

func (s *Server) listarProductos(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := store.FiltroProductos{
		Buscar:           q.Get("q"),
		IncluirInactivos: q.Get("incluir_inactivos") == "true",
		SoloConStock:     q.Get("con_stock") == "true",
	}
	if v := q.Get("categoria_id"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			responderError(w, http.StatusBadRequest, "validacion", "categoria_id inválido", nil)
			return
		}
		f.CategoriaID = &id
	}

	clave := "productos:" + strconv.FormatBool(f.IncluirInactivos) + ":" +
		strconv.FormatBool(f.SoloConStock) + ":" + f.Buscar + ":" +
		strconv.FormatInt(idClave(f.CategoriaID), 10)

	var out []store.Producto
	if s.cache.Obtener(r.Context(), s.cache.Gen(r.Context()), clave, &out) {
		responderJSON(w, http.StatusOK, out)
		return
	}
	out, err := s.store.ListarProductos(r.Context(), f)
	if err != nil {
		s.traducirError(w, err)
		return
	}
	s.cache.Guardar(r.Context(), s.cache.Gen(r.Context()), clave, out)
	responderJSON(w, http.StatusOK, out)
}

func (s *Server) obtenerProducto(w http.ResponseWriter, r *http.Request) {
	id, ok := s.idParam(w, r)
	if !ok {
		return
	}
	p, lotes, err := s.store.ObtenerProducto(r.Context(), id)
	if err != nil {
		s.traducirError(w, err)
		return
	}
	responderJSON(w, http.StatusOK, productoResp{Producto: p, Lotes: lotes})
}

func (s *Server) crearProducto(w http.ResponseWriter, r *http.Request) {
	var b productoBody
	if !leerJSON(w, r, &b) {
		return
	}
	p, lotes, err := s.store.CrearProducto(r.Context(), store.ProductoInput{
		CategoriaID: b.CategoriaID,
		Nombre:      b.Nombre,
		Descripcion: b.Descripcion,
		Stock:       b.Stock,
		Lote:        b.Lote,
	})
	if err != nil {
		s.traducirError(w, err)
		return
	}
	s.cache.Invalidar(r.Context())
	responderJSON(w, http.StatusCreated, productoResp{Producto: p, Lotes: lotes})
}

func (s *Server) actualizarProducto(w http.ResponseWriter, r *http.Request) {
	id, ok := s.idParam(w, r)
	if !ok {
		return
	}
	var b productoBody
	if !leerJSON(w, r, &b) {
		return
	}
	p, err := s.store.ActualizarProducto(r.Context(), id, store.ProductoInput{
		CategoriaID: b.CategoriaID,
		Nombre:      b.Nombre,
		Descripcion: b.Descripcion,
	})
	if err != nil {
		s.traducirError(w, err)
		return
	}
	s.cache.Invalidar(r.Context())
	responderJSON(w, http.StatusOK, p)
}

func (s *Server) desactivarProducto(w http.ResponseWriter, r *http.Request) {
	id, ok := s.idParam(w, r)
	if !ok {
		return
	}
	if err := s.store.DesactivarProducto(r.Context(), id); err != nil {
		s.traducirError(w, err)
		return
	}
	s.cache.Invalidar(r.Context())
	w.WriteHeader(http.StatusNoContent)
}

// subirFoto recibe la imagen (campo "foto"), la guarda con miniatura y
// actualiza la prenda.
func (s *Server) subirFoto(w http.ResponseWriter, r *http.Request) {
	id, ok := s.idParam(w, r)
	if !ok {
		return
	}
	// la app ya debe haber creado la prenda antes de subir la foto
	if _, _, err := s.store.ObtenerProducto(r.Context(), id); err != nil {
		s.traducirError(w, err)
		return
	}

	if err := r.ParseMultipartForm(12 << 20); err != nil {
		responderError(w, http.StatusBadRequest, "form_invalido",
			"no se pudo leer la imagen enviada (máximo 12 MB)", nil)
		return
	}
	archivo, _, err := r.FormFile("foto")
	if err != nil {
		responderError(w, http.StatusBadRequest, "form_invalido",
			`falta el campo "foto" con la imagen`, nil)
		return
	}
	defer archivo.Close()

	nombre, err := fotos.Guardar(s.cfg.FotosDir, archivo, s.cfg.AnchoMax, s.cfg.AnchoThumb)
	if err != nil {
		s.log.Error("guardar foto", "error", err)
		responderError(w, http.StatusBadRequest, "foto_invalida", err.Error(), nil)
		return
	}
	if err := s.store.GuardarFoto(r.Context(), id, nombre); err != nil {
		s.traducirError(w, err)
		return
	}
	s.cache.Invalidar(r.Context())

	responderJSON(w, http.StatusOK, map[string]string{
		"foto_path":      nombre,
		"foto_url":       "/api/fotos/" + nombre,
		"foto_thumb_url": "/api/fotos/" + fotos.Miniatura(nombre),
	})
}

// ---------------------------------------------------------------------------
// Auxiliares
// ---------------------------------------------------------------------------

func (s *Server) idParam(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		responderError(w, http.StatusBadRequest, "validacion", "id inválido", nil)
		return 0, false
	}
	return id, true
}

func idClave(v *int64) int64 {
	if v == nil {
		return 0
	}
	return *v
}
