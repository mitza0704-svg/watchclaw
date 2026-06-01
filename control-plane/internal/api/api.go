// Package api exposes the control-plane HTTP surface.
// Uses Go 1.22+ method-aware routing in net/http (no external router needed yet).
package api

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"html"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/fullstackit/watchclaw/control-plane/internal/model"
	"github.com/fullstackit/watchclaw/control-plane/internal/store"
	"github.com/fullstackit/watchclaw/control-plane/internal/topology"
)

//go:embed landing.html
var landingHTML []byte

//go:embed dashboard.html
var dashboardHTML []byte

//go:embed cytoscape.min.js
var cytoscapeJS []byte

type Handler struct {
	store  store.Store
	logger *slog.Logger
}

func New(s store.Store, logger *slog.Logger) http.Handler {
	h := &Handler{store: s, logger: logger}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", h.landing)
	mux.HandleFunc("GET /app", h.dashboard)
	mux.HandleFunc("GET /cytoscape.min.js", h.cytoscape)
	mux.HandleFunc("GET /progress", h.progress)
	mux.HandleFunc("GET /healthz", h.health)
	mux.HandleFunc("POST /v1/telemetry", h.postTelemetry)
	mux.HandleFunc("GET /v1/endpoints", h.listEndpoints)
	mux.HandleFunc("POST /v1/discovery", h.postDiscovery)
	mux.HandleFunc("GET /v1/topology", h.getTopology)
	return mux
}

func (h *Handler) landing(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(landingHTML)
}

func (h *Handler) dashboard(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(dashboardHTML)
}

func (h *Handler) cytoscape(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	_, _ = w.Write(cytoscapeJS)
}

// progress serves the LIVE-PROGRESS.md file as a phone-readable HTML page.
func (h *Handler) progress(w http.ResponseWriter, _ *http.Request) {
	path := os.Getenv("WATCHCLAW_PROGRESS")
	if path == "" {
		path = "../LIVE-PROGRESS.md"
	}
	body := "progress file not available yet"
	if data, err := os.ReadFile(path); err == nil {
		body = string(data)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, progressHTML, html.EscapeString(body))
}

const progressHTML = `<!doctype html><html lang="en"><head>
<meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<meta http-equiv="refresh" content="15"><meta name="theme-color" content="#0a0c10">
<title>Watchclaw — Progress</title>
<style>body{margin:0;background:#0a0c10;color:#e6eaf0;font:13px/1.55 ui-monospace,Menlo,Consolas,monospace;padding:16px}
h1{font-size:14px;color:#3dd68c;margin:0 0 4px}.sub{color:#7a8596;font-size:12px;margin:0 0 14px}
a{color:#5b8cff;text-decoration:none}pre{white-space:pre-wrap;word-wrap:break-word;margin:0}</style>
</head><body><h1>🔴 Watchclaw — Live Progress</h1>
<p class="sub"><a href="/">← Topology</a> · auto-refresh 15s</p><pre>%s</pre></body></html>`

func (h *Handler) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) postTelemetry(w http.ResponseWriter, r *http.Request) {
	var report model.EndpointReport
	// Tolerant decode (no DisallowUnknownFields): the agent evolves and may add
	// fields the server doesn't map yet — forward compatibility over strictness.
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<20)) // 4 MiB (inventory)
	if err := dec.Decode(&report); err != nil {
		h.logger.Warn("telemetry decode failed", "error", err)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid telemetry payload"})
		return
	}
	if report.Hostname == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "hostname is required"})
		return
	}
	if report.CollectedAt == "" {
		report.CollectedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if err := h.store.SaveReport(r.Context(), report); err != nil {
		h.logger.Error("save report failed", "host", report.Hostname, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not persist report"})
		return
	}
	h.logger.Info("telemetry stored", "host", report.Hostname, "cpu", report.CPUUsagePct, "mem", report.MemUsagePct)
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted", "host": report.Hostname})
}

func (h *Handler) listEndpoints(w http.ResponseWriter, r *http.Request) {
	endpoints, err := h.store.ListEndpoints(r.Context())
	if err != nil {
		h.logger.Error("list endpoints failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not list endpoints"})
		return
	}
	if endpoints == nil {
		endpoints = []store.EndpointSummary{}
	}
	writeJSON(w, http.StatusOK, endpoints)
}

func (h *Handler) postDiscovery(w http.ResponseWriter, r *http.Request) {
	var scan model.NetworkScan
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<20)) // 4 MiB cap
	if err := dec.Decode(&scan); err != nil {
		h.logger.Warn("discovery decode failed", "error", err)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid scan payload"})
		return
	}
	if err := h.store.SaveScan(r.Context(), scan); err != nil {
		h.logger.Error("save scan failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not persist scan"})
		return
	}
	h.logger.Info("scan stored", "subnet", scan.Subnet, "devices", len(scan.Devices), "reporter", scan.Reporter)
	writeJSON(w, http.StatusAccepted, map[string]any{"status": "accepted", "devices": len(scan.Devices)})
}

func (h *Handler) getTopology(w http.ResponseWriter, r *http.Request) {
	scan, err := h.store.LatestScan(r.Context())
	if err != nil {
		h.logger.Error("latest scan failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not load scan"})
		return
	}
	if scan == nil {
		writeJSON(w, http.StatusOK, model.TopologyGraph{Nodes: []model.TopoNode{}, Edges: []model.TopoEdge{}})
		return
	}
	writeJSON(w, http.StatusOK, topology.Build(scan))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
