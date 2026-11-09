package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// LineaVenta es una fila de la venta: producto, talla, unidades y precio
// de venta del momento (ese precio varía venta a venta, por eso está acá).
type LineaVenta struct {
	DetalleID      int64   `json:"detalle_id"`
	ProductoID     int64   `json:"producto_id"`
	Producto       string  `json:"producto"`
	Talla          string  `json:"talla"`
	Cantidad       int     `json:"cantidad"`
	PrecioUnitario float64 `json:"precio_unitario"`
	Subtotal       float64 `json:"subtotal"`
}

// Venta es el registro de una venta (sin caja, sin cliente, sin SUNAT).
type Venta struct {
	ID            int64        `json:"id"`
	Fecha         string       `json:"fecha"`
	Estado        string       `json:"estado"`
	Notas         *string      `json:"notas"`
	Unidades      int          `json:"unidades"`
	Total         float64      `json:"total"`
	Lineas        []LineaVenta `json:"lineas"`
	CreadoEn      string       `json:"creado_en"`
	ActualizadoEn string       `json:"actualizado_en"`
}

// VentaInput lo que manda la app al registrar una venta.
type VentaInput struct {
	Notas  *string      `json:"notas"`
	Lineas []LineaInput `json:"lineas"`
}

// LineaInput es un producto vendido en una talla.
type LineaInput struct {
	ProductoID     int64   `json:"producto_id"`
	Talla          string  `json:"talla"`
	Cantidad       int     `json:"cantidad"`
	PrecioUnitario float64 `json:"precio_unitario"`
}

// CrearVenta valida el stock de cada línea, descuenta y registra la venta,
// todo en una sola transacción: o entra completa o no entra nada.
func (s *Store) CrearVenta(ctx context.Context, in VentaInput) (Venta, error) {
	lineas, err := validarLineas(in.Lineas)
	if err != nil {
		return Venta{}, err
	}

	var v Venta
	err = s.transaccion(ctx, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx,
			`INSERT INTO venta (notas) VALUES ($1)
			  RETURNING id, to_char(fecha, 'YYYY-MM-DD HH24:MI:SS'), estado, notas,
			            to_char(creado_en, 'YYYY-MM-DD HH24:MI:SS'),
			            to_char(actualizado_en, 'YYYY-MM-DD HH24:MI:SS')`,
			in.Notas).
			Scan(&v.ID, &v.Fecha, &v.Estado, &v.Notas, &v.CreadoEn, &v.ActualizadoEn); err != nil {
			return err
		}

		for i := range lineas {
			l := &lineas[i]
			// FOR UPDATE bloquea la fila de stock: dos ventas simultáneas no
			// pueden vender las mismas unidades.
			var nombre string
			var disponible int
			err := tx.QueryRow(ctx,
				`SELECT p.nombre, pt.cantidad
				   FROM producto p
				   JOIN producto_talla pt ON pt.producto_id = p.id
				  WHERE p.id = $1 AND pt.talla = $2::talla_t
				  FOR UPDATE OF pt`, l.ProductoID, l.Talla).
				Scan(&nombre, &disponible)
			if errors.Is(err, pgx.ErrNoRows) {
				return &ErrorValidacion{Campo: "lineas",
					Mensaje: fmt.Sprintf("la prenda %d no tiene stock registrado en talla %s", l.ProductoID, l.Talla)}
			}
			if err != nil {
				return err
			}
			if disponible < l.Cantidad {
				return &ErrorStock{ProductoID: l.ProductoID, Producto: nombre, Talla: l.Talla,
					Disponible: disponible, Solicitado: l.Cantidad}
			}
			l.Producto = nombre

			if err := tx.QueryRow(ctx,
				`INSERT INTO detalle_venta (venta_id, producto_id, talla, cantidad, precio_unitario)
				  VALUES ($1, $2, $3::talla_t, $4, $5::float8) RETURNING id`,
				v.ID, l.ProductoID, l.Talla, l.Cantidad, l.PrecioUnitario).
				Scan(&l.DetalleID); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx,
				`UPDATE producto_talla SET cantidad = cantidad - $3
				  WHERE producto_id = $1 AND talla = $2::talla_t`,
				l.ProductoID, l.Talla, l.Cantidad); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return Venta{}, err
	}

	v.Lineas = lineas
	for _, l := range lineas {
		v.Unidades += l.Cantidad
		v.Total += float64(l.Cantidad) * l.PrecioUnitario
	}
	v.Total = Redondear(v.Total)
	return v, nil
}

