// Package model defines the telemetry wire format shared with the agent.
// Field tags MUST match the JSON the Rust agent emits (see agent/src/model.rs).
package model

import "encoding/json"

type EndpointReport struct {
	Hostname      string      `json:"hostname"`
	OS            string      `json:"os"`
	OSVersion     string      `json:"os_version"`
	KernelVersion string      `json:"kernel_version"`
	CPUCores      int         `json:"cpu_cores"`
	CPUUsagePct   float64     `json:"cpu_usage_pct"`
	MemTotalMB    uint64      `json:"mem_total_mb"`
	MemUsedMB     uint64      `json:"mem_used_mb"`
	MemUsagePct   float64     `json:"mem_usage_pct"`
	UptimeSeconds uint64      `json:"uptime_seconds"`
	Disks         []DiskUsage `json:"disks"`
	// Hardware inventory kept as raw JSON (USB/serials/software/SMART) so the
	// payload round-trips losslessly without coupling to every agent field.
	Hardware    json.RawMessage `json:"hardware,omitempty"`
	CollectedAt string          `json:"collected_at"`
}

type DiskUsage struct {
	Mount    string  `json:"mount"`
	TotalGB  float64 `json:"total_gb"`
	UsedGB   float64 `json:"used_gb"`
	UsagePct float64 `json:"usage_pct"`
}

// NetworkScan is what a collector agent reports after a LAN sweep
// (mirrors agent/src/model.rs NetworkScan).
type NetworkScan struct {
	Subnet    string          `json:"subnet"`
	ScannedAt string          `json:"scanned_at"`
	HostCount int             `json:"host_count"`
	// Gateway is the real default-gateway IP reported by the collector.
	Gateway string          `json:"gateway"`
	Devices []NetworkDevice `json:"devices"`
	// Reporter is the hostname of the collector agent (set server-side if empty).
	Reporter string `json:"reporter,omitempty"`
}

type NetworkDevice struct {
	IP       string `json:"ip"`
	MAC      string `json:"mac"`
	Hostname string `json:"hostname"`
	// NicVendor = network-card maker (MAC OUI). NOT the device identity.
	NicVendor string `json:"nic_vendor"`
	// DeviceType = agent's port+vendor fingerprint (router/printer/nas/...).
	DeviceType string `json:"device_type"`
	OpenPorts  []int  `json:"open_ports"`
}

// TopologyGraph is the fused network map served to the dashboard.
type TopologyGraph struct {
	Subnet string     `json:"subnet"`
	Nodes  []TopoNode `json:"nodes"`
	Edges  []TopoEdge `json:"edges"`
}

type TopoNode struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Type      string `json:"type"` // internet | gateway | endpoint | device
	IP        string `json:"ip,omitempty"`
	MAC       string `json:"mac,omitempty"`
	Hostname  string `json:"hostname,omitempty"`
	NicVendor string `json:"nic_vendor,omitempty"`
	OpenPorts []int  `json:"open_ports,omitempty"`
}

type TopoEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
	Type string `json:"type"` // wan | l3 | l2
}
