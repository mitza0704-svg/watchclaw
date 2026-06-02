// Package store persists endpoint telemetry.
//
// The Store interface lets the control plane swap backends without touching the
// API layer: SQLite (pure-Go) for local dev/tests, TimescaleDB in production.
// F0 ships the SQLite backend; the TimescaleDB implementation lands when we
// stand up the infra/ stack.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/fullstackit/watchclaw/control-plane/internal/model"
	_ "modernc.org/sqlite"
)

// EndpointSummary is the latest-known state of one endpoint, for dashboards.
type EndpointSummary struct {
	Hostname    string  `json:"hostname"`
	OS          string  `json:"os"`
	OSVersion   string  `json:"os_version"`
	CPUUsagePct float64 `json:"cpu_usage_pct"`
	MemUsagePct float64 `json:"mem_usage_pct"`
	DiskUsagePct float64 `json:"disk_usage_pct"`
	// Health is derived from thresholds: ok | warning | critical.
	Health   string `json:"health"`
	LastSeen string `json:"last_seen"`
	ReportCount int `json:"report_count"`
}

// health derives an alert level from resource thresholds.
func health(cpu, mem, disk float64) string {
	switch {
	case disk >= 95 || cpu >= 97 || mem >= 97:
		return "critical"
	case disk >= 85 || cpu >= 90 || mem >= 90:
		return "warning"
	default:
		return "ok"
	}
}

// metricLevel returns "critical" | "warning" | "" for one resource metric.
// Per-metric thresholds mirror health() but let us attribute an alert to a
// specific resource (cpu/mem/disk) instead of a blended endpoint health.
func metricLevel(value, warn, crit float64) string {
	switch {
	case value >= crit:
		return "critical"
	case value >= warn:
		return "warning"
	default:
		return ""
	}
}

// Alert is an open or resolved condition on an endpoint. The control plane is
// the source of tickets: an alert with status "open" is an actionable ticket.
type Alert struct {
	ID        int64   `json:"id"`
	Hostname  string  `json:"hostname"`
	Kind      string  `json:"kind"`     // cpu | mem | disk | smart | offline
	Severity  string  `json:"severity"` // warning | critical
	Message   string  `json:"message"`
	Value     float64 `json:"value"`
	Status    string  `json:"status"` // open | resolved
	Count     int     `json:"count"`  // how many consecutive reports kept it open
	FirstSeen string  `json:"first_seen"`
	LastSeen  string  `json:"last_seen"`
}

// ScriptRun is one audited remote command execution. Every remote script run
// is recorded here (success or failure) — the audit trail is non-negotiable for
// an RMM that runs arbitrary commands on customer endpoints.
type ScriptRun struct {
	ID         int64  `json:"id"`
	Hostname   string `json:"hostname"`   // target host as addressed (ip/name)
	Shell      string `json:"shell"`      // powershell | cmd
	Script     string `json:"script"`     // exact command executed
	ExitCode   int    `json:"exit_code"`  // remote process exit code
	Stdout     string `json:"stdout"`     // captured (bounded) output
	Stderr     string `json:"stderr"`     // captured (bounded) error stream
	DurationMs int64  `json:"duration_ms"`
	Status     string `json:"status"` // ok | failed (transport/dispatch error)
	Error      string `json:"error,omitempty"`
	RanBy      string `json:"ran_by"` // operator/actor (best-effort, from request)
	RanAt      string `json:"ran_at"`
}

type Store interface {
	SaveReport(ctx context.Context, r model.EndpointReport) error
	ListEndpoints(ctx context.Context) ([]EndpointSummary, error)
	SaveScan(ctx context.Context, s model.NetworkScan) error
	LatestScan(ctx context.Context) (*model.NetworkScan, error)
	ListAlerts(ctx context.Context) ([]Alert, error)
	EvaluateOffline(ctx context.Context, threshold time.Duration) error
	LatestReport(ctx context.Context, hostname string) (json.RawMessage, error)
	SaveScriptRun(ctx context.Context, run ScriptRun) (int64, error)
	ListScriptRuns(ctx context.Context, limit int) ([]ScriptRun, error)
	Close() error
}

type SQLiteStore struct{ db *sql.DB }

