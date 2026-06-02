// Package api exposes the control-plane HTTP surface.
// Uses Go 1.22+ method-aware routing in net/http (no external router needed yet).
package api

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"html"
	"log/slog"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"time"

	"github.com/fullstackit/watchclaw/control-plane/internal/agentless"
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
	mux.HandleFunc("GET /v1/endpoint/{host}", h.getEndpoint)
	mux.HandleFunc("GET /v1/alerts", h.listAlerts)
	mux.HandleFunc("POST /v1/discovery", h.postDiscovery)
	mux.HandleFunc("GET /v1/topology", h.getTopology)
	mux.HandleFunc("POST /v1/agentless/scan", h.postAgentlessScan)
	mux.HandleFunc("POST /v1/scripts/run", h.postScriptRun)
	mux.HandleFunc("GET /v1/scripts/runs", h.listScriptRuns)
	mux.HandleFunc("POST /v1/jobs", h.postJob)
	mux.HandleFunc("GET /v1/jobs", h.listJobs)
	mux.HandleFunc("POST /v1/patch/apply", h.postPatchApply)
	mux.HandleFunc("GET /v1/agent/jobs/{hostname}", h.claimJobs)
	mux.HandleFunc("POST /v1/agent/jobs/{id}/result", h.jobResult)
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

func (h *Handler) getEndpoint(w http.ResponseWriter, r *http.Request) {
	host := r.PathValue("host")
	payload, err := h.store.LatestReport(r.Context(), host)
	if err != nil {
		h.logger.Error("latest report failed", "host", host, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not load endpoint"})
		return
	}
	if payload == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "endpoint not found"})
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write(payload)
}

func (h *Handler) listAlerts(w http.ResponseWriter, r *http.Request) {
	alerts, err := h.store.ListAlerts(r.Context())
	if err != nil {
		h.logger.Error("list alerts failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not list alerts"})
		return
	}
	if alerts == nil {
		alerts = []store.Alert{}
	}
	writeJSON(w, http.StatusOK, alerts)
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

// postAgentlessScan runs a credentialed WinRM inventory of a remote Windows
// host (no agent installed) and stores it as a normal endpoint report, so it
// shows up in the dashboard like any agent-reported machine.
// NOTE: credentials are passed in the request body for now; move to Vault +
// per-site scoping before exposing this beyond the trusted network.
func (h *Handler) postAgentlessScan(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Host     string `json:"host"`
		Port     int    `json:"port"`
		HTTPS    bool   `json:"https"`
		Insecure bool   `json:"insecure"`
		User     string `json:"user"`
		Pass     string `json:"pass"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	if err := dec.Decode(&req); err != nil || req.Host == "" || req.User == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "host, user, pass required"})
		return
	}
	if req.Port == 0 {
		req.Port = 5985
	}
	ctx, cancel := context.WithTimeout(r.Context(), 4*time.Minute)
	defer cancel()
	report, err := agentless.ScanWindows(ctx, req.Host, req.Port, req.HTTPS, req.Insecure, req.User, req.Pass)
	if err != nil {
		h.logger.Warn("agentless scan failed", "host", req.Host, "error", err)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	if err := h.store.SaveReport(ctx, report); err != nil {
		h.logger.Error("agentless save failed", "host", req.Host, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not persist report"})
		return
	}
	h.logger.Info("agentless scan stored", "host", req.Host, "hostname", report.Hostname)
	writeJSON(w, http.StatusAccepted, map[string]any{"status": "accepted", "hostname": report.Hostname, "source": "agentless-winrm"})
}

// postScriptRun executes a remote PowerShell/cmd command on a Windows host over
// WinRM and records the result in the audit trail (script_runs) — ALWAYS, even
// when dispatch fails. This is the core RMM remote-action primitive.
// SECURITY: same posture as agentless scan — credentials in the request body,
// trusted-network only. Move to Vault + per-site scoping + operator auth before
// exposing beyond the LAN/Tailscale.
func (h *Handler) postScriptRun(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Host     string `json:"host"`
		Port     int    `json:"port"`
		HTTPS    bool   `json:"https"`
		Insecure bool   `json:"insecure"`
		User     string `json:"user"`
		Pass     string `json:"pass"`
		Shell    string `json:"shell"`  // powershell (default) | cmd
		Script   string `json:"script"` // command to execute
		RanBy    string `json:"ran_by"` // operator label (optional, audited)
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	if err := dec.Decode(&req); err != nil || req.Host == "" || req.User == "" || req.Script == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "host, user, pass, script required"})
		return
	}
	if req.Shell == "" {
		req.Shell = "powershell"
	}
	ctx, cancel := context.WithTimeout(r.Context(), 4*time.Minute)
	defer cancel()

	conn := agentless.Conn{Host: req.Host, Port: req.Port, HTTPS: req.HTTPS, Insecure: req.Insecure, User: req.User, Pass: req.Pass}
	res, runErr := agentless.RunScript(ctx, conn, req.Shell, req.Script)

	// Audit FIRST — record the attempt regardless of outcome.
	audit := store.ScriptRun{
		Hostname:   req.Host,
		Shell:      req.Shell,
		Script:     req.Script,
		ExitCode:   res.ExitCode,
		Stdout:     res.Stdout,
		Stderr:     res.Stderr,
		DurationMs: res.DurationMs,
		Status:     "ok",
		RanBy:      req.RanBy,
	}
	if runErr != nil {
		audit.Status = "failed"
		audit.Error = runErr.Error()
	}
	id, saveErr := h.store.SaveScriptRun(ctx, audit)
	if saveErr != nil {
		h.logger.Error("script audit save failed", "host", req.Host, "error", saveErr)
	}

	if runErr != nil {
		h.logger.Warn("script run failed", "host", req.Host, "shell", req.Shell, "error", runErr)
		writeJSON(w, http.StatusBadGateway, map[string]any{"run_id": id, "status": "failed", "error": runErr.Error()})
		return
	}
	h.logger.Info("script run", "host", req.Host, "shell", req.Shell, "exit", res.ExitCode, "ms", res.DurationMs, "by", req.RanBy)
	writeJSON(w, http.StatusOK, map[string]any{
		"run_id": id, "status": "ok", "exit_code": res.ExitCode,
		"stdout": res.Stdout, "stderr": res.Stderr,
		"duration_ms": res.DurationMs, "truncated": res.Truncated,
	})
}

// listScriptRuns returns the remote-execution audit trail, newest first.
func (h *Handler) listScriptRuns(w http.ResponseWriter, r *http.Request) {
	runs, err := h.store.ListScriptRuns(r.Context(), 100)
	if err != nil {
		h.logger.Error("list script runs failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not list script runs"})
		return
	}
	if runs == nil {
		runs = []store.ScriptRun{}
	}
	writeJSON(w, http.StatusOK, runs)
}

// postJob enqueues a command-channel job for an agent to execute in its own
// machine context. kind "patch" carries a winget package id in command; "script"
// carries a raw PS/cmd/sh command.
func (h *Handler) postJob(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Hostname string `json:"hostname"`
		Kind     string `json:"kind"`
		Shell    string `json:"shell"`
		Command  string `json:"command"`
		CreatedBy string `json:"created_by"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	if err := dec.Decode(&req); err != nil || req.Hostname == "" || req.Command == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "hostname and command required"})
		return
	}
	if req.Kind == "" {
		req.Kind = "script"
	}
	if req.Shell == "" {
		req.Shell = "powershell"
	}
	id, err := h.store.EnqueueJob(r.Context(), store.Job{
		Hostname: req.Hostname, Kind: req.Kind, Shell: req.Shell,
		Command: req.Command, CreatedBy: req.CreatedBy,
	})
	if err != nil {
		h.logger.Error("enqueue job failed", "host", req.Hostname, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not enqueue job"})
		return
	}
	h.logger.Info("job enqueued", "id", id, "host", req.Hostname, "kind", req.Kind, "by", req.CreatedBy)
	writeJSON(w, http.StatusAccepted, map[string]any{"id": id, "status": "pending"})
}

