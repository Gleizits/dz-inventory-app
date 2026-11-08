-- =============================================================================
-- DZ Urban Style — Inventario de prendas
-- Migración V1: esquema inicial (normalizado hasta 3FN, listo para producción)
-- Compatible con PostgreSQL 17 local y con Supabase (schema public).
-- =============================================================================

-- -----------------------------------------------------------------------------
-- Tipos enumerados
-- -----------------------------------------------------------------------------
CREATE TYPE talla_t      AS ENUM ('S', 'M', 'L', 'XL');
CREATE TYPE estado_venta_t AS ENUM ('activa', 'anulada');

-- -----------------------------------------------------------------------------
-- Trigger: mantiene actualizado_en en todas las tablas que lo usan
-- -----------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION f_set_actualizado_en() RETURNS trigger AS $$
BEGIN
    NEW.actualizado_en := now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- -----------------------------------------------------------------------------
-- categoria: el usuario escribe el nombre libremente
-- -----------------------------------------------------------------------------
CREATE TABLE categoria (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    nombre         TEXT        NOT NULL,
    descripcion    TEXT,
    activo         BOOLEAN     NOT NULL DEFAULT TRUE,
    creado_en      TIMESTAMPTZ NOT NULL DEFAULT now(),
    actualizado_en TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT categoria_nombre_unico UNIQUE (nombre),
    CONSTRAINT categoria_nombre_no_vacio CHECK (length(trim(nombre)) > 0)
);

CREATE TRIGGER trg_categoria_actualizado
    BEFORE UPDATE ON categoria
    FOR EACH ROW EXECUTE FUNCTION f_set_actualizado_en();

-- -----------------------------------------------------------------------------
-- producto: prenda perteneciente a una categoría
-- -----------------------------------------------------------------------------
CREATE TABLE producto (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    categoria_id   BIGINT      NOT NULL,
    nombre         TEXT        NOT NULL,
    descripcion    TEXT,
    -- Ruta/clave de la foto: disco del contenedor en dev, Supabase Storage en prod
    foto_path      TEXT,
    activo         BOOLEAN     NOT NULL DEFAULT TRUE,
    creado_en      TIMESTAMPTZ NOT NULL DEFAULT now(),
    actualizado_en TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT producto_categoria_fk
        FOREIGN KEY (categoria_id) REFERENCES categoria (id) ON DELETE RESTRICT,
    CONSTRAINT producto_nombre_por_categoria_unico UNIQUE (categoria_id, nombre),
    CONSTRAINT producto_nombre_no_vacio CHECK (length(trim(nombre)) > 0)
);

CREATE INDEX idx_producto_categoria ON producto (categoria_id);
CREATE INDEX idx_producto_activo    ON producto (activo) WHERE activo;

CREATE TRIGGER trg_producto_actualizado
    BEFORE UPDATE ON producto
    FOR EACH ROW EXECUTE FUNCTION f_set_actualizado_en();

-- -----------------------------------------------------------------------------
-- lote: compra de prendas. El costo unitario NUNCA se guarda: se deriva de
-- (costo_total + costo_envio) / unidades  ->  sin redundancia, sin violar 3FN
-- -----------------------------------------------------------------------------
CREATE TABLE lote (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    producto_id    BIGINT      NOT NULL,
    fecha          DATE        NOT NULL DEFAULT current_date,
    unidades       INTEGER     NOT NULL,
    costo_total    NUMERIC(12, 2) NOT NULL DEFAULT 0,
    costo_envio    NUMERIC(12, 2) NOT NULL DEFAULT 0,
    creado_en      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT lote_producto_fk
        FOREIGN KEY (producto_id) REFERENCES producto (id) ON DELETE RESTRICT,
    CONSTRAINT lote_unidades_positivas  CHECK (unidades > 0),
    CONSTRAINT lote_costo_total_no_neg  CHECK (costo_total >= 0),
    CONSTRAINT lote_costo_envio_no_neg  CHECK (costo_envio >= 0)
);

CREATE INDEX idx_lote_producto_fecha ON lote (producto_id, fecha);