func OpenSQLite(path string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	s := &SQLiteStore{db: db}
	if err := s.init(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *SQLiteStore) init() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS telemetry (
	id            INTEGER PRIMARY KEY AUTOINCREMENT,
	hostname      TEXT NOT NULL,
	os            TEXT,
	os_version    TEXT,
	cpu_usage_pct REAL,
	mem_usage_pct REAL,
	disk_usage_pct REAL,
	payload       TEXT NOT NULL,
	collected_at  TEXT NOT NULL,
	received_at   TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_telemetry_host ON telemetry(hostname, id);

CREATE TABLE IF NOT EXISTS scans (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	reporter    TEXT,
	subnet      TEXT,
	device_count INTEGER,
	payload     TEXT NOT NULL,
	received_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS alerts (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	hostname   TEXT NOT NULL,
	kind       TEXT NOT NULL,
	severity   TEXT NOT NULL,
	message    TEXT NOT NULL,
	value      REAL,
	status     TEXT NOT NULL DEFAULT 'open',
	count      INTEGER NOT NULL DEFAULT 1,
	first_seen TEXT NOT NULL,
	last_seen  TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_alerts_open ON alerts(status, hostname, kind);

CREATE TABLE IF NOT EXISTS script_runs (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	hostname    TEXT NOT NULL,
	shell       TEXT NOT NULL,
	script      TEXT NOT NULL,
	exit_code   INTEGER NOT NULL DEFAULT 0,
	stdout      TEXT,
	stderr      TEXT,
	duration_ms INTEGER NOT NULL DEFAULT 0,
	status      TEXT NOT NULL,
	error       TEXT,
	ran_by      TEXT,
	ran_at      TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_script_runs_host ON script_runs(hostname, id);
`)
	if err != nil {
		return fmt.Errorf("init schema: %w", err)
	}
	// Migrations for DBs created before a column existed. CREATE TABLE IF NOT
	// EXISTS never alters an existing table, so a persistent volume from an
	// older build keeps the old shape (this is what broke telemetry inserts on
	// zmfbot: "table telemetry has no column named disk_usage_pct").
	s.ensureColumn("telemetry", "disk_usage_pct", "REAL")
	return nil
}

// ensureColumn adds a column if the table lacks it. Idempotent and best-effort:
// inspects PRAGMA table_info and only ALTERs when missing.
func (s *SQLiteStore) ensureColumn(table, column, typ string) {
	rows, err := s.db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notnull, pk int
		var name, ctype string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return
		}
		if name == column {
			return // already present
		}
	}
	_, _ = s.db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, typ))
}

func (s *SQLiteStore) SaveScan(ctx context.Context, scan model.NetworkScan) error {
	payload, err := json.Marshal(scan)
	if err != nil {
		return fmt.Errorf("marshal scan: %w", err)
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO scans(reporter, subnet, device_count, payload, received_at) VALUES(?, ?, ?, ?, ?)`,
		scan.Reporter, scan.Subnet, len(scan.Devices), string(payload),
		time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("insert scan: %w", err)
	}
	return nil
}

func (s *SQLiteStore) LatestScan(ctx context.Context) (*model.NetworkScan, error) {
	var payload string
	err := s.db.QueryRowContext(ctx,
		`SELECT payload FROM scans ORDER BY id DESC LIMIT 1`).Scan(&payload)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query latest scan: %w", err)
	}
	var scan model.NetworkScan
	if err := json.Unmarshal([]byte(payload), &scan); err != nil {
		return nil, fmt.Errorf("unmarshal scan: %w", err)
	}
	return &scan, nil
}

func (s *SQLiteStore) SaveReport(ctx context.Context, r model.EndpointReport) error {
	payload, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("marshal report: %w", err)
	}
	var maxDisk float64
	for _, d := range r.Disks {
		if d.UsagePct > maxDisk {
			maxDisk = d.UsagePct
		}
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO telemetry(hostname, os, os_version, cpu_usage_pct, mem_usage_pct, disk_usage_pct, payload, collected_at, received_at)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.Hostname, r.OS, r.OSVersion, r.CPUUsagePct, r.MemUsagePct, maxDisk,
		string(payload), r.CollectedAt, time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("insert telemetry: %w", err)
	}
	// Alerting is best-effort: a failure here must not lose the telemetry that
	// was already persisted. The API logs nothing from the store, so we swallow.
	_ = s.evaluateAlerts(ctx, r.Hostname, r.CPUUsagePct, r.MemUsagePct, maxDisk)
	return nil
}

// evaluateAlerts raises or resolves one alert per resource metric for an
// endpoint, based on the report just stored. An alert that stays open across
// reports keeps its first_seen and increments count (flap-resistant).
func (s *SQLiteStore) evaluateAlerts(ctx context.Context, hostname string, cpu, mem, disk float64) error {
	now := time.Now().UTC().Format(time.RFC3339)
	check := func(kind, label string, value, warn, crit float64) error {
		lvl := metricLevel(value, warn, crit)
		if lvl == "" {
			return s.resolveAlert(ctx, hostname, kind, now)
		}
		msg := fmt.Sprintf("%s at %.1f%%", label, value)
		return s.raiseAlert(ctx, hostname, kind, lvl, msg, value, now)
	}
	if err := check("cpu", "CPU usage", cpu, 90, 97); err != nil {
		return err
	}
	if err := check("mem", "Memory usage", mem, 90, 97); err != nil {
		return err
	}
	return check("disk", "Disk usage", disk, 85, 95)
}

// raiseAlert upserts an open alert keyed by (hostname, kind). A new condition
// inserts; an existing open one updates severity/value and bumps count.
func (s *SQLiteStore) raiseAlert(ctx context.Context, hostname, kind, severity, message string, value float64, now string) error {
	var id int64
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT id, count FROM alerts WHERE hostname=? AND kind=? AND status='open'`,
		hostname, kind).Scan(&id, &count)
	if err == sql.ErrNoRows {
		_, err = s.db.ExecContext(ctx,
			`INSERT INTO alerts(hostname, kind, severity, message, value, status, count, first_seen, last_seen)
			 VALUES(?, ?, ?, ?, ?, 'open', 1, ?, ?)`,
			hostname, kind, severity, message, value, now, now)
		return err
	}
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx,
		`UPDATE alerts SET severity=?, message=?, value=?, count=?, last_seen=? WHERE id=?`,
		severity, message, value, count+1, now, id)
	return err
}

// resolveAlert closes any open alert for (hostname, kind) once the metric is
// back under threshold.
func (s *SQLiteStore) resolveAlert(ctx context.Context, hostname, kind, now string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE alerts SET status='resolved', last_seen=? WHERE hostname=? AND kind=? AND status='open'`,
		now, hostname, kind)
	return err
}

