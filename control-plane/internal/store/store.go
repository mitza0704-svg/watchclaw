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

type Store interface {
	SaveReport(ctx context.Context, r model.EndpointReport) error
	ListEndpoints(ctx context.Context) ([]EndpointSummary, error)
	SaveScan(ctx context.Context, s model.NetworkScan) error
	LatestScan(ctx context.Context) (*model.NetworkScan, error)
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
`)
	if err != nil {
		return fmt.Errorf("init schema: %w", err)
	}
	return nil
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

func (s *SQLiteStore) Close() error { return s.db.Close() }
