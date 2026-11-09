// Package storage integra el almacenamiento de objetos de Supabase.
package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type Supabase struct {
	baseURL string
	apiKey  string
	bucket  string
	client  *http.Client
}

func NewSupabase(baseURL, apiKey, bucket string) *Supabase {
	if baseURL == "" || apiKey == "" || bucket == "" {
		return nil
	}
	return &Supabase{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		bucket:  bucket,
		client:  http.DefaultClient,
	}
}

func (s *Supabase) path(nombre string) string {
	return s.baseURL + "/storage/v1/object/" + url.PathEscape(s.bucket) + "/" + url.PathEscape(nombre)
}

func (s *Supabase) Subir(ctx context.Context, nombre string, contenido []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.path(nombre), bytes.NewReader(contenido))
	if err != nil {
		return fmt.Errorf("crear solicitud de subida: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("apikey", s.apiKey)
	req.Header.Set("Content-Type", "image/jpeg")
	req.Header.Set("x-upsert", "true")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("subir foto a Supabase Storage: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		detalle, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return fmt.Errorf("subir foto a Supabase Storage: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(detalle)))
	}
	return nil
}

func (s *Supabase) Descargar(ctx context.Context, nombre string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.path(nombre), nil)
	if err != nil {
		return nil, fmt.Errorf("crear solicitud de descarga: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("apikey", s.apiKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("descargar foto de Supabase Storage: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("foto no encontrada")
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		detalle, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return nil, fmt.Errorf("descargar foto de Supabase Storage: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(detalle)))
	}
	contenido, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("leer foto de Supabase Storage: %w", err)
	}
	return contenido, nil
}
