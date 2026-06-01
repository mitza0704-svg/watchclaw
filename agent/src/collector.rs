//! OS telemetry collection via the cross-platform `sysinfo` crate.
//!
//! `collect()` produces one [`EndpointReport`]. CPU utilisation requires two
//! samples spaced by `MINIMUM_CPU_UPDATE_INTERVAL`, so this call blocks briefly.

use sysinfo::{Disks, System};

use crate::model::{DiskUsage, EndpointReport};

pub fn collect() -> EndpointReport {
    let mut sys = System::new_all();
    sys.refresh_all();

    // CPU usage is a delta between two refreshes; take a short sample window.
    sys.refresh_cpu_usage();
    std::thread::sleep(sysinfo::MINIMUM_CPU_UPDATE_INTERVAL);
    sys.refresh_cpu_usage();

    let mem_total = sys.total_memory(); // bytes
    let mem_used = sys.used_memory(); // bytes
    let mem_usage_pct = if mem_total > 0 {
        (mem_used as f64 / mem_total as f64 * 100.0) as f32
    } else {
        0.0
    };

    let disks = Disks::new_with_refreshed_list();
    let disk_usage: Vec<DiskUsage> = disks
        .iter()
        .map(|d| {
            let total = d.total_space();
            let avail = d.available_space();
            let used = total.saturating_sub(avail);
            DiskUsage {
                mount: d.mount_point().to_string_lossy().to_string(),
                total_gb: round2(total as f64 / 1e9),
                used_gb: round2(used as f64 / 1e9),
                usage_pct: if total > 0 {
                    round2(used as f64 / total as f64 * 100.0)
                } else {
                    0.0
                },
            }
        })
        .collect();

    EndpointReport {
        hostname: System::host_name().unwrap_or_else(|| "unknown".into()),
        os: System::name().unwrap_or_else(|| "unknown".into()),
        os_version: System::os_version().unwrap_or_default(),
        kernel_version: System::kernel_version().unwrap_or_default(),
        cpu_cores: sys.cpus().len(),
        cpu_usage_pct: round2_f32(sys.global_cpu_usage()),
        mem_total_mb: mem_total / 1024 / 1024,
        mem_used_mb: mem_used / 1024 / 1024,
        mem_usage_pct: round2_f32(mem_usage_pct),
        uptime_seconds: System::uptime(),
        disks: disk_usage,
        hardware: crate::inventory::collect(),
        collected_at: chrono::Utc::now().to_rfc3339(),
    }
}

fn round2(v: f64) -> f64 {
    (v * 100.0).round() / 100.0
}

fn round2_f32(v: f32) -> f32 {
    (v * 100.0).round() / 100.0
}
