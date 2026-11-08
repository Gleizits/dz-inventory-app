// Package report concentra los cálculos de los reportes de ventas:
// unidades, costo de compra, venta, ganancia y porcentaje de ganancia.
//
// Regla de negocio acordada: el costo de compra es el COSTO PROMEDIO
// PONDERADO de los lotes del producto con fecha <= fin del rango reportado
// (nunca se guarda un costo "histórico" por venta, eso rompería la 3FN).
package report

import "math"

// Linea es una fila del reporte por producto.
type Linea struct {
	ProductoID   int64   `json:"producto_id"`
	Producto     string  `json:"producto"`
	Categoria    string  `json:"categoria"`
	Unidades     int     `json:"unidades"`
	VentaTotal   float64 `json:"venta_total"`
	CostoCompra  float64 `json:"costo_compra"`
	Ganancia     float64 `json:"ganancia"`
	Porcentaje   float64 `json:"porcentaje_ganancia"`
}

// Total agrega las cifras de todo el rango.
type Total struct {
	Productos      int     `json:"productos"`
	Ventas         int     `json:"ventas"`
	Unidades       int     `json:"unidades_vendidas"`
	VentaTotal     float64 `json:"venta_total"`
	CostoCompra    float64 `json:"costo_compra"`
	Ganancia       float64 `json:"ganancia"`
	Porcentaje     float64 `json:"porcentaje_ganancia"`
}

// PorcentajeGanancia devuelve ((venta - costo) / costo) * 100 redondeado a 2
// decimales. Ejemplo del requerimiento: costo 12, venta 18 -> 50%.
// Si el costo es 0 no hay base para calcular: devuelve 0.
func PorcentajeGanancia(costo, venta float64) float64 {
	if costo <= 0 {
		return 0
	}
	return Redondear2((venta - costo) / costo * 100)
}

// CalcularLinea completa una línea con ganancia y porcentaje.
func CalcularLinea(l Linea) Linea {
	l.VentaTotal = Redondear2(l.VentaTotal)
	l.CostoCompra = Redondear2(l.CostoCompra)
	l.Ganancia = Redondear2(l.VentaTotal - l.CostoCompra)
	l.Porcentaje = PorcentajeGanancia(l.CostoCompra, l.VentaTotal)
	return l
}

// CalcularTotal agrega las líneas y devuelve el total del rango.
func CalcularTotal(lineas []Linea, ventas int) Total {
	t := Total{Productos: len(lineas), Ventas: ventas}
	for _, l := range lineas {
		t.Unidades += l.Unidades
		t.VentaTotal += l.VentaTotal
		t.CostoCompra += l.CostoCompra
	}
	t.VentaTotal = Redondear2(t.VentaTotal)
	t.CostoCompra = Redondear2(t.CostoCompra)
	t.Ganancia = Redondear2(t.VentaTotal - t.CostoCompra)
	t.Porcentaje = PorcentajeGanancia(t.CostoCompra, t.VentaTotal)
	return t
}

// Redondear2 redondea a 2 decimales. Ojo con los decimales que no existen
// exactamente en float64 (1.005 en realidad es 1.00499...): se redondea el
// valor real que tiene la máquina.
func Redondear2(v float64) float64 {
	return math.Round(v*100) / 100
}
