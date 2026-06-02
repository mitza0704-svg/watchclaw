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

	// Monitor loop: offline endpoints are detected by absence of telemetry, so
	// a periodic check (not report-time) raises/resolves the "offline" alert.
	monCtx, monCancel := context.WithCancel(context.Background())
	defer monCancel()
	offlineAfter := getdur("WATCHCLAW_OFFLINE_AFTER", 3*time.Minute)
	monInterval := getdur("WATCHCLAW_MONITOR_INTERVAL", 30*time.Second)
	go runMonitor(monCtx, st, logger, monInterval, offlineAfter)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	logger.Info("shutting down")
	_ = srv.Shutdown(ctx)
}

// runMonitor periodically evaluates offline endpoints until the context is done.
func runMonitor(ctx context.Context, st store.Store, logger *slog.Logger, interval, offlineAfter time.Duration) {
	logger.Info("monitor started", "interval", interval, "offline_after", offlineAfter)
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := st.EvaluateOffline(ctx, offlineAfter); err != nil {
				logger.Warn("offline evaluation failed", "error", err)
			}
		}
	}
}

// getdur reads a Go duration string from env (e.g. "3m", "30s"), else fallback.
func getdur(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
