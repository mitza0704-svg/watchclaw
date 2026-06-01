//! SSDP (UPnP) discovery — finds devices that announce themselves on the LAN
//! via multicast: routers/gateways, smart TVs, media renderers, NAS, some IoT.
//! Complements the ARP sweep with a self-reported device class + server string.
//! Pure std (UDP multicast), no extra crates.

use std::collections::HashMap;
use std::net::UdpSocket;
use std::time::{Duration, Instant};

#[derive(Debug, Default, Clone)]
pub struct SsdpInfo {
    /// SERVER header — usually "OS/ver UPnP/1.0 Product/ver".
    pub server: String,
    /// Search target / device type URN.
    pub st: String,
}

/// Broadcast an M-SEARCH and collect responses for `timeout_secs`.
/// Returns source-IP -> SsdpInfo (first response per IP wins).
pub fn discover(timeout_secs: u64) -> HashMap<String, SsdpInfo> {
    let mut out = HashMap::new();
    let sock = match UdpSocket::bind("0.0.0.0:0") {
        Ok(s) => s,
        Err(_) => return out,
    };
    let _ = sock.set_read_timeout(Some(Duration::from_millis(800)));
    let msearch = "M-SEARCH * HTTP/1.1\r\n\
                   HOST: 239.255.255.250:1900\r\n\
                   MAN: \"ssdp:discover\"\r\n\
                   MX: 2\r\n\
                   ST: ssdp:all\r\n\r\n";
    let _ = sock.send_to(msearch.as_bytes(), "239.255.255.250:1900");

    let mut buf = [0u8; 2048];
    let deadline = Instant::now() + Duration::from_secs(timeout_secs);
    while Instant::now() < deadline {
        match sock.recv_from(&mut buf) {
            Ok((n, src)) => {
                let resp = String::from_utf8_lossy(&buf[..n]);
                out.entry(src.ip().to_string()).or_insert(SsdpInfo {
                    server: header(&resp, "SERVER"),
                    st: header(&resp, "ST"),
                });
            }
            Err(_) => continue, // read timeout tick; loop until deadline
        }
    }
    out
}

/// Map an SSDP search-target URN to a device type. Empty if no strong signal.
pub fn device_type_from_st(st: &str) -> &'static str {
    let s = st.to_lowercase();
    if s.contains("internetgatewaydevice") || s.contains("wandevice") || s.contains("wanconnectiondevice") {
        "router"
    } else if s.contains("mediarenderer") || s.contains("mediaserver") || s.contains("dial") {
        "media"
    } else if s.contains("printer") {
        "printer"
    } else if s.contains("basic:1") {
        "iot"
    } else {
        ""
    }
}

fn header(resp: &str, key: &str) -> String {
    for line in resp.lines() {
        if let Some((k, v)) = line.split_once(':') {
            if k.trim().eq_ignore_ascii_case(key) {
                return v.trim().to_string();
            }
        }
    }
    String::new()
}
