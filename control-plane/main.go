// Watchclaw RMM control plane — telemetry ingestion + endpoint query API.
//
// F0: receive endpoint reports from the Rust agent, persist them, expose a
// list endpoint for the dashboard. Storage backend is pluggable (SQLite for
// dev, TimescaleDB in production).
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

	"github.com/fullstackit/watchclaw/control-plane/internal/api"
	"github.com/fullstackit/watchclaw/control-plane/internal/store"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	addr := getenv("WATCHCLAW_ADDR", ":8080")
	dbPath := getenv("WATCHCLAW_DB", "watchclaw.db")

	st, err := store.OpenSQLite(dbPath)
	if err != nil {
		logger.Error("open store failed", "error", err)
		os.Exit(1)
	}
	defer st.Close()

	srv := &http.Server{
		Addr:              addr,
		Handler:           api.New(st, logger),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("control plane listening", "addr", addr, "db", dbPath)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	logger.Info("shutting down")
	_ = srv.Shutdown(ctx)
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
