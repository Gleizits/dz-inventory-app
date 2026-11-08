-- Lotes pertenecen a una categoría y pueden recibir varias prendas.
ALTER TABLE lote
    ADD COLUMN categoria_id BIGINT;

UPDATE lote l
   SET categoria_id = p.categoria_id
  FROM producto p
 WHERE p.id = l.producto_id;

ALTER TABLE lote
    ALTER COLUMN categoria_id SET NOT NULL,
    ALTER COLUMN producto_id DROP NOT NULL;

ALTER TABLE lote
    ADD CONSTRAINT lote_categoria_fk
        FOREIGN KEY (categoria_id) REFERENCES categoria (id) ON DELETE RESTRICT;

CREATE INDEX idx_lote_categoria_fecha ON lote (categoria_id, fecha);

ALTER TABLE producto
    ADD COLUMN lote_id BIGINT,
    ADD COLUMN unidades_lote INTEGER;

ALTER TABLE producto
    ADD CONSTRAINT producto_lote_fk
        FOREIGN KEY (lote_id) REFERENCES lote (id) ON DELETE RESTRICT,
    ADD CONSTRAINT producto_unidades_lote_positivas
        CHECK (unidades_lote IS NULL OR unidades_lote > 0);

CREATE INDEX idx_producto_lote ON producto (lote_id);
