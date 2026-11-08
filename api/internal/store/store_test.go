package store

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// TestIntegracion corre contra una base de PostgreSQL real. Se omite si no
// está definida DZ_TEST_DATABASE_URL (así `go test ./...` pasa en cualquier
// máquina sin base levantada).
//
// Preparación (una sola vez):
//
//	podman exec dz-postgres psql -U dz -c "CREATE DATABASE dz_inventory_test;"
//	$env:DZ_TEST_DATABASE_URL="postgres://...@127.0.0.1:5432/dz_inventory_test"
//	go test ./internal/store/
//
// La prueba RESETEA el esquema público de esa base: úsala solo para tests.
func TestIntegracion(t *testing.T) {
	url := os.Getenv("DZ_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("DZ_TEST_DATABASE_URL no definida: se omite la prueba de integración")
	}
	ctx := context.Background()

	st, err := New(ctx, url)
	if err != nil {
		t.Fatalf("conexión: %v", err)
	}
	defer st.Cerrar()
	resetEsquema(t, st, ctx)

	// ------------------------------------------------------------------
	// 1) Categorías
	// ------------------------------------------------------------------
	cat, err := st.CrearCategoria(ctx, "Polos oversize", nil)
	if err != nil {
		t.Fatalf("crear categoría: %v", err)
	}
	if cat.ID == 0 || cat.Nombre != "Polos oversize" {
		t.Errorf("categoría = %+v", cat)
	}

	if _, err := st.CrearCategoria(ctx, "Polos oversize", nil); !errors.Is(err, ErrConflicto) {
		t.Errorf("categoría duplicada: esperaba ErrConflicto, llegó %v", err)
	}
	if _, err := st.CrearCategoria(ctx, "   ", nil); !esValidacion(err) {
		t.Errorf("nombre vacío: esperaba ErrorValidacion, llegó %v", err)
	}

	// ------------------------------------------------------------------
	// 2) Producto con stock inicial y lote: costo unitario derivado
	// ------------------------------------------------------------------
	lote := LoteInput{Unidades: 10, CostoTotal: 120, CostoEnvio: 20}
	prod, lotes, err := st.CrearProducto(ctx, ProductoInput{
		CategoriaID: cat.ID,
		Nombre:      "Polo negro oversize",
		Stock:       map[string]int{"S": 4, "M": 6, "XL": 0},
		Lote:        &lote,
	})
	if err != nil {
		t.Fatalf("crear producto: %v", err)
	}
	if prod.Unidades != 10 {
		t.Errorf("unidades = %d, quiero 10", prod.Unidades)
	}
	if len(prod.Stock) != 3 {
		t.Errorf("stock = %+v, quiero 3 tallas", prod.Stock)
	}
	if len(lotes) != 1 || lotes[0].CostoUnitario != 14 {
		t.Errorf("costo unitario = %+v, quiero 14.00 ((120+20)/10)", lotes)
	}

	// talla inválida
	if _, _, err := st.CrearProducto(ctx, ProductoInput{
		CategoriaID: cat.ID, Nombre: "Mal producto",
		Stock: map[string]int{"G": 3},
	}); !esValidacion(err) {
		t.Errorf("talla G: esperaba ErrorValidacion, llegó %v", err)
	}

	// unidades del lote distintas a la suma del stock
	if _, _, err := st.CrearProducto(ctx, ProductoInput{
		CategoriaID: cat.ID, Nombre: "Otro polo",
		Stock: map[string]int{"S": 1},
		Lote:  &LoteInput{Unidades: 5, CostoTotal: 50},
	}); err == nil {
		t.Error("debió rechazar unidades distintas a la suma del stock")
	}

	// ------------------------------------------------------------------
	// 3) Venta válida descuenta stock
	// ------------------------------------------------------------------
	venta, err := st.CrearVenta(ctx, VentaInput{
		Lineas: []LineaInput{{ProductoID: prod.ID, Talla: "M", Cantidad: 2, PrecioUnitario: 18}},
	})
	if err != nil {
		t.Fatalf("crear venta: %v", err)
	}
	if venta.Total != 36 || venta.Unidades != 2 {
		t.Errorf("venta = total %v / %d unidades, quiero 36 / 2", venta.Total, venta.Unidades)
	}
	stockM := stockDe(t, st, ctx, prod.ID, "M")
	if stockM != 4 {
		t.Errorf("stock M = %d, quiero 4 (6 - 2)", stockM)
	}

	// venta repetida del mismo producto+talla en la misma petición se fusiona
	venta2, err := st.CrearVenta(ctx, VentaInput{Lineas: []LineaInput{
		{ProductoID: prod.ID, Talla: "M", Cantidad: 1, PrecioUnitario: 18},
		{ProductoID: prod.ID, Talla: "M", Cantidad: 1, PrecioUnitario: 18},
	}})
	if err != nil {
		t.Fatalf("venta con líneas repetidas: %v", err)
	}
	if len(venta2.Lineas) != 1 || venta2.Lineas[0].Cantidad != 2 {
		t.Errorf("las líneas debieron fusionarse: %+v", venta2.Lineas)
	}

	// ------------------------------------------------------------------
	// 4) Venta por encima del stock: se niega y no cambia nada
	// ------------------------------------------------------------------
	antes := stockDe(t, st, ctx, prod.ID, "M")
	_, err = st.CrearVenta(ctx, VentaInput{Lineas: []LineaInput{
		{ProductoID: prod.ID, Talla: "M", Cantidad: 99, PrecioUnitario: 18},
	}})
	var es *ErrorStock
	if !errors.As(err, &es) {
		t.Fatalf("esperaba ErrorStock, llegó %v", err)
	}
	if es.Disponible != antes || es.Solicitado != 99 {
		t.Errorf("ErrorStock = %+v", es)
	}
	if stockDe(t, st, ctx, prod.ID, "M") != antes {
		t.Error("el stock no debe cambiar cuando la venta se rechaza")
	}

	// venta de una talla sin stock registrado
	if _, err := st.CrearVenta(ctx, VentaInput{Lineas: []LineaInput{
		{ProductoID: prod.ID, Talla: "L", Cantidad: 1, PrecioUnitario: 18},
	}}); err == nil {
		t.Error("debió rechazar la talla L (sin stock registrado)")
	}

	// ------------------------------------------------------------------
	// 5) Reporte antes de anular: costo promedio ponderado
	// ------------------------------------------------------------------
	rep, err := st.ReporteVentas(ctx, "", "")
	if err != nil {
		t.Fatalf("reporte: %v", err)
	}
	if rep.Total.Unidades != 4 {
		t.Errorf("unidades vendidas = %d, quiero 4 (2 + 2)", rep.Total.Unidades)
	}
	if rep.Total.VentaTotal != 72 {
		t.Errorf("venta total = %v, quiero 72", rep.Total.VentaTotal)
	}
	// costo: 4 unidades * 14 promedio = 56
	if rep.Total.CostoCompra != 56 {
		t.Errorf("costo de compra = %v, quiero 56", rep.Total.CostoCompra)
	}
	if rep.Total.Ganancia != 16 {
		t.Errorf("ganancia = %v, quiero 16", rep.Total.Ganancia)
	}
	if rep.Total.Porcentaje != 28.57 {
		t.Errorf("porcentaje = %v, quiero 28.57", rep.Total.Porcentaje)
	}
	if len(rep.PorDia) == 0 {
		t.Error("el desglose por día no puede venir vacío")
	}

	// ------------------------------------------------------------------
	// 6) Anular venta: repone stock y sale del reporte
	// ------------------------------------------------------------------
	anulada, err := st.AnularVenta(ctx, venta.ID)
	if err != nil {
		t.Fatalf("anular: %v", err)
	}
	if anulada.Estado != "anulada" {
		t.Errorf("estado = %q", anulada.Estado)
	}
	if stockDe(t, st, ctx, prod.ID, "M") != 4 {
		t.Errorf("stock M tras anular = %d, quiero 4 (6 - 2 de la venta que sigue activa)",
			stockDe(t, st, ctx, prod.ID, "M"))
	}
	if _, err := st.AnularVenta(ctx, venta.ID); !errors.Is(err, ErrConflicto) {
		t.Errorf("anular dos veces: esperaba ErrConflicto, llegó %v", err)
	}
	if _, err := st.AnularVenta(ctx, 999999); !errors.Is(err, ErrNoEncontrado) {
		t.Errorf("venta inexistente: esperaba ErrNoEncontrado, llegó %v", err)
	}

	rep, err = st.ReporteVentas(ctx, "", "")
	if err != nil {
		t.Fatalf("reporte tras anular: %v", err)
	}
	if rep.Total.Unidades != 2 {
		t.Errorf("tras anular debe quedar 1 venta con 2 unidades, llegó %d", rep.Total.Unidades)
	}

	// ------------------------------------------------------------------
	// 7) Stock en cero para la notificación
	// ------------------------------------------------------------------
	bajos, err := st.StockBajos(ctx, 0)
	if err != nil {
		t.Fatalf("stock bajos: %v", err)
	}
	enCero := map[string]bool{}
	for _, b := range bajos {
		if b.ProductoID == prod.ID {
			enCero[b.Talla] = true
		}
	}
	if !enCero["XL"] {
		t.Errorf("XL está en 0 y debe aparecer: %+v", bajos)
	}
	if enCero["S"] || enCero["M"] {
		t.Errorf("S y M no están en 0 y no deben aparecer: %+v", bajos)
	}

	// ------------------------------------------------------------------
	// 8) Borrado lógico y protecciones
	// ------------------------------------------------------------------
	if err := st.DesactivarCategoria(ctx, cat.ID); !errors.Is(err, ErrConflicto) {
		t.Errorf("categoría con productos activos: esperaba ErrConflicto, llegó %v", err)
	}
	if err := st.DesactivarProducto(ctx, prod.ID); err != nil {
		t.Errorf("desactivar producto: %v", err)
	}
	if err := st.DesactivarProducto(ctx, 999999); !errors.Is(err, ErrNoEncontrado) {
		t.Errorf("producto inexistente: esperaba ErrNoEncontrado, llegó %v", err)
	}
	if err := st.DesactivarCategoria(ctx, cat.ID); err != nil {
		t.Errorf("categoría sin productos activos debería poder desactivarse: %v", err)
	}

	// el producto desactivado ya no aparece en el catálogo por defecto
	lista, err := st.ListarProductos(ctx, FiltroProductos{})
	if err != nil {
		t.Fatalf("listar: %v", err)
	}
	for _, p := range lista {
		if p.ID == prod.ID {
			t.Error("el producto desactivado no debe listarse por defecto")
		}
	}
}

