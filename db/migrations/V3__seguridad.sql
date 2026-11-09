-- =============================================================================
-- DZ Urban Style — endurecimiento de seguridad para Supabase
-- La API Go usa la conexión PostgreSQL del servidor; Android no accede
-- directamente a las tablas de Supabase.
-- =============================================================================

-- Bloquea el acceso directo de roles no privilegiados mediante PostgREST.
-- La conexión administrativa usada por la API y Flyway conserva el acceso.
ALTER TABLE public.categoria ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.producto ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.lote ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.producto_talla ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.venta ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.detalle_venta ENABLE ROW LEVEL SECURITY;

-- La vista respeta las políticas RLS de las tablas subyacentes.
ALTER VIEW public.v_lote_costo_unitario SET (security_invoker = true);

-- Evita que una función de trigger dependa del search_path de la sesión.
ALTER FUNCTION public.f_set_actualizado_en()
    SET search_path = pg_catalog, public;