-- Vista derivada: costo por unidad de cada lote (nunca se almacena)
CREATE VIEW v_lote_costo_unitario AS
SELECT l.id,
       l.producto_id,
       l.fecha,
       l.unidades,
       l.costo_total,
       l.costo_envio,
       ROUND((l.costo_total + l.costo_envio) / l.unidades, 2) AS costo_unitario
FROM lote l;

-- -----------------------------------------------------------------------------
-- producto_talla: stock actual por talla. Clave compuesta (producto, talla):
-- una fila por combinación, sin valores repetidos ni columnas "col S/M/L/XL"
-- -----------------------------------------------------------------------------
CREATE TABLE producto_talla (
    producto_id    BIGINT      NOT NULL,
    talla          talla_t     NOT NULL,
    cantidad       INTEGER     NOT NULL DEFAULT 0,
    actualizado_en TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT producto_talla_pk PRIMARY KEY (producto_id, talla),
    CONSTRAINT producto_talla_producto_fk
        FOREIGN KEY (producto_id) REFERENCES producto (id) ON DELETE CASCADE,
    CONSTRAINT producto_talla_cantidad_no_neg CHECK (cantidad >= 0)
);

-- Consulta clave de negocio: "¿qué tallas están en 0?"
CREATE INDEX idx_producto_talla_en_cero
    ON producto_talla (producto_id, talla) WHERE cantidad = 0;

CREATE TRIGGER trg_producto_talla_actualizado
    BEFORE UPDATE ON producto_talla
    FOR EACH ROW EXECUTE FUNCTION f_set_actualizado_en();

-- -----------------------------------------------------------------------------
-- venta: cabecera del registro de venta (sin caja, sin cliente, sin SUNAT)
-- -----------------------------------------------------------------------------
CREATE TABLE venta (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    fecha          TIMESTAMPTZ NOT NULL DEFAULT now(),
    estado         estado_venta_t NOT NULL DEFAULT 'activa',
    notas          TEXT,
    creado_en      TIMESTAMPTZ NOT NULL DEFAULT now(),
    actualizado_en TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_venta_fecha      ON venta (fecha);
CREATE INDEX idx_venta_estado     ON venta (estado) WHERE estado = 'activa';

CREATE TRIGGER trg_venta_actualizado
    BEFORE UPDATE ON venta
    FOR EACH ROW EXECUTE FUNCTION f_set_actualizado_en();

-- -----------------------------------------------------------------------------
-- detalle_venta: unidades vendidas por talla, con el precio de venta del momento
-- (el precio varía venta a venta: se registra en la línea, no en el producto)
-- -----------------------------------------------------------------------------
CREATE TABLE detalle_venta (
    id               BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    venta_id         BIGINT      NOT NULL,
    producto_id      BIGINT      NOT NULL,
    talla            talla_t     NOT NULL,
    cantidad         INTEGER     NOT NULL,
    precio_unitario  NUMERIC(12, 2) NOT NULL,
    CONSTRAINT detalle_venta_venta_fk
        FOREIGN KEY (venta_id) REFERENCES venta (id) ON DELETE CASCADE,
    CONSTRAINT detalle_venta_producto_fk
        FOREIGN KEY (producto_id) REFERENCES producto (id) ON DELETE RESTRICT,
    -- Una sola línea por producto+talla dentro de la misma venta
    CONSTRAINT detalle_venta_linea_unica UNIQUE (venta_id, producto_id, talla),
    CONSTRAINT detalle_venta_cantidad_positiva  CHECK (cantidad > 0),
    CONSTRAINT detalle_venta_precio_no_neg      CHECK (precio_unitario >= 0)
);

CREATE INDEX idx_detalle_venta_producto ON detalle_venta (producto_id, talla);

-- -----------------------------------------------------------------------------
-- Anulación de venta: en una sola transacción se marca la venta y se repone
-- el stock. Lo ejecuta la API (Go); queda documentado aquí como contrato.
--
--   BEGIN;
--     UPDATE venta SET estado = 'anulada' WHERE id = $1 AND estado = 'activa';
--     UPDATE producto_talla pt
--        SET cantidad = pt.cantidad + dv.cantidad
--       FROM detalle_venta dv
--      WHERE dv.venta_id = $1 AND dv.producto_id = pt.producto_id AND dv.talla = pt.talla;
--   COMMIT;
-- -----------------------------------------------------------------------------