// ---------------------------------------------------------------------------

// esValidacion comprueba que el error sea de validación de entrada.
func esValidacion(err error) bool {
	var ev *ErrorValidacion
	return errors.As(err, &ev)
}

func stockDe(t *testing.T, st *Store, ctx context.Context, productoID int64, talla string) int {
	t.Helper()
	var n int
	err := st.pool.QueryRow(ctx,
		`SELECT cantidad FROM producto_talla WHERE producto_id = $1 AND talla = $2::talla_t`,
		productoID, talla).Scan(&n)
	if err != nil {
		t.Fatalf("leer stock de %s: %v", talla, err)
	}
	return n
}

// resetEsquema deja la base limpia y aplica db/migrations/*.sql.
func resetEsquema(t *testing.T, st *Store, ctx context.Context) {
	t.Helper()
	conn, err := st.pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("adquirir conexión: %v", err)
	}
	defer conn.Release()
	pg := conn.Conn().PgConn()

	if _, err := pg.Exec(ctx, "DROP SCHEMA public CASCADE; CREATE SCHEMA public;").ReadAll(); err != nil {
		t.Fatalf("resetear esquema: %v", err)
	}

	ruta := filepath.Join("..", "..", "..", "db", "migrations")
	archivos, err := filepath.Glob(filepath.Join(ruta, "*.sql"))
	if err != nil || len(archivos) == 0 {
		t.Fatalf("no encontré migraciones en %s (err=%v)", ruta, err)
	}
	sort.Strings(archivos)
	for _, archivo := range archivos {
		sql, err := os.ReadFile(archivo)
		if err != nil {
			t.Fatalf("leer %s: %v", archivo, err)
		}
		if _, err := pg.Exec(ctx, string(sql)).ReadAll(); err != nil {
			t.Fatalf("aplicar %s: %v", filepath.Base(archivo), err)
		}
	}
}
