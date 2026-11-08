package store

import (
	"context"

	"github.com/Gleizits/dz-inventory-app/api/internal/report"
)

// Reporte es el resultado de "ventas por rango de fechas": lo que la app
// muestra y exporta a PDF.
type Reporte struct {
	Desde       string          `json:"desde"`
	Hasta       string          `json:"hasta"`
	Total       report.Total    `json:"total"`
	PorProducto []report.Linea  `json:"por_producto"`
	PorDia      []ReporteDia    `json:"por_dia"`
}

// ReporteDia es una fila del desglose diario.
type ReporteDia struct {
	Fecha       string  `json:"fecha"`
	Unidades    int     `json:"unidades"`
	VentaTotal  float64 `json:"venta_total"`
	CostoCompra float64 `json:"costo_compra"`
	Ganancia    float64 `json:"ganancia"`
}

// StockBajo es una talla que llegó al mínimo (para la notificación).
type StockBajo struct {
	ProductoID int64  `json:"producto_id"`
	Producto   string `json:"producto"`
	Categoria  string `json:"categoria"`
	Talla      string `json:"talla"`
	Cantidad   int    `json:"cantidad"`
}

// ReporteVentas calcula unidades vendidas, costo de compra, venta total,
// ganancia y porcentaje para un rango de fechas.
//
// El costo de compra es el COSTO PROMEDIO PONDERADO de los lotes del
// producto con fecha <= fin del rango (nunca se guarda el costo por venta:
// eso rompería la 3FN).
func (s *Store) ReporteVentas(ctx context.Context, desde, hasta string) (Reporte, error) {
	d, h, err := rangoFechas(desde, hasta)
	if err != nil {
		return Reporte{}, err
	}

	// 1) unidades y venta por producto en el rango
	rows, err := s.pool.Query(ctx,
		`SELECT dv.producto_id, p.nombre, c.nombre,
		        SUM(dv.cantidad)::int,
		        SUM(dv.cantidad * dv.precio_unitario)::float8
		   FROM detalle_venta dv
		   JOIN venta v    ON v.id = dv.venta_id AND v.estado = 'activa'
		   JOIN producto p ON p.id = dv.producto_id
		   JOIN categoria c ON c.id = p.categoria_id
		  WHERE v.fecha BETWEEN $1 AND $2
		  GROUP BY dv.producto_id, p.nombre, c.nombre
		  ORDER BY p.nombre`, d, h)
	if err != nil {
		return Reporte{}, err
	}
	defer rows.Close()

	lineas := []report.Linea{}
	costoPorProducto := map[int64]float64{}
	for rows.Next() {
		var l report.Linea
		if err := rows.Scan(&l.ProductoID, &l.Producto, &l.Categoria, &l.Unidades, &l.VentaTotal); err != nil {
			return Reporte{}, err
		}
		lineas = append(lineas, l)
	}
	if err := rows.Err(); err != nil {
		return Reporte{}, err
	}

	// 2) costo promedio ponderado por producto (lotes hasta el fin del rango)
	costos, err := s.pool.Query(ctx,
		`SELECT producto_id,
		        SUM(costo_total + costo_envio)::float8 / NULLIF(SUM(unidades), 0)::float8
		   FROM lote
		  WHERE fecha <= $1
		  GROUP BY producto_id`, h)
	if err != nil {
		return Reporte{}, err
	}
	defer costos.Close()
	for costos.Next() {
		var id int64
		var promedio float64
		if err := costos.Scan(&id, &promedio); err != nil {
			return Reporte{}, err
		}
		costoPorProducto[id] = Redondear(promedio)
	}
	if err := costos.Err(); err != nil {
		return Reporte{}, err
	}

	// 3) completar cada línea con costo, ganancia y porcentaje
	for i := range lineas {
		l := &lineas[i]
		promedio := costoPorProducto[l.ProductoID]
		l.CostoCompra = Redondear(promedio * float64(l.Unidades))
		lineas[i] = report.CalcularLinea(*l)
	}

	// 4) conteo de ventas del rango (para el total)
	var nVentas int
	if err := s.pool.QueryRow(ctx,
		`SELECT count(*) FROM venta WHERE estado = 'activa' AND fecha BETWEEN $1 AND $2`,
		d, h).Scan(&nVentas); err != nil {
		return Reporte{}, err
	}

	// 5) desglose por día
	dias, err := s.pool.Query(ctx,
		`SELECT v.fecha::date::text, dv.producto_id,
		        SUM(dv.cantidad)::int,
		        SUM(dv.cantidad * dv.precio_unitario)::float8
		   FROM detalle_venta dv
		   JOIN venta v ON v.id = dv.venta_id AND v.estado = 'activa'
		  WHERE v.fecha BETWEEN $1 AND $2
		  GROUP BY 1, 2
		  ORDER BY 1`, d, h)
	if err != nil {
		return Reporte{}, err
	}
	defer dias.Close()

	porDia := map[string]*ReporteDia{}
	ordenDias := []string{}
	for dias.Next() {
		var fecha string
		var productoID int64
		var unidades int
		var venta float64
		if err := dias.Scan(&fecha, &productoID, &unidades, &venta); err != nil {
			return Reporte{}, err
		}
		fila, ok := porDia[fecha]
		if !ok {
			fila = &ReporteDia{Fecha: fecha}
			porDia[fecha] = fila
			ordenDias = append(ordenDias, fecha)
		}
		fila.Unidades += unidades
		fila.VentaTotal += venta
		fila.CostoCompra += costoPorProducto[productoID] * float64(unidades)
	}
	if err := dias.Err(); err != nil {
		return Reporte{}, err
	}

	porDiaLista := make([]ReporteDia, 0, len(ordenDias))
	for _, fecha := range ordenDias {
		f := porDia[fecha]
		f.VentaTotal = Redondear(f.VentaTotal)
		f.CostoCompra = Redondear(f.CostoCompra)
		f.Ganancia = Redondear(f.VentaTotal - f.CostoCompra)
		porDiaLista = append(porDiaLista, *f)
	}

	return Reporte{
		Desde:       d.Format("2006-01-02"),
		Hasta:       h.Format("2006-01-02"),
		Total:       report.CalcularTotal(lineas, nVentas),
		PorProducto: lineas,
		PorDia:      porDiaLista,
	}, nil
}

// StockBajos devuelve las tallas en cero (o por debajo del umbral) para que
// la app dispare la notificación "sin stock".
func (s *Store) StockBajos(ctx context.Context, umbral int) ([]StockBajo, error) {
	if umbral < 0 {
		umbral = 0
	}
	rows, err := s.pool.Query(ctx,
		`SELECT p.id, p.nombre, c.nombre, pt.talla::text, pt.cantidad
		   FROM producto_talla pt
		   JOIN producto p   ON p.id = pt.producto_id
		   JOIN categoria c  ON c.id = p.categoria_id
		  WHERE p.activo AND pt.cantidad <= $1
		  ORDER BY pt.cantidad, p.nombre, pt.talla`, umbral)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []StockBajo{}
	for rows.Next() {
		var b StockBajo
		if err := rows.Scan(&b.ProductoID, &b.Producto, &b.Categoria, &b.Talla, &b.Cantidad); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}
