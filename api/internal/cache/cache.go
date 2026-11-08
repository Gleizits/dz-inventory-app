// Package cache envuelve Redis para cachear las consultas repetidas
// (catálogo, stock y reportes) y evitar golpear la base de datos.
//
// Estrategia: un contador global "dz:gen". Cada clave lleva la generación
// actual dentro, así que invalidar es un solo INCR (no hay que buscar claves
// con patrones). Si Redis no está disponible, la API sigue funcionando sin
// caché en lugar de fallar.
package cache

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
)

const prefijo = "dz:"

type Cache struct {
	rdb *redis.Client
	ttl time.Duration
}

// New crea el caché. Si la URL es vacía o Redis no responde, devuelve un
// caché "apagado" (todas las operaciones son no-op).
func New(url string, ttl time.Duration) *Cache {
	if url == "" {
		return &Cache{}
	}
	opt, err := redis.ParseURL(url)
	if err != nil {
		return &Cache{}
	}
	rdb := redis.NewClient(opt)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return &Cache{}
	}
	if ttl <= 0 {
		ttl = time.Minute
	}
	return &Cache{rdb: rdb, ttl: ttl}
}

// Disponible informa si el caché está operativo.
func (c *Cache) Disponible() bool { return c != nil && c.rdb != nil }

// Gen devuelve la generación actual de caché ("0" si no hay Redis).
func (c *Cache) Gen(ctx context.Context) string {
	if !c.Disponible() {
		return "0"
	}
	v, err := c.rdb.Get(ctx, prefijo+"gen").Result()
	if err == redis.Nil {
		return "0"
	}
	if err != nil {
		return "0"
	}
	return v
}

// Invalidar descarta todo el caché vigente (se llama en cada escritura).
func (c *Cache) Invalidar(ctx context.Context) {
	if !c.Disponible() {
		return
	}
	_ = c.rdb.Incr(ctx, prefijo+"gen").Err()
}

// Obtener devuelve true y llena destino si la clave existe.
func (c *Cache) Obtener(ctx context.Context, gen, clave string, destino any) bool {
	if !c.Disponible() {
		return false
	}
	bruto, err := c.rdb.Get(ctx, prefijo+gen+":"+clave).Bytes()
	if err != nil {
		return false
	}
	return json.Unmarshal(bruto, destino) == nil
}

// Guardar guarda el valor con el TTL configurado.
func (c *Cache) Guardar(ctx context.Context, gen, clave string, valor any) {
	if !c.Disponible() {
		return
	}
	bruto, err := json.Marshal(valor)
	if err != nil {
		return
	}
	_ = c.rdb.Set(ctx, prefijo+gen+":"+clave, bruto, c.ttl).Err()
}

// Cerrar libera la conexión con Redis.
func (c *Cache) Cerrar() error {
	if !c.Disponible() {
		return nil
	}
	return c.rdb.Close()
}
