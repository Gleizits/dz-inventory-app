package cache

import (
	"context"
	"testing"
	"time"
)

// Sin Redis disponible el caché debe ser no-op, nunca un error.
func TestSinRedis_NoFalla(t *testing.T) {
	c := &Cache{}
	ctx := context.Background()

	if c.Disponible() {
		t.Error("un caché vacío no debe estar disponible")
	}
	if c.Gen(ctx) != "0" {
		t.Error("sin Redis la generación debe ser 0")
	}
	if c.Obtener(ctx, "0", "clave", new(map[string]any)) {
		t.Error("sin Redis nunca debe haber acierto")
	}
	c.Invalidar(ctx)     // no debe panic
	c.Guardar(ctx, "0", "clave", "x") // no debe panic
	if err := c.Cerrar(); err != nil {
		t.Errorf("Cerrar: %v", err)
	}
}

func TestURLInvalida_NoFalla(t *testing.T) {
	c := New("esto-no-es-una-url", time.Minute)
	if c.Disponible() {
		t.Error("una URL inválida debe dar un caché apagado")
	}
}

func TestObtener_Guardar_ConRedisLocal(t *testing.T) {
	url := "redis://127.0.0.1:6379/15" // base 15 para no tocar la de desarrollo
	c := New(url, time.Minute)
	if !c.Disponible() {
		t.Skip("Redis no está corriendo; se omite esta prueba")
	}
	ctx := context.Background()
	c.Invalidar(ctx)
	gen := c.Gen(ctx)

	type dato struct {
		Nombre string `json:"nombre"`
		Cantidad int  `json:"cantidad"`
	}
	entrada := dato{Nombre: "Polo negro", Cantidad: 10}

	if c.Obtener(ctx, gen, "prueba:familia", new(dato)) {
		t.Fatal("no debe haber acierto antes de guardar")
	}
	c.Guardar(ctx, gen, "prueba:familia", entrada)

	var salida dato
	if !c.Obtener(ctx, gen, "prueba:familia", &salida) {
		t.Fatal("debió haber acierto tras guardar")
	}
	if salida != entrada {
		t.Errorf("salida = %+v, quiero %+v", salida, entrada)
	}

	// Invalidar cambia la generación: la clave vieja deja de verse.
	c.Invalidar(ctx)
	nuevaGen := c.Gen(ctx)
	if nuevaGen == gen {
		t.Error("Invalidar debe aumentar la generación")
	}
	if c.Obtener(ctx, nuevaGen, "prueba:familia", new(dato)) {
		t.Error("tras invalidar no debe haber acierto")
	}
}
