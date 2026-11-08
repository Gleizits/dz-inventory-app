// API de inventario de DZ Urban Style.
//
//	go run ./api
//
// Lee infra/.env (PostgreSQL y Redis de Podman) y sirve la API en :8080.
// Desde el celular conectado por USB: adb reverse tcp:8080 tcp:8080
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Gleizits/dz-inventory-app/api/internal/cache"
	"github.com/Gleizits/dz-inventory-app/api/internal/config"
	"github.com/Gleizits/dz-inventory-app/api/internal/httpapi"
	"github.com/Gleizits/dz-inventory-app/api/internal/store"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg, err := config.Load()
	if err != nil {
		log.Error("configuración inválida", "error", err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	st, err := store.New(ctx, cfg.DBURL)
	if err != nil {
		log.Error("no se pudo conectar a PostgreSQL", "error", err,
			"pista", "levanta la base con: pwsh -File infra/dev.ps1 up && pwsh -File infra/dev.ps1 migrate")
		os.Exit(1)
	}
	defer st.Cerrar()

	ca := cache.New(cfg.RedisURL, cfg.CacheTTL)
	if !ca.Disponible() {
		log.Warn("Redis no disponible: la API arranca sin caché")
	}
	defer ca.Cerrar()

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           httpapi.New(st, ca, cfg, log),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		log.Info("API escuchando", "addr", cfg.Addr, "fotos", cfg.FotosDir)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("el servidor falló", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	log.Info("apagando la API...")
	shutdownCtx, cancel2 := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel2()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("no se pudo cerrar limpio", "error", err)
	}
}
