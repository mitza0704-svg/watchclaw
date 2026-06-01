//! mDNS / DNS-SD discovery — the richest identification layer on a modern LAN.
//! Printers (AirPrint/IPP), Chromecast, Apple TV/AirPlay, NAS (SMB/AFP), HomeKit
//! accessories announce themselves with a hostname + service type. Gives both a
//! good hostname and a precise device type.

use std::collections::HashMap;
use std::time::{Duration, Instant};

use mdns_sd::{ServiceDaemon, ServiceEvent};

#[derive(Debug, Default, Clone)]
pub struct MdnsInfo {
    pub hostname: String,
    pub service_types: Vec<String>,
}

/// Browse common service types for `timeout_secs`, returning IP -> MdnsInfo.
pub fn discover(timeout_secs: u64) -> HashMap<String, MdnsInfo> {
    let mut out: HashMap<String, MdnsInfo> = HashMap::new();
    let mdns = match ServiceDaemon::new() {
        Ok(d) => d,
        Err(_) => return out,
    };

    const TYPES: &[&str] = &[
        "_ipp._tcp.local.",
        "_printer._tcp.local.",
        "_pdl-datastream._tcp.local.",
        "_airplay._tcp.local.",
        "_raop._tcp.local.",
        "_googlecast._tcp.local.",
        "_spotify-connect._tcp.local.",
        "_smb._tcp.local.",
        "_afpovertcp._tcp.local.",
        "_ssh._tcp.local.",
        "_hap._tcp.local.",
        "_http._tcp.local.",
    ];

    let mut receivers = Vec::new();
    for t in TYPES {
        if let Ok(rx) = mdns.browse(t) {
            receivers.push((*t, rx));
        }
    }

    let deadline = Instant::now() + Duration::from_secs(timeout_secs);
    while Instant::now() < deadline {
        for (t, rx) in &receivers {
            while let Ok(ev) = rx.recv_timeout(Duration::from_millis(40)) {
                if let ServiceEvent::ServiceResolved(info) = ev {
                    let host = info.get_hostname().trim_end_matches('.').to_string();
                    for addr in info.get_addresses() {
                        let entry = out.entry(addr.to_string()).or_default();
                        if entry.hostname.is_empty() {
                            entry.hostname = host.clone();
                        }
                        if !entry.service_types.iter().any(|s| s == t) {
                            entry.service_types.push((*t).to_string());
                        }
                    }
                }
            }
        }
    }

    let _ = mdns.shutdown();
    out
}

/// Map advertised service types to a device type. Empty if inconclusive.
pub fn device_type_from_services(types: &[String]) -> &'static str {
    let has = |needle: &str| types.iter().any(|t| t.contains(needle));
    if has("_ipp") || has("_printer") || has("_pdl") {
        "printer"
    } else if has("_googlecast") || has("_airplay") || has("_raop") || has("_spotify") {
        "media"
    } else if has("_smb") || has("_afpovertcp") || has("_nfs") {
        "nas"
    } else if has("_hap") {
        "iot"
    } else if has("_ssh") {
        "server"
    } else {
        ""
    }
}