// ListAlerts returns open alerts (the actionable tickets), critical first.
func (s *SQLiteStore) ListAlerts(ctx context.Context) ([]Alert, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, hostname, kind, severity, message, value, status, count, first_seen, last_seen
FROM alerts
WHERE status='open'
ORDER BY CASE severity WHEN 'critical' THEN 0 ELSE 1 END, last_seen DESC`)
	if err != nil {
		return nil, fmt.Errorf("query alerts: %w", err)
	}
	defer rows.Close()

	var out []Alert
	for rows.Next() {
		var a Alert
		if err := rows.Scan(&a.ID, &a.Hostname, &a.Kind, &a.Severity, &a.Message,
			&a.Value, &a.Status, &a.Count, &a.FirstSeen, &a.LastSeen); err != nil {
			return nil, fmt.Errorf("scan alert: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// EvaluateOffline raises an "offline" alert for any endpoint whose most recent
// telemetry is older than threshold, and resolves it once the endpoint reports
// again. Called periodically by the control plane's monitor loop — offline is
// detected by ABSENCE, so it can't be evaluated on report ingest like the
// resource alerts.
func (s *SQLiteStore) EvaluateOffline(ctx context.Context, threshold time.Duration) error {
	rows, err := s.db.QueryContext(ctx,
		`SELECT hostname, MAX(received_at) FROM telemetry GROUP BY hostname`)
	if err != nil {
		return fmt.Errorf("query last-seen: %w", err)
	}
	type seen struct{ host, last string }
	var all []seen
	for rows.Next() {
		var sv seen
		if err := rows.Scan(&sv.host, &sv.last); err != nil {
			rows.Close()
			return fmt.Errorf("scan last-seen: %w", err)
		}
		all = append(all, sv)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339)
	for _, sv := range all {
		last, err := time.Parse(time.RFC3339, sv.last)
		if err != nil {
			continue // skip unparseable timestamps rather than false-alarm
		}
		gap := now.Sub(last)
		if gap > threshold {
			mins := gap.Minutes()
			msg := fmt.Sprintf("No telemetry for %.0f min (last seen %s)", mins, sv.last)
			if e := s.raiseAlert(ctx, sv.host, "offline", "critical", msg, mins, nowStr); e != nil {
				return e
			}
		} else {
			if e := s.resolveAlert(ctx, sv.host, "offline", nowStr); e != nil {
				return e
			}
		}
	}
	return nil
}

// ListEndpoints returns the latest report per hostname plus a total report count.
func (s *SQLiteStore) ListEndpoints(ctx context.Context) ([]EndpointSummary, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT t.hostname, t.os, t.os_version, t.cpu_usage_pct, t.mem_usage_pct, t.disk_usage_pct, t.received_at, c.cnt
FROM telemetry t
JOIN (
	SELECT hostname, MAX(id) AS max_id, COUNT(*) AS cnt
	FROM telemetry GROUP BY hostname
) c ON t.hostname = c.hostname AND t.id = c.max_id
ORDER BY t.hostname`)
	if err != nil {
		return nil, fmt.Errorf("query endpoints: %w", err)
	}
	defer rows.Close()

	var out []EndpointSummary
	for rows.Next() {
		var e EndpointSummary
		var disk sql.NullFloat64
		if err := rows.Scan(&e.Hostname, &e.OS, &e.OSVersion, &e.CPUUsagePct, &e.MemUsagePct, &disk, &e.LastSeen, &e.ReportCount); err != nil {
			return nil, fmt.Errorf("scan endpoint: %w", err)
		}
		e.DiskUsagePct = disk.Float64
		e.Health = health(e.CPUUsagePct, e.MemUsagePct, e.DiskUsagePct)
		out = append(out, e)
	}
	return out, rows.Err()
}

