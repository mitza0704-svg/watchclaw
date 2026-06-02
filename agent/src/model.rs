//! Data model for endpoint telemetry reports.
//!
//! This is the wire format the agent sends to the control plane. It is OS-agnostic
//! so the same struct describes a Windows workstation, a Linux server, or a macOS host.

use serde::Serialize;

/// A single point-in-time snapshot of an endpoint's health.
#[derive(Debug, Serialize)]
pub struct EndpointReport {
    pub hostname: String,
    /// OS family name, e.g. "Windows", "Ubuntu", "Darwin".
    pub os: String,
    pub os_version: String,
    pub kernel_version: String,
    pub cpu_cores: usize,
    pub cpu_usage_pct: f32,
    pub mem_total_mb: u64,
    pub mem_used_mb: u64,
    pub mem_usage_pct: f32,
    pub uptime_seconds: u64,
    pub disks: Vec<DiskUsage>,
    /// Deep hardware inventory (USB, serials, system). Collected on a slower
    /// cadence than live metrics in production; None where unsupported.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub hardware: Option<HardwareInventory>,
    /// RFC 3339 / ISO 8601 UTC timestamp of collection.
    pub collected_at: String,
}

#[derive(Debug, Serialize)]
pub struct DiskUsage {
    pub mount: String,
    pub total_gb: f64,
    pub used_gb: f64,
    pub usage_pct: f64,
}

/// Point-in-time hardware inventory — "what you'd see standing at the machine".
#[derive(Debug, Serialize, Default)]
pub struct HardwareInventory {
    pub system: SystemInfo,
    pub disks: Vec<DiskDrive>,
    pub usb_devices: Vec<UsbDevice>,
    pub software: Vec<InstalledApp>,
    /// Installed Windows update KB ids.
    pub hotfixes: Vec<String>,
    /// Disks whose SMART subsystem predicts failure (empty = all healthy).
    pub disk_health_warnings: Vec<String>,
    /// Pending package updates (winget). Empty when all up to date / unsupported.
    #[serde(skip_serializing_if = "Vec::is_empty")]
    pub available_updates: Vec<PackageUpdate>,
    /// Auto-start services (the reboot-persistent surface) with binary path —
    /// the set that matters for ops health and rogue-service detection.
    #[serde(skip_serializing_if = "Vec::is_empty")]
    pub services: Vec<ServiceInfo>,
    /// Autorun entries (registry Run/RunOnce) — classic persistence surface.
    #[serde(skip_serializing_if = "Vec::is_empty")]
    pub startup_items: Vec<StartupItem>,
}

/// An auto-start Windows service. PathName is the binary command line — the
/// key field for spotting a service running from a suspicious location.
#[derive(Debug, Serialize, Default)]
pub struct ServiceInfo {
    pub name: String,
    pub display_name: String,
    pub state: String,      // Running | Stopped | ...
    pub start_mode: String, // Auto | Manual | Disabled
    pub path: String,
}

/// An autorun entry from a registry Run/RunOnce key.
#[derive(Debug, Serialize, Default)]
pub struct StartupItem {
    pub name: String,
    pub command: String,
    /// Which hive/key it came from, e.g. "HKLM\\...\\Run".
    pub location: String,
}

/// One pending package update reported by the OS package manager (winget).
#[derive(Debug, Serialize, Default)]
pub struct PackageUpdate {
    pub name: String,
    pub id: String,
    pub current: String,
    pub available: String,
}

#[derive(Debug, Serialize, Default)]
pub struct InstalledApp {
    pub name: String,
    pub version: String,
    pub publisher: String,
}

#[derive(Debug, Serialize, Default)]
pub struct SystemInfo {
    pub manufacturer: String,
    pub model: String,
    pub bios_serial: String,
    pub baseboard_serial: String,
}

#[derive(Debug, Serialize, Default)]
pub struct DiskDrive {
    pub model: String,
    pub serial: String,
    pub size_gb: f64,
    pub interface: String,
}

#[derive(Debug, Serialize, Default)]
pub struct UsbDevice {
    pub name: String,
    pub manufacturer: String,
    /// USB Vendor ID (4 hex chars) parsed from the PNP device id, when present.
    pub vid: String,
    /// USB Product ID (4 hex chars).
    pub pid: String,
    pub serial: String,
    pub pnp_id: String,
}

/// Result of a LAN scan run by a collector agent. This is the raw material the
/// control plane fuses (via MAC) into the BADUC-style topology graph.
#[derive(Debug, Serialize, Default)]
pub struct NetworkScan {
    pub subnet: String,
    pub scanned_at: String,
    pub host_count: usize,
    /// Real default-gateway IP from the routing table (not guessed).
    pub gateway: String,
    pub devices: Vec<NetworkDevice>,
}

#[derive(Debug, Serialize, Default)]
pub struct NetworkDevice {
    pub ip: String,
    pub mac: String,
    /// Resolved hostname (reverse DNS / NetBIOS), when available — the real identity.
    pub hostname: String,
    /// NIC vendor from the MAC OUI prefix. This is the network-card maker,
    /// NOT the device type/identity (e.g. an Asus PC can have a TP-Link NIC).
    pub nic_vendor: String,
    /// Best-guess device class from port + vendor fingerprinting:
    /// router | printer | nas | camera | phone | workstation | server | media | iot | device.
    pub device_type: String,
    pub open_ports: Vec<u16>,
    /// Service banners grabbed from open ports, e.g. "ssh: OpenSSH_8.9p1",
    /// "http: nginx". Cheap L7 identity; empty when nothing responded.
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub banners: Vec<String>,
}
