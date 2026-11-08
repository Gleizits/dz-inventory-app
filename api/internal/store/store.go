// Package store centraliza el acceso a PostgreSQL. Todas las reglas de
// negocio que tocan datos (validación de stock, anulación de ventas,
// costo promedio ponderado) viven acá, en transacciones.
package store

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Errores de dominio que los handlers traducen a códigos HTTP.
var (
	ErrNoEncontrado = errors.New("no encontrado")
	ErrConflicto    = errors.New("conflicto")
)

// ErrorValidacion: el cliente mandó algo inválido (HTTP 400).
type ErrorValidacion struct {
	Campo   string `json:"campo"`
	Mensaje string `json:"mensaje"`
}

func (e *ErrorValidacion) Error() string {
	return fmt.Sprintf("%s: %s", e.Campo, e.Mensaje)
}

// ErrorStock: la venta supera el stock disponible (HTTP 409).
type ErrorStock struct {
	ProductoID int64  `json:"producto_id"`
	Producto   string `json:"producto"`
	Talla      string `json:"talla"`
	Disponible int    `json:"disponible"`
	Solicitado int    `json:"solicitado"`
}

func (e *ErrorStock) Error() string {
	return fmt.Sprintf("stock insuficiente de %s talla %s: hay %d, pediste %d",
		e.Producto, e.Talla, e.Disponible, e.Solicitado)
}

// Tallas válidas: el negocio solo trabaja con estas cuatro.
var TallasValidas = map[string]bool{"S": true, "M": true, "L": true, "XL": true}

type Store struct {
	pool *pgxpool.Pool
}

// New abre el pool de conexiones a PostgreSQL.
func New(ctx context.Context, url string) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("URL de base de datos inválida: %w", err)
	}
	cfg.MaxConns = 5
	cfg.MaxConnLifetime = time.Hour

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("no se pudo conectar a PostgreSQL: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("PostgreSQL no responde: %w", err)
	}
	return &Store{pool: pool}, nil
}

// Ping comprueba que la base responda (para /api/salud).
func (s *Store) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

// Cerrar libera el pool.
func (s *Store) Cerrar() { s.pool.Close() }

// transaccion ejecuta fn dentro de una transacción con rollback automático.
func (s *Store) transaccion(ctx context.Context, fn func(tx pgx.Tx) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // el rollback si no hay commit es correcto
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// validarTalla devuelve un ErrorValidacion si la talla no está permitida.
func validarTalla(campo, talla string) error {
	if !TallasValidas[talla] {
		return &ErrorValidacion{Campo: campo,
			Mensaje: fmt.Sprintf("talla inválida %q: use S, M, L o XL", talla)}
	}
	return nil
}

// Redondear redondea importes a 2 decimales (los montos viajan como float8
// a JSON, así evitamos ruido binario tipo 18.000000000000004).
func Redondear(v float64) float64 {
	return math.Round(v*100) / 100
}

// fechaRango convierte "2026-11-08" a un instante medianoche local.
func fechaRango(s string, fin bool) (time.Time, error) {
	t, err := time.ParseInLocation("2006-01-02", s, time.Local)
	if err != nil {
		return time.Time{}, &ErrorValidacion{Campo: "fecha",
			Mensaje: fmt.Sprintf("%q no es una fecha válida (use AAAA-MM-DD)", s)}
	}
	if fin {
		t = t.Add(24*time.Hour - time.Nanosecond)
	}
	return t, nil
}