// ListarVentas devuelve las ventas de un rango con sus líneas.
func (s *Store) ListarVentas(ctx context.Context, desde, hasta string, limite, offset int) ([]Venta, error) {
	d, h, err := rangoFechas(desde, hasta)
	if err != nil {
		return nil, err
	}
	if limite <= 0 || limite > 500 {
		limite = 200
	}

	rows, err := s.pool.Query(ctx,
		`SELECT v.id, to_char(v.fecha, 'YYYY-MM-DD HH24:MI:SS'), v.estado, v.notas,
		        COALESCE(SUM(dv.cantidad), 0)::int,
		        COALESCE(SUM(dv.cantidad * dv.precio_unitario), 0)::float8,
		        to_char(v.creado_en, 'YYYY-MM-DD HH24:MI:SS'),
		        to_char(v.actualizado_en, 'YYYY-MM-DD HH24:MI:SS')
		   FROM venta v
		   LEFT JOIN detalle_venta dv ON dv.venta_id = v.id
		  WHERE v.fecha BETWEEN $1 AND $2
		  GROUP BY v.id, v.fecha, v.estado, v.notas, v.creado_en, v.actualizado_en
		  ORDER BY v.fecha DESC
		  LIMIT $3 OFFSET $4`, d, h, limite, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ventas := []Venta{}
	ids := []int64{}
	for rows.Next() {
		var v Venta
		if err := rows.Scan(&v.ID, &v.Fecha, &v.Estado, &v.Notas, &v.Unidades, &v.Total,
			&v.CreadoEn, &v.ActualizadoEn); err != nil {
			return nil, err
		}
		v.Lineas = []LineaVenta{}
		ventas = append(ventas, v)
		ids = append(ids, v.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return ventas, nil
	}

	lr, err := s.pool.Query(ctx,
		`SELECT dv.venta_id, dv.id, dv.producto_id, p.nombre, dv.talla::text, dv.cantidad,
		        dv.precio_unitario::float8
		   FROM detalle_venta dv JOIN producto p ON p.id = dv.producto_id
		  WHERE dv.venta_id = ANY($1)
		  ORDER BY p.nombre, dv.talla`, ids)
	if err != nil {
		return nil, err
	}
	defer lr.Close()

	porVenta := map[int64]*Venta{}
	for i := range ventas {
		porVenta[ventas[i].ID] = &ventas[i]
	}
	for lr.Next() {
		var ventaID int64
		var l LineaVenta
		if err := lr.Scan(&ventaID, &l.DetalleID, &l.ProductoID, &l.Producto, &l.Talla,
			&l.Cantidad, &l.PrecioUnitario); err != nil {
			return nil, err
		}
		l.Subtotal = Redondear(float64(l.Cantidad) * l.PrecioUnitario)
		if v, ok := porVenta[ventaID]; ok {
			v.Lineas = append(v.Lineas, l)
		}
	}
	return ventas, lr.Err()
}

// AnularVenta marca la venta como anulada y devuelve las unidades al stock.
// Solo se puede anular una venta activa.
func (s *Store) AnularVenta(ctx context.Context, id int64) (Venta, error) {
	var v Venta
	err := s.transaccion(ctx, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx,
			`UPDATE venta SET estado = 'anulada' WHERE id = $1 AND estado = 'activa'
			  RETURNING id, to_char(fecha, 'YYYY-MM-DD HH24:MI:SS'), estado, notas,
			            to_char(creado_en, 'YYYY-MM-DD HH24:MI:SS'),
			            to_char(actualizado_en, 'YYYY-MM-DD HH24:MI:SS')`, id).
			Scan(&v.ID, &v.Fecha, &v.Estado, &v.Notas, &v.CreadoEn, &v.ActualizadoEn)
		if errors.Is(err, pgx.ErrNoRows) {
			var estado string
			e := tx.QueryRow(ctx, `SELECT estado FROM venta WHERE id = $1`, id).Scan(&estado)
			if errors.Is(e, pgx.ErrNoRows) {
				return fmt.Errorf("venta %d: %w", id, ErrNoEncontrado)
			}
			return fmt.Errorf("la venta %d ya está %s: %w", id, estado, ErrConflicto)
		}
		if err != nil {
			return err
		}

		// Repone el stock (contrato documentado en V1__init.sql)
		_, err = tx.Exec(ctx,
			`UPDATE producto_talla pt
			    SET cantidad = pt.cantidad + dv.cantidad
			   FROM detalle_venta dv
			  WHERE dv.venta_id = $1
			    AND dv.producto_id = pt.producto_id
			    AND dv.talla = pt.talla`, id)
		return err
	})
	if err != nil {
		return v, err
	}

	lineas, err := s.lineasDeVenta(ctx, id)
	if err != nil {
		return v, err
	}
	v.Lineas = lineas
	for _, l := range lineas {
		v.Unidades += l.Cantidad
		v.Total += float64(l.Cantidad) * l.PrecioUnitario
	}
	v.Total = Redondear(v.Total)
	return v, nil
}

