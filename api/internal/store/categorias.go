package store

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// Categoria agrupa productos (ej. "Polos oversize").
type Categoria struct {
	ID           int64   `json:"id"`
	Nombre       string  `json:"nombre"`
	Descripcion  *string `json:"descripcion"`
	Activo       bool    `json:"activo"`
	CreadoEn     string  `json:"creado_en"`
	ActualizadoEn string `json:"actualizado_en"`
}

const colsCategoria = "id, nombre, descripcion, activo, " +
	"to_char(creado_en, 'YYYY-MM-DD HH24:MI:SS') , " +
	"to_char(actualizado_en, 'YYYY-MM-DD HH24:MI:SS')"

// ListarCategorias devuelve las categorías ordenadas por nombre.
func (s *Store) ListarCategorias(ctx context.Context, incluirInactivas bool) ([]Categoria, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT `+colsCategoria+` FROM categoria
		  WHERE $1 OR activo ORDER BY nombre`, incluirInactivas)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Categoria{}
	for rows.Next() {
		var c Categoria
		if err := rows.Scan(&c.ID, &c.Nombre, &c.Descripcion, &c.Activo, &c.CreadoEn, &c.ActualizadoEn); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ObtenerCategoria por id.
func (s *Store) ObtenerCategoria(ctx context.Context, id int64) (Categoria, error) {
	var c Categoria
	err := s.pool.QueryRow(ctx,
		`SELECT `+colsCategoria+` FROM categoria WHERE id = $1`, id).
		Scan(&c.ID, &c.Nombre, &c.Descripcion, &c.Activo, &c.CreadoEn, &c.ActualizadoEn)
	if errors.Is(err, pgx.ErrNoRows) {
		return c, fmt.Errorf("categoría %d: %w", id, ErrNoEncontrado)
	}
	return c, err
}

// CrearCategoria inserta una categoría con el nombre libre del usuario.
func (s *Store) CrearCategoria(ctx context.Context, nombre string, descripcion *string) (Categoria, error) {
	nombre = limpiar(nombre)
	if nombre == "" {
		return Categoria{}, &ErrorValidacion{Campo: "nombre", Mensaje: "el nombre de la categoría es obligatorio"}
	}
	var c Categoria
	err := s.pool.QueryRow(ctx,
		`INSERT INTO categoria (nombre, descripcion) VALUES ($1, $2)
		  RETURNING `+colsCategoria, nombre, descripcion).
		Scan(&c.ID, &c.Nombre, &c.Descripcion, &c.Activo, &c.CreadoEn, &c.ActualizadoEn)
	if err != nil && esDuplicado(err) {
		return Categoria{}, fmt.Errorf("ya existe una categoría llamada %q: %w", nombre, ErrConflicto)
	}
	return c, err
}

// ActualizarCategoria cambia nombre y descripción.
func (s *Store) ActualizarCategoria(ctx context.Context, id int64, nombre string, descripcion *string) (Categoria, error) {
	nombre = limpiar(nombre)
	if nombre == "" {
		return Categoria{}, &ErrorValidacion{Campo: "nombre", Mensaje: "el nombre de la categoría es obligatorio"}
	}
	var c Categoria
	err := s.pool.QueryRow(ctx,
		`UPDATE categoria SET nombre = $2, descripcion = $3 WHERE id = $1
		  RETURNING `+colsCategoria, id, nombre, descripcion).
		Scan(&c.ID, &c.Nombre, &c.Descripcion, &c.Activo, &c.CreadoEn, &c.ActualizadoEn)
	if errors.Is(err, pgx.ErrNoRows) {
		return c, fmt.Errorf("categoría %d: %w", id, ErrNoEncontrado)
	}
	if err != nil && esDuplicado(err) {
		return c, fmt.Errorf("ya existe una categoría llamada %q: %w", nombre, ErrConflicto)
	}
	return c, err
}

// DesactivarCategoria hace un borrado lógico. Se niega si todavía tiene
// productos activos, para no dejar prendas huérfanas invisibles.
func (s *Store) DesactivarCategoria(ctx context.Context, id int64) error {
	return s.transaccion(ctx, func(tx pgx.Tx) error {
		var nombre string
		err := tx.QueryRow(ctx, `SELECT nombre FROM categoria WHERE id = $1`, id).Scan(&nombre)
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("categoría %d: %w", id, ErrNoEncontrado)
		}
		if err != nil {
			return err
		}

		var activos int
		if err := tx.QueryRow(ctx,
			`SELECT count(*) FROM producto WHERE categoria_id = $1 AND activo`, id).Scan(&activos); err != nil {
			return err
		}
		if activos > 0 {
			return fmt.Errorf("la categoría %q tiene %d productos activos: muévalos o desactívelos primero: %w",
				nombre, activos, ErrConflicto)
		}
		_, err = tx.Exec(ctx, `UPDATE categoria SET activo = false WHERE id = $1`, id)
		return err
	})
}

// limpiar recorta y colapsa espacios/saltos de línea del texto del usuario.
func limpiar(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// esDuplicado detecta la violación de UNIQUE (23505).
func esDuplicado(err error) bool {
	return err != nil && (strings.Contains(err.Error(), "23505") ||
		strings.Contains(err.Error(), "duplicate key"))
}
