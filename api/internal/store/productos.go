package store

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
)

// StockTalla es el stock actual de una prenda en una talla.
type StockTalla struct {
	Talla    string `json:"talla"`
	Cantidad int    `json:"cantidad"`
}

// Producto es una prenda del catálogo.
type Producto struct {
	ID            int64        `json:"id"`
	CategoriaID   int64        `json:"categoria_id"`
	Categoria     string       `json:"categoria"`
	LoteID        int64        `json:"lote_id"`
	UnidadesLote  int          `json:"unidades_lote"`
	Nombre        string       `json:"nombre"`
	Descripcion   *string      `json:"descripcion"`
	FotoPath      *string      `json:"foto_path"`
	Activo        bool         `json:"activo"`
	Unidades      int          `json:"unidades"`
	Stock         []StockTalla `json:"stock"`
	CreadoEn      string       `json:"creado_en"`
	ActualizadoEn string       `json:"actualizado_en"`
}

// Lote es una compra de prendas: de ahí sale el costo por unidad.
type Lote struct {
	ID            int64   `json:"id"`
	CategoriaID   int64   `json:"categoria_id"`
	Categoria     string  `json:"categoria"`
	ProductoID    int64   `json:"producto_id"`
	Fecha         string  `json:"fecha"`
	Unidades      int     `json:"unidades"`
	CostoTotal    float64 `json:"costo_total"`
	CostoEnvio    float64 `json:"costo_envio"`
	CostoUnitario float64 `json:"costo_unitario"`
	CreadoEn      string  `json:"creado_en"`
	Asignadas     int     `json:"unidades_asignadas"`
	Disponibles   int     `json:"unidades_disponibles"`
}

// FiltroProductos para el listado del catálogo.
type FiltroProductos struct {
	CategoriaID      *int64
	Buscar           string
	IncluirInactivos bool
	SoloConStock     bool
}

// ProductoInput es lo que la app manda al crear/actualizar una prenda.
type ProductoInput struct {
	CategoriaID  int64
	Nombre       string
	Descripcion  *string
	Stock        map[string]int // talla -> unidades del lote inicial
	Lote         *LoteInput     // costo del lote inicial (opcional)
	LoteID       *int64         // lote existente de la categoría
	UnidadesLote int            // unidades asignadas de un lote existente
}

// LoteInput es una compra de prendas.
type LoteInput struct {
	Fecha      string         `json:"fecha"`    // opcional, AAAA-MM-DD
	Unidades   int            `json:"unidades"` // requerido si no viene Stock
	CostoTotal float64        `json:"costo_total"`
	CostoEnvio float64        `json:"costo_envio"`
	Stock      map[string]int `json:"stock"` // opcional: reparto del lote por talla
}

const colsProducto = `p.id, p.categoria_id, c.nombre, COALESCE(p.lote_id, 0), COALESCE(p.unidades_lote, 0),
	p.nombre, p.descripcion, p.foto_path, p.activo,
	to_char(p.creado_en, 'YYYY-MM-DD HH24:MI:SS'), to_char(p.actualizado_en, 'YYYY-MM-DD HH24:MI:SS')`

