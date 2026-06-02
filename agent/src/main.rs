//! Watchclaw RMM endpoint agent — entry point.
//!
//! Usage:
//!   watchclaw-agent           collect a telemetry snapshot
//!   watchclaw-agent scan      run an active LAN discovery
//!
//! If WATCHCLAW_URL (control-plane base URL, e.g. http://host:8787) is set, the
//! result is POSTed to the matching endpoint; otherwise it is printed as JSON.
//! Next increment: scheduled loop + local store-and-forward + mTLS + enroll.

mod banner;
mod collector;
mod discovery;
mod fingerprint;
mod inventory;
mod jobs;
mod mdns;
mod model;
mod oui;
mod ssdp;
mod transport;

fn main() {
    let base = std::env::var("WATCHCLAW_URL").ok().filter(|s| !s.is_empty());

    match std::env::args().nth(1).as_deref() {
        Some("scan") => {
            let scan = discovery::scan();
            if !deliver(base.as_deref(), "/v1/discovery", &scan, "scan", scan.devices.len()) {
                std::process::exit(1);
            }
        }
        Some("loop") => run_loop(base.as_deref()),
        _ => {
            let report = collector::collect();
            if !deliver(base.as_deref(), "/v1/telemetry", &report, &report.hostname, 1) {
                std::process::exit(1);
            }
        }
    }
}

/// Run forever: report telemetry + network scan every WATCHCLAW_INTERVAL seconds
/// (default 60). This turns the agent into a continuous reporter so the dashboard
/// refreshes itself. Store-and-forward retry queue lands next.
fn run_loop(base: Option<&str>) {
    let interval = std::env::var("WATCHCLAW_INTERVAL")
        .ok()
        .and_then(|s| s.parse::<u64>().ok())
        .unwrap_or(60);
    // Network scan is expensive (~40-60s for a /24) and intrusive, so run it on a
    // slower cadence than telemetry. SCAN_EVERY cycles between scans.
    const SCAN_EVERY: u64 = 10;

    // The command channel polls on its OWN fast cadence in a separate thread so a
    // queued job is picked up promptly even while the main loop is mid-scan
    // (a /24 scan blocks for ~40-60s). Default 5s, override WATCHCLAW_JOB_INTERVAL.
    if let Some(b) = base {
        let job_url = b.to_string();
        let job_interval = std::env::var("WATCHCLAW_JOB_INTERVAL")
            .ok()
            .and_then(|s| s.parse::<u64>().ok())
            .unwrap_or(5);
        std::thread::spawn(move || loop {
            jobs::poll_and_run(&job_url);
            std::thread::sleep(std::time::Duration::from_secs(job_interval));
        });
        println!("command channel polling every {job_interval}s");
    }

    println!("agent loop started (telemetry every {interval}s, scan every {}s)", interval * SCAN_EVERY);
    let mut tick: u64 = 0;
    loop {
        // Errors are logged inside deliver and intentionally do NOT stop the loop —
        // a transient network/server hiccup must never kill the agent.
        let report = collector::collect();
        deliver(base, "/v1/telemetry", &report, &report.hostname, 1);
        if tick % SCAN_EVERY == 0 {
            let scan = discovery::scan();
            deliver(base, "/v1/discovery", &scan, "scan", scan.devices.len());
        }
        tick = tick.wrapping_add(1);
        std::thread::sleep(std::time::Duration::from_secs(interval));
    }
}

/// POST `body` to `base + path`, or print it as JSON when no base URL is set.
/// Returns true on success (or print mode). Never exits — the caller decides.
fn deliver<T: serde::Serialize>(base: Option<&str>, path: &str, body: &T, label: &str, count: usize) -> bool {
    match base {
        Some(b) => {
            let url = format!("{}{}", b.trim_end_matches('/'), path);
            match transport::post_json(&url, body) {
                Ok(status) => {
                    println!("sent {label} ({count}) -> {url} status={status}");
                    true
                }
                Err(e) => {
                    eprintln!("send failed: {e}");
                    false
                }
            }
        }
        None => {
            println!("{}", serde_json::to_string_pretty(body).expect("serialize"));
            true
        }
    }
}
