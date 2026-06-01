//! Active LAN discovery — the "collector" capability.
//!
//! Stage 1 (this file): derive the local subnet, probe every host in parallel
//! (TCP connect on common ports — a connection *or* a refusal both prove the
//! host is alive), read the OS ARP table to map IP -> MAC, and resolve the
//! vendor from the MAC OUI prefix.
//!
//! Roadmap (stage 2): ICMP + raw ARP for hosts that drop TCP silently, SNMP +
//! LLDP/CDP for switch-level L2 topology, mDNS/SSDP for self-announcing devices,
//! and a full IEEE OUI database instead of the embedded shortlist below.
//! Scope is configurable per site; only run on authorised networks.

use std::collections::HashMap;
use std::net::{IpAddr, Ipv4Addr, SocketAddr, TcpStream};
use std::time::Duration;

use rayon::prelude::*;

use crate::model::{NetworkDevice, NetworkScan};

/// Ports probed for liveness + light service identification.
const PROBE_PORTS: &[u16] = &[22, 53, 80, 135, 139, 443, 445, 515, 3389, 8080, 9100, 62078];
const CONNECT_TIMEOUT: Duration = Duration::from_millis(300);
/// Safety cap so a misconfigured mask can't trigger a 65k-host sweep.
const MAX_HOSTS: u32 = 1024;

pub fn scan() -> NetworkScan {
    let Some((base, prefix)) = local_subnet() else {
        eprintln!("discovery: could not determine local subnet");
        return NetworkScan::default();
    };

    let (start, end) = host_range(base, prefix);
    let total = end.saturating_sub(start) + 1;
    let subnet = format!("{}/{}", base, prefix);

    // Probe every host in parallel. The TCP attempts double as ARP triggers:
    // even a host that drops TCP must answer ARP at L2, populating the cache.
    let probed: Vec<(String, Vec<u16>, bool)> = (start..=end)
        .into_par_iter()
        .map(|h| {
            let ip = Ipv4Addr::from(h);
            let (ports, alive) = probe_host(ip);
            (ip.to_string(), ports, alive)
        })
        .collect();

    let arp = arp_table();
    let mut by_ip: HashMap<String, NetworkDevice> = HashMap::new();

    // Source 1: TCP-reachable hosts (with their open ports).
    for (ip, ports, alive) in probed {
        if alive || !ports.is_empty() {
            by_ip.insert(
                ip.clone(),
                NetworkDevice { ip, open_ports: ports, ..Default::default() },
            );
        }
    }

    // Source 2: ARP cache — catches L2-present devices that ignore TCP.
    for (ip, mac) in &arp {
        if !in_subnet(ip, base, prefix) {
            continue;
        }
        let dev = by_ip
            .entry(ip.clone())
            .or_insert_with(|| NetworkDevice { ip: ip.clone(), ..Default::default() });
        dev.mac = mac.clone();
        dev.nic_vendor = crate::oui::vendor(mac); // NIC maker, NOT device identity
    }

    let mut devices: Vec<NetworkDevice> = by_ip.into_values().collect();
    devices.sort_by(|a, b| ip_key(&a.ip).cmp(&ip_key(&b.ip)));

    // Self-announced devices: SSDP/UPnP (router/media/...) and mDNS/DNS-SD
    // (printers, Chromecast, NAS, HomeKit) — both refine type + identity.
    let ssdp = crate::ssdp::discover(3);
    let mdns = crate::mdns::discover(3);

    // Identity pipeline: fingerprint (port+vendor) -> SSDP -> mDNS, plus the best
    // available hostname (reverse-DNS, falling back to mDNS).
    for d in devices.iter_mut() {
        d.hostname = resolve_hostname(&d.ip);
        d.device_type = crate::fingerprint::classify(&d.open_ports, &d.nic_vendor);

        if d.device_type == "device" {
            if let Some(info) = ssdp.get(&d.ip) {
                let t = crate::ssdp::device_type_from_st(&info.st);
                if !t.is_empty() {
                    d.device_type = t.to_string();
                }
            }
        }
        if let Some(m) = mdns.get(&d.ip) {
            if d.device_type == "device" {
                let t = crate::mdns::device_type_from_services(&m.service_types);
                if !t.is_empty() {
                    d.device_type = t.to_string();
                }
            }
            if d.hostname.is_empty() && !m.hostname.is_empty() {
                d.hostname = m.hostname.clone();
            }
        }
    }

    NetworkScan {
        subnet,
        scanned_at: chrono::Utc::now().to_rfc3339(),
        host_count: total as usize,
        gateway: default_gateway(),
        devices,
    }
}

/// Real default-gateway IP from the OS routing table (not guessed from .1).
fn default_gateway() -> String {
    match netdev::get_default_gateway() {
        Ok(gw) => gw
            .ipv4
            .first()
            .map(|ip| ip.to_string())
            .unwrap_or_default(),
        Err(_) => String::new(),
    }
}

