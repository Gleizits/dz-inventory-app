package report

import "testing"

func TestPorcentajeGanancia_EjemploDelRequerimiento(t *testing.T) {
	// "precio 12 venta 18 porcentaje de ganancia 50%"
	if got := PorcentajeGanancia(12, 18); got != 50 {
		t.Errorf("PorcentajeGanancia(12, 18) = %v, quiero 50", got)
	}
}

func TestPorcentajeGanancia_CostoConEnvio(t *testing.T) {
	// lote 120 + envio 20 en 10 unidades = 14 de costo unitario, venta a 18
	if got := PorcentajeGanancia(14, 18); got != 28.57 {
		t.Errorf("PorcentajeGanancia(14, 18) = %v, quiero 28.57", got)
	}
}

func TestPorcentajeGanancia_CostoCero(t *testing.T) {
	if got := PorcentajeGanancia(0, 18); got != 0 {
		t.Errorf("con costo 0 debe devolver 0, no %v", got)
	}
}

func TestPorcentajeGanancia_Perida(t *testing.T) {
	if got := PorcentajeGanancia(18, 12); got != -33.33 {
		t.Errorf("PorcentajeGanancia(18, 12) = %v, quiero -33.33", got)
	}
}

func TestCalcularLinea(t *testing.T) {
	l := CalcularLinea(Linea{Unidades: 3, VentaTotal: 54, CostoCompra: 42})
	if l.Ganancia != 12 {
		t.Errorf("ganancia = %v, quiero 12", l.Ganancia)
	}
	if l.Porcentaje != 28.57 {
		t.Errorf("porcentaje = %v, quiero 28.57", l.Porcentaje)
	}
}

func TestCalcularTotal(t *testing.T) {
	lineas := []Linea{
		{Unidades: 3, VentaTotal: 54, CostoCompra: 42},
		{Unidades: 2, VentaTotal: 36, CostoCompra: 24},
	}
	total := CalcularTotal(lineas, 5)
	if total.Unidades != 5 || total.VentaTotal != 90 || total.CostoCompra != 66 {
		t.Errorf("total = %+v", total)
	}
	if total.Ganancia != 24 || total.Porcentaje != 36.36 {
		t.Errorf("ganancia/porcentaje = %v / %v, quiero 24 / 36.36", total.Ganancia, total.Porcentaje)
	}
	if total.Ventas != 5 || total.Productos != 2 {
		t.Errorf("conteos = %+v", total)
	}
}

func TestRedondear2(t *testing.T) {
	casos := map[float64]float64{
		1.004:  1.0,
		1.006:  1.01, // 1.005 exacto no existe en float64 (es 1.00499...), por eso se evita
		18.0:   18.0,
		33.333: 33.33,
	}
	for entrada, esperado := range casos {
		if got := Redondear2(entrada); got != esperado {
			t.Errorf("Redondear2(%v) = %v, quiero %v", entrada, got, esperado)
		}
	}
}