// claimJobs is polled by an agent: it atomically claims this host's pending jobs
// (pending -> running) and returns them to execute.
func (h *Handler) claimJobs(w http.ResponseWriter, r *http.Request) {
	host := r.PathValue("hostname")
	jobs, err := h.store.ClaimJobs(r.Context(), host)
	if err != nil {
		h.logger.Error("claim jobs failed", "host", host, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not claim jobs"})
		return
	}
	if jobs == nil {
		jobs = []store.Job{}
	}
	writeJSON(w, http.StatusOK, jobs)
}

// jobResult is posted by an agent after it finishes a job.
func (h *Handler) jobResult(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid job id"})
		return
	}
	var res store.JobResult
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<20))
	if err := dec.Decode(&res); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid result payload"})
		return
	}
	if err := h.store.CompleteJob(r.Context(), id, res); err != nil {
		h.logger.Warn("complete job failed", "id", id, "error", err)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	h.logger.Info("job completed", "id", id, "status", res.Status, "exit", res.ExitCode)
	writeJSON(w, http.StatusOK, map[string]string{"status": "recorded"})
}

// wingetIDPattern restricts patch package ids to winget's id charset, so the
// id can be interpolated into the agent command with no injection risk.
var wingetIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.\-+_]*$`)

// postPatchApply enqueues a winget upgrade as a command-channel job. The agent
// runs it in its machine/user context — the path that actually works for winget
// (agentless WinRM cannot, by platform design). package_id "all" upgrades every
// pending package; otherwise it must be a valid winget id.
func (h *Handler) postPatchApply(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Hostname  string `json:"hostname"`
		PackageID string `json:"package_id"`
		CreatedBy string `json:"created_by"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	if err := dec.Decode(&req); err != nil || req.Hostname == "" || req.PackageID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "hostname and package_id required"})
		return
	}
	flags := "--silent --accept-source-agreements --accept-package-agreements --disable-interactivity"
	var cmd string
	if req.PackageID == "all" {
		cmd = "winget upgrade --all " + flags
	} else if wingetIDPattern.MatchString(req.PackageID) {
		cmd = "winget upgrade --id " + req.PackageID + " --exact " + flags
	} else {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid package_id"})
		return
	}
	id, err := h.store.EnqueueJob(r.Context(), store.Job{
		Hostname: req.Hostname, Kind: "patch", Shell: "powershell",
		Command: cmd, CreatedBy: req.CreatedBy,
	})
	if err != nil {
		h.logger.Error("enqueue patch failed", "host", req.Hostname, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not enqueue patch"})
		return
	}
	h.logger.Info("patch enqueued", "id", id, "host", req.Hostname, "pkg", req.PackageID, "by", req.CreatedBy)
	writeJSON(w, http.StatusAccepted, map[string]any{"id": id, "status": "pending", "package": req.PackageID})
}

// listJobs returns the command-channel job history (audit) for the dashboard.
func (h *Handler) listJobs(w http.ResponseWriter, r *http.Request) {
	jobs, err := h.store.ListJobs(r.Context(), 100)
	if err != nil {
		h.logger.Error("list jobs failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not list jobs"})
		return
	}
	if jobs == nil {
		jobs = []store.Job{}
	}
	writeJSON(w, http.StatusOK, jobs)
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