// LatestReport returns the most recent raw telemetry payload for a host — the
// full agent report (metrics + deep hardware/services/connections inventory).
func (s *SQLiteStore) LatestReport(ctx context.Context, hostname string) (json.RawMessage, error) {
	var payload string
	err := s.db.QueryRowContext(ctx,
		`SELECT payload FROM telemetry WHERE hostname=? ORDER BY id DESC LIMIT 1`, hostname).Scan(&payload)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query latest report: %w", err)
	}
	return json.RawMessage(payload), nil
}

// SaveScriptRun persists one audited remote execution and returns its row id.
func (s *SQLiteStore) SaveScriptRun(ctx context.Context, run ScriptRun) (int64, error) {
	if run.RanAt == "" {
		run.RanAt = time.Now().UTC().Format(time.RFC3339)
	}
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO script_runs(hostname, shell, script, exit_code, stdout, stderr, duration_ms, status, error, ran_by, ran_at)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		run.Hostname, run.Shell, run.Script, run.ExitCode, run.Stdout, run.Stderr,
		run.DurationMs, run.Status, run.Error, run.RanBy, run.RanAt)
	if err != nil {
		return 0, fmt.Errorf("insert script_run: %w", err)
	}
	return res.LastInsertId()
}

// ListScriptRuns returns the most recent audited executions, newest first.
func (s *SQLiteStore) ListScriptRuns(ctx context.Context, limit int) ([]ScriptRun, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT id, hostname, shell, script, exit_code, stdout, stderr, duration_ms, status, error, ran_by, ran_at
FROM script_runs ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("query script_runs: %w", err)
	}
	defer rows.Close()

	var out []ScriptRun
	for rows.Next() {
		var r ScriptRun
		var stdout, stderr, errStr, ranBy sql.NullString
		if err := rows.Scan(&r.ID, &r.Hostname, &r.Shell, &r.Script, &r.ExitCode,
			&stdout, &stderr, &r.DurationMs, &r.Status, &errStr, &ranBy, &r.RanAt); err != nil {
			return nil, fmt.Errorf("scan script_run: %w", err)
		}
		r.Stdout, r.Stderr, r.Error, r.RanBy = stdout.String, stderr.String, errStr.String, ranBy.String
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) Close() error { return s.db.Close() }