/// Best-effort reverse-DNS hostname. Empty when the network has no PTR records
/// (common on LANs) — the control plane then correlates by MAC with agent inventory.
fn resolve_hostname(ip: &str) -> String {
    let Ok(addr) = ip.parse::<std::net::IpAddr>() else {
        return String::new();
    };
    match dns_lookup::lookup_addr(&addr) {
        Ok(name) if name != ip => name,
        _ => String::new(),
    }
}

/// Returns (open ports, alive). A connection or a refusal both prove liveness;
/// the attempt also forces ARP resolution for on-link hosts.
fn probe_host(ip: Ipv4Addr) -> (Vec<u16>, bool) {
    let mut open = Vec::new();
    let mut alive = false;
    for &port in PROBE_PORTS {
        let addr = SocketAddr::new(IpAddr::V4(ip), port);
        match TcpStream::connect_timeout(&addr, CONNECT_TIMEOUT) {
            Ok(_) => {
                open.push(port);
                alive = true;
            }
            Err(e) if e.kind() == std::io::ErrorKind::ConnectionRefused => {
                alive = true;
            }
            Err(_) => {}
        }
    }
    (open, alive)
}

fn in_subnet(ip: &str, base: Ipv4Addr, prefix: u8) -> bool {
    match ip.parse::<Ipv4Addr>() {
        Ok(addr) => network_base(addr, prefix) == base,
        Err(_) => false,
    }
}

/// Pick the real LAN to scan. Prefers a private RFC1918 interface with a true
/// subnet mask (prefix <= 30), which naturally skips Tailscale (100.64/10 CGNAT,
/// /32 point-to-point), loopback and link-local. Falls back to any /<=30 IPv4.
fn local_subnet() -> Option<(Ipv4Addr, u8)> {
    let ifaces = if_addrs::get_if_addrs().ok()?;
    let mut fallback: Option<(Ipv4Addr, u8)> = None;

    for iface in ifaces {
        if iface.is_loopback() {
            continue;
        }
        let if_addrs::IfAddr::V4(v4) = iface.addr else {
            continue;
        };
        if v4.ip.is_link_local() {
            continue;
        }
        let prefix = netmask_to_prefix(v4.netmask);
        if prefix >= 31 {
            continue; // point-to-point (e.g. Tailscale /32) — not a scannable LAN
        }
        let base = network_base(v4.ip, prefix);
        if v4.ip.is_private() {
            return Some((base, prefix)); // real LAN — use it
        }
        fallback.get_or_insert((base, prefix));
    }
    fallback
}

fn netmask_to_prefix(mask: Ipv4Addr) -> u8 {
    u32::from(mask).count_ones() as u8
}

fn network_base(ip: Ipv4Addr, prefix: u8) -> Ipv4Addr {
    let mask = if prefix == 0 { 0 } else { u32::MAX << (32 - prefix) };
    Ipv4Addr::from(u32::from(ip) & mask)
}

/// Inclusive host-address range (excludes network + broadcast for prefix <= 30),
/// capped at MAX_HOSTS.
fn host_range(base: Ipv4Addr, prefix: u8) -> (u32, u32) {
    let base_u = u32::from(base);
    if prefix >= 31 {
        return (base_u, base_u);
    }
    let size = 1u32 << (32 - prefix);
    let start = base_u + 1;
    let mut end = base_u + size - 2;
    if end - start + 1 > MAX_HOSTS {
        end = start + MAX_HOSTS - 1;
    }
    (start, end)
}

fn ip_key(ip: &str) -> u32 {
    ip.parse::<Ipv4Addr>().map(u32::from).unwrap_or(0)
}

// ---------- ARP table ----------

#[cfg(windows)]
fn arp_table() -> HashMap<String, String> {
    let mut map = HashMap::new();
    let Ok(out) = std::process::Command::new("arp").arg("-a").output() else {
        return map;
    };
    let text = String::from_utf8_lossy(&out.stdout);
    for line in text.lines() {
        let cols: Vec<&str> = line.split_whitespace().collect();
        if cols.len() >= 2 {
            if let Ok(ip) = cols[0].parse::<Ipv4Addr>() {
                let mac = normalize_mac(cols[1]);
                if mac.len() == 17 {
                    map.insert(ip.to_string(), mac);
                }
            }
        }
    }
    map
}

#[cfg(target_os = "linux")]
fn arp_table() -> HashMap<String, String> {
    let mut map = HashMap::new();
    if let Ok(text) = std::fs::read_to_string("/proc/net/arp") {
        for line in text.lines().skip(1) {
            let cols: Vec<&str> = line.split_whitespace().collect();
            if cols.len() >= 4 {
                if cols[0].parse::<Ipv4Addr>().is_ok() {
                    let mac = normalize_mac(cols[3]);
                    if mac.len() == 17 && mac != "00:00:00:00:00:00" {
                        map.insert(cols[0].to_string(), mac);
                    }
                }
            }
        }
    }
    map
}

#[cfg(not(any(windows, target_os = "linux")))]
fn arp_table() -> HashMap<String, String> {
    HashMap::new()
}

/// Normalise `aa-bb-cc-dd-ee-ff` or `aa:bb:...` to upper-case colon form.
fn normalize_mac(raw: &str) -> String {
    raw.replace('-', ":").to_uppercase()
}