// lineasDeVenta carga las líneas de una venta.
func (s *Store) lineasDeVenta(ctx context.Context, ventaID int64) ([]LineaVenta, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT dv.id, dv.producto_id, p.nombre, dv.talla::text, dv.cantidad,
		        dv.precio_unitario::float8
		   FROM detalle_venta dv JOIN producto p ON p.id = dv.producto_id
		  WHERE dv.venta_id = $1 ORDER BY p.nombre, dv.talla`, ventaID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []LineaVenta{}
	for rows.Next() {
		var l LineaVenta
		if err := rows.Scan(&l.DetalleID, &l.ProductoID, &l.Producto, &l.Talla,
			&l.Cantidad, &l.PrecioUnitario); err != nil {
			return nil, err
		}
		l.Subtotal = Redondear(float64(l.Cantidad) * l.PrecioUnitario)
		out = append(out, l)
	}
	return out, rows.Err()
}

// validarLineas valida cada línea y fusiona las repeticiones del mismo
// producto+talla (la BD tiene una línea única por combinación).
func validarLineas(lineas []LineaInput) ([]LineaVenta, error) {
	if len(lineas) == 0 {
		return nil, &ErrorValidacion{Campo: "lineas", Mensaje: "la venta debe tener al menos una línea"}
	}
	fusion := map[string]*LineaVenta{}
	orden := []string{}
	for i, l := range lineas {
		campo := fmt.Sprintf("lineas[%d]", i)
		if err := validarTalla(campo+".talla", l.Talla); err != nil {
			return nil, err
		}
		if l.ProductoID <= 0 {
			return nil, &ErrorValidacion{Campo: campo + ".producto_id", Mensaje: "producto inválido"}
		}
		if l.Cantidad <= 0 {
			return nil, &ErrorValidacion{Campo: campo + ".cantidad", Mensaje: "las unidades vendidas deben ser mayores a cero"}
		}
		if l.PrecioUnitario < 0 {
			return nil, &ErrorValidacion{Campo: campo + ".precio_unitario", Mensaje: "el precio de venta no puede ser negativo"}
		}
		clave := fmt.Sprintf("%d|%s", l.ProductoID, l.Talla)
		if existente, ok := fusion[clave]; ok {
			if existente.PrecioUnitario != l.PrecioUnitario {
				return nil, &ErrorValidacion{Campo: campo + ".precio_unitario",
					Mensaje: "mismo producto y talla no puede tener dos precios distintos en la misma venta"}
			}
			existente.Cantidad += l.Cantidad
			continue
		}
		fusion[clave] = &LineaVenta{ProductoID: l.ProductoID, Talla: l.Talla,
			Cantidad: l.Cantidad, PrecioUnitario: Redondear(l.PrecioUnitario)}
		orden = append(orden, clave)
	}
	out := make([]LineaVenta, 0, len(orden))
	for _, k := range orden {
		out = append(out, *fusion[k])
	}
	return out, nil
}

// rangoFechas convierte "AAAA-MM-DD" a instantes; si no vienen, usa todo el
// mes en curso.
func rangoFechas(desde, hasta string) (time.Time, time.Time, error) {
	if desde == "" {
		hoy := time.Now()
		d := time.Date(hoy.Year(), hoy.Month(), 1, 0, 0, 0, 0, hoy.Location())
		desde = d.Format("2006-01-02")
	}
	if hasta == "" {
		hasta = time.Now().Format("2006-01-02")
	}
	d, err := fechaRango(desde, false)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	h, err := fechaRango(hasta, true)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	if h.Before(d) {
		return time.Time{}, time.Time{}, &ErrorValidacion{Campo: "hasta",
			Mensaje: "la fecha final no puede ser anterior a la inicial"}
	}
	return d, h, nil
}