// ListarProductos devuelve el catálogo con su stock por talla.
func (s *Store) ListarProductos(ctx context.Context, f FiltroProductos) ([]Producto, error) {
	q := `SELECT ` + colsProducto + `
	      FROM producto p JOIN categoria c ON c.id = p.categoria_id
	     WHERE ($1::bigint IS NULL OR p.categoria_id = $1)
	       AND ($2::text IS NULL OR p.nombre ILIKE '%' || $2 || '%'
	                              OR c.nombre ILIKE '%' || $2 || '%')
	       AND (p.activo OR $3::bool)
	     ORDER BY p.nombre`
	rows, err := s.pool.Query(ctx, q, f.CategoriaID, limpiar(f.Buscar), f.IncluirInactivos)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Producto{}
	ids := []int64{}
	for rows.Next() {
		p, err := escanearProducto(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
		ids = append(ids, p.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := s.cargarStock(ctx, out, ids); err != nil {
		return nil, err
	}

	if f.SoloConStock {
		conStock := out[:0]
		for _, p := range out {
			if p.Unidades > 0 {
				conStock = append(conStock, p)
			}
		}
		out = conStock
	}
	return out, nil
}

// ObtenerProducto devuelve una prenda con su stock y sus últimos lotes.
func (s *Store) ObtenerProducto(ctx context.Context, id int64) (Producto, []Lote, error) {
	var p Producto
	row := s.pool.QueryRow(ctx, `SELECT `+colsProducto+`
	      FROM producto p JOIN categoria c ON c.id = p.categoria_id
	     WHERE p.id = $1`, id)
	if err := escanearProductoRow(row, &p); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return p, nil, fmt.Errorf("producto %d: %w", id, ErrNoEncontrado)
		}
		return p, nil, err
	}
	lista := []Producto{p}
	if err := s.cargarStock(ctx, lista, []int64{id}); err != nil {
		return p, nil, err
	}
	p = lista[0]

	lotes, err := s.listarLotes(ctx, id, 5)
	if err != nil {
		return p, nil, err
	}
	return p, lotes, nil
}

// CrearProducto da de alta una prenda con su stock inicial y, si se envía,
// el lote con el costo total y el envío (de ahí sale el precio por unidad).
func (s *Store) CrearProducto(ctx context.Context, in ProductoInput) (Producto, []Lote, error) {
	in.Nombre = limpiar(in.Nombre)
	if in.Nombre == "" {
		return Producto{}, nil, &ErrorValidacion{Campo: "nombre", Mensaje: "el nombre de la prenda es obligatorio"}
	}
	if err := validarStockMap(in.Stock); err != nil {
		return Producto{}, nil, err
	}
	if in.LoteID != nil {
		if in.UnidadesLote <= 0 {
			in.UnidadesLote = sumaStock(in.Stock)
		}
		if in.UnidadesLote != sumaStock(in.Stock) {
			return Producto{}, nil, &ErrorValidacion{Campo: "unidades_lote",
				Mensaje: fmt.Sprintf("las unidades del producto (%d) no coinciden con la suma por talla (%d)",
					in.UnidadesLote, sumaStock(in.Stock))}
		}
	}

	var nuevoID int64
	err := s.transaccion(ctx, func(tx pgx.Tx) error {
		var activa bool
		if in.LoteID != nil {
			err := tx.QueryRow(ctx,
				`SELECT c.id, c.activo
				   FROM lote l JOIN categoria c ON c.id = l.categoria_id
				  WHERE l.id = $1`, *in.LoteID).
				Scan(&in.CategoriaID, &activa)
			if errors.Is(err, pgx.ErrNoRows) {
				return &ErrorValidacion{Campo: "lote_id", Mensaje: "el lote no existe"}
			}
			if err != nil {
				return err
			}
		} else {
			err := tx.QueryRow(ctx, `SELECT activo FROM categoria WHERE id = $1`, in.CategoriaID).Scan(&activa)
			if errors.Is(err, pgx.ErrNoRows) {
				return &ErrorValidacion{Campo: "categoria_id", Mensaje: "la categoría no existe"}
			}
			if err != nil {
				return err
			}
		}
		if !activa {
			return &ErrorValidacion{Campo: "categoria_id", Mensaje: "la categoría está desactivada"}
		}
		if in.LoteID != nil {
			var asignadas, disponibles int
			err := tx.QueryRow(ctx,
				`SELECT COALESCE(SUM(unidades_lote), 0)::int, l.unidades
				   FROM lote l
				   LEFT JOIN producto p ON p.lote_id = l.id
				  WHERE l.id = $1
				  GROUP BY l.unidades`, *in.LoteID).
				Scan(&asignadas, &disponibles)
			if err != nil {
				return err
			}
			if asignadas+in.UnidadesLote > disponibles {
				return &ErrorValidacion{Campo: "lote_id",
					Mensaje: fmt.Sprintf("el lote solo tiene %d unidades disponibles", disponibles-asignadas)}
			}
		}

		var id int64
		insertErr := tx.QueryRow(ctx,
			`INSERT INTO producto (categoria_id, lote_id, unidades_lote, nombre, descripcion)
			 VALUES ($1, $2, $3, $4, $5) RETURNING id`,
			in.CategoriaID, in.LoteID, nullablePositive(in.UnidadesLote), in.Nombre, in.Descripcion).Scan(&id)
		if esDuplicado(insertErr) {
			return fmt.Errorf("ya existe una prenda llamada %q en esa categoría: %w", in.Nombre, ErrConflicto)
		}
		if insertErr != nil {
			return insertErr
		}
		nuevoID = id

		if in.Lote != nil {
			// crearLoteEn es quien suma el stock: si también lo hiciera
			// sumarStock aquí, las unidades entrarían dos veces.
			if err := crearLoteEn(ctx, tx, id, *in.Lote, in.Stock); err != nil {
				return err
			}
		} else if err := sumarStock(ctx, tx, id, in.Stock); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return Producto{}, nil, err
	}
	return s.ObtenerProducto(ctx, nuevoID)
}

// ActualizarProducto cambia nombre, descripción y categoría.
func (s *Store) ActualizarProducto(ctx context.Context, id int64, in ProductoInput) (Producto, error) {
	in.Nombre = limpiar(in.Nombre)
	if in.Nombre == "" {
		return Producto{}, &ErrorValidacion{Campo: "nombre", Mensaje: "el nombre de la prenda es obligatorio"}
	}
	var p Producto
	err := s.pool.QueryRow(ctx,
		`UPDATE producto SET nombre = $2, descripcion = $3, categoria_id = $4
		  WHERE id = $1
		  RETURNING id, categoria_id, nombre, descripcion, foto_path, activo,
		            to_char(creado_en, 'YYYY-MM-DD HH24:MI:SS'),
		            to_char(actualizado_en, 'YYYY-MM-DD HH24:MI:SS')`,
		id, in.Nombre, in.Descripcion, in.CategoriaID).
		Scan(&p.ID, &p.CategoriaID, &p.Nombre, &p.Descripcion, &p.FotoPath, &p.Activo,
			&p.CreadoEn, &p.ActualizadoEn)
	if errors.Is(err, pgx.ErrNoRows) {
		return p, fmt.Errorf("producto %d: %w", id, ErrNoEncontrado)
	}
	if err != nil && esDuplicado(err) {
		return p, fmt.Errorf("ya existe una prenda con ese nombre en la categoría: %w", ErrConflicto)
	}
	if err != nil {
		return p, err
	}
	// refrescar categoría y stock
	lista := []Producto{p}
	_ = s.cargarStock(ctx, lista, []int64{id})
	actualizada, _, err := s.ObtenerProducto(ctx, id)
	return actualizada, err
}

// DesactivarProducto hace borrado lógico: el historial de ventas se conserva.
func (s *Store) DesactivarProducto(ctx context.Context, id int64) error {
	tag, err := s.pool.Exec(ctx, `UPDATE producto SET activo = false WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("producto %d: %w", id, ErrNoEncontrado)
	}
	return nil
}

// GuardarFoto actualiza la ruta de la foto de la prenda.
func (s *Store) GuardarFoto(ctx context.Context, id int64, nombreArchivo string) error {
	tag, err := s.pool.Exec(ctx, `UPDATE producto SET foto_path = $2 WHERE id = $1`, id, nombreArchivo)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("producto %d: %w", id, ErrNoEncontrado)
	}
	return nil
}

// CrearLote registra una compra y suma las unidades al stock si viene la
// distribución por talla.
func (s *Store) CrearLote(ctx context.Context, productoID int64, in LoteInput) (Lote, error) {
	var l Lote
	err := s.transaccion(ctx, func(tx pgx.Tx) error {
		var nombre string
		if err := tx.QueryRow(ctx, `SELECT nombre FROM producto WHERE id = $1`, productoID).Scan(&nombre); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("producto %d: %w", productoID, ErrNoEncontrado)
			}
			return err
		}
		if err := validarStockMap(in.Stock); err != nil {
			return err
		}
		if err := crearLoteEn(ctx, tx, productoID, in, in.Stock); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return l, err
	}
	// devolver el lote recién creado
	row := s.pool.QueryRow(ctx,
		`SELECT id, producto_id, to_char(fecha,'YYYY-MM-DD'), unidades,
		        costo_total::float8, costo_envio::float8,
		        ROUND((costo_total + costo_envio) / unidades, 2)::float8,
		        to_char(creado_en,'YYYY-MM-DD HH24:MI:SS')
		   FROM lote WHERE producto_id = $1 ORDER BY id DESC LIMIT 1`, productoID)
	err = row.Scan(&l.ID, &l.ProductoID, &l.Fecha, &l.Unidades, &l.CostoTotal,
		&l.CostoEnvio, &l.CostoUnitario, &l.CreadoEn)
	return l, err
}

// CrearLoteCategoria crea un lote vacío que luego puede recibir varias prendas.
func (s *Store) CrearLoteCategoria(ctx context.Context, categoriaID int64, in LoteInput) (Lote, error) {
	if in.Unidades <= 0 {
		return Lote{}, &ErrorValidacion{Campo: "unidades", Mensaje: "las unidades del lote deben ser mayores a cero"}
	}
	if in.CostoTotal < 0 || in.CostoEnvio < 0 {
		return Lote{}, &ErrorValidacion{Campo: "costo_total", Mensaje: "los costos no pueden ser negativos"}
	}
	fecha := time.Now()
	if in.Fecha != "" {
		f, err := time.ParseInLocation("2006-01-02", in.Fecha, time.Local)
		if err != nil {
			return Lote{}, &ErrorValidacion{Campo: "fecha", Mensaje: "fecha inválida"}
		}
		fecha = f
	}
	var l Lote
	err := s.pool.QueryRow(ctx,
		`INSERT INTO lote (categoria_id, fecha, unidades, costo_total, costo_envio)
		 SELECT id, $2, $3, $4, $5 FROM categoria
		  WHERE id = $1 AND activo
		 RETURNING id, categoria_id, 0, to_char(fecha,'YYYY-MM-DD'), unidades,
		           costo_total::float8, costo_envio::float8,
		           ROUND((costo_total + costo_envio) / unidades, 2)::float8,
		           to_char(creado_en,'YYYY-MM-DD HH24:MI:SS')`,
		categoriaID, fecha, in.Unidades, in.CostoTotal, in.CostoEnvio).
		Scan(&l.ID, &l.CategoriaID, &l.ProductoID, &l.Fecha, &l.Unidades,
			&l.CostoTotal, &l.CostoEnvio, &l.CostoUnitario, &l.CreadoEn)
	if errors.Is(err, pgx.ErrNoRows) {
		return Lote{}, &ErrorValidacion{Campo: "categoria_id", Mensaje: "la categoría no existe o está desactivada"}
	}
	l.Disponibles = l.Unidades
	return l, err
}

// ListarLotesCategoria devuelve lotes disponibles para agregar prendas.
func (s *Store) ListarLotesCategoria(ctx context.Context, categoriaID *int64) ([]Lote, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT l.id, l.categoria_id, c.nombre, COALESCE(l.producto_id, 0),
		        to_char(l.fecha,'YYYY-MM-DD'), l.unidades,
		        l.costo_total::float8, l.costo_envio::float8,
		        ROUND((l.costo_total + l.costo_envio) / l.unidades, 2)::float8,
		        to_char(l.creado_en,'YYYY-MM-DD HH24:MI:SS'),
		        COALESCE(SUM(p.unidades_lote), 0)::int,
		        l.unidades - COALESCE(SUM(p.unidades_lote), 0)::int
		   FROM lote l
		   JOIN categoria c ON c.id = l.categoria_id
		   LEFT JOIN producto p ON p.lote_id = l.id AND p.activo
		  WHERE ($1::bigint IS NULL OR l.categoria_id = $1)
		  GROUP BY l.id, c.nombre
		  ORDER BY l.fecha DESC, l.id DESC`, categoriaID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Lote{}
	for rows.Next() {
		var l Lote
		if err := rows.Scan(&l.ID, &l.CategoriaID, &l.Categoria, &l.ProductoID,
			&l.Fecha, &l.Unidades, &l.CostoTotal, &l.CostoEnvio,
			&l.CostoUnitario, &l.CreadoEn, &l.Asignadas, &l.Disponibles); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func sumaStock(stock map[string]int) int {
	total := 0
	for _, n := range stock {
		total += n
	}
	return total
}

func nullablePositive(n int) any {
	if n <= 0 {
		return nil
	}
	return n
}

// crearLoteEn inserta el lote dentro de una transacción existente y, si hay
// distribución por talla, suma esas unidades al stock.
func crearLoteEn(ctx context.Context, tx pgx.Tx, productoID int64, in LoteInput, stock map[string]int) error {
	if in.CostoTotal < 0 || in.CostoEnvio < 0 {
		return &ErrorValidacion{Campo: "costo_total", Mensaje: "los costos no pueden ser negativos"}
	}
	unidades := in.Unidades
	if len(stock) > 0 {
		total := 0
		for _, n := range stock {
			total += n
		}
		if unidades == 0 {
			unidades = total
		} else if unidades != total {
			return &ErrorValidacion{Campo: "unidades",
				Mensaje: fmt.Sprintf("las unidades (%d) no coinciden con la suma del stock por talla (%d)", unidades, total)}
		}
	}
	if unidades <= 0 {
		return &ErrorValidacion{Campo: "unidades", Mensaje: "las unidades del lote deben ser mayores a cero"}
	}

	fecha := time.Now()
	if in.Fecha != "" {
		f, err := time.ParseInLocation("2006-01-02", in.Fecha, time.Local)
		if err != nil {
			return &ErrorValidacion{Campo: "fecha", Mensaje: fmt.Sprintf("%q no es una fecha válida (use AAAA-MM-DD)", in.Fecha)}
		}
		fecha = f
	}

	var id int64
	err := tx.QueryRow(ctx,
		`INSERT INTO lote (producto_id, fecha, unidades, costo_total, costo_envio)
		 VALUES ($1, $2, $3, $4::float8, $5::float8) RETURNING id`,
		productoID, fecha, unidades, in.CostoTotal, in.CostoEnvio).Scan(&id)
	if err != nil {
		return err
	}
	return sumarStock(ctx, tx, productoID, stock)
}

// sumarStock incrementa (o crea) el stock por talla de una prenda.
func sumarStock(ctx context.Context, tx pgx.Tx, productoID int64, stock map[string]int) error {
	tallas := ordenarTallas(stock)
	for _, talla := range tallas {
		if !TallasValidas[talla] {
			return &ErrorValidacion{Campo: "stock",
				Mensaje: fmt.Sprintf("talla inválida %q: use S, M, L o XL", talla)}
		}
		if stock[talla] < 0 {
			return &ErrorValidacion{Campo: "stock", Mensaje: "las cantidades no pueden ser negativas"}
		}
		_, err := tx.Exec(ctx,
			`INSERT INTO producto_talla (producto_id, talla, cantidad) VALUES ($1, $2::talla_t, $3)
			 ON CONFLICT (producto_id, talla)
			 DO UPDATE SET cantidad = producto_talla.cantidad + EXCLUDED.cantidad`,
			productoID, talla, stock[talla])
		if err != nil {
			return err
		}
	}
	return nil
}

// listarLotes devuelve los lotes más recientes de una prenda.
func (s *Store) listarLotes(ctx context.Context, productoID int64, limite int) ([]Lote, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, categoria_id, COALESCE(producto_id, 0), to_char(fecha,'YYYY-MM-DD'), unidades,
		        costo_total::float8, costo_envio::float8,
		        ROUND((costo_total + costo_envio) / unidades, 2)::float8,
		        to_char(creado_en,'YYYY-MM-DD HH24:MI:SS')
		   FROM lote WHERE producto_id = $1 ORDER BY fecha DESC, id DESC LIMIT $2`,
		productoID, limite)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Lote{}
	for rows.Next() {
		var l Lote
		if err := rows.Scan(&l.ID, &l.CategoriaID, &l.ProductoID, &l.Fecha, &l.Unidades, &l.CostoTotal,
			&l.CostoEnvio, &l.CostoUnitario, &l.CreadoEn); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// cargarStock rellena el stock de una lista de productos ya consultados.
func (s *Store) cargarStock(ctx context.Context, ps []Producto, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	rows, err := s.pool.Query(ctx,
		`SELECT producto_id, talla::text, cantidad FROM producto_talla
		  WHERE producto_id = ANY($1) ORDER BY talla`, ids)
	if err != nil {
		return err
	}
	defer rows.Close()

	porID := map[int64]*Producto{}
	for i := range ps {
		ps[i].Stock = []StockTalla{}
		ps[i].Unidades = 0
		porID[ps[i].ID] = &ps[i]
	}
	for rows.Next() {
		var pid int64
		var st StockTalla
		if err := rows.Scan(&pid, &st.Talla, &st.Cantidad); err != nil {
			return err
		}
		if p, ok := porID[pid]; ok {
			p.Stock = append(p.Stock, st)
			p.Unidades += st.Cantidad
		}
	}
	return rows.Err()
}

func validarStockMap(stock map[string]int) error {
	for talla, n := range stock {
		if !TallasValidas[talla] {
			return &ErrorValidacion{Campo: "stock",
				Mensaje: fmt.Sprintf("talla inválida %q: use S, M, L o XL", talla)}
		}
		if n < 0 {
			return &ErrorValidacion{Campo: "stock", Mensaje: "las cantidades no pueden ser negativas"}
		}
	}
	return nil
}

// ordenarTallas fija el orden canónico S, M, L, XL.
func ordenarTallas(stock map[string]int) []string {
	orden := []string{"S", "M", "L", "XL"}
	out := make([]string, 0, len(stock))
	visitadas := map[string]bool{}
	for _, t := range orden {
		if _, ok := stock[t]; ok {
			out = append(out, t)
			visitadas[t] = true
		}
	}
	extras := []string{}
	for t := range stock {
		if !visitadas[t] {
			extras = append(extras, t)
		}
	}
	sort.Strings(extras)
	return append(out, extras...)
}

type escaner interface {
	Scan(dest ...any) error
}

func escanearProducto(row escaner) (Producto, error) {
	var p Producto
	err := escanearProductoRow(row, &p)
	return p, err
}

func escanearProductoRow(row escaner, p *Producto) error {
	return row.Scan(&p.ID, &p.CategoriaID, &p.Categoria, &p.LoteID, &p.UnidadesLote,
		&p.Nombre, &p.Descripcion,
		&p.FotoPath, &p.Activo, &p.CreadoEn, &p.ActualizadoEn)
}
