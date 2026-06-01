//! Lightweight L7 banner grabbing for open ports.
//!
//! Many services announce themselves on connect (SSH/FTP/SMTP/POP3/IMAP) or in
//! an HTTP `Server:` header. Reading that banner is a cheap, high-signal way to
//! confirm what a host actually runs — far stronger than a port number alone.
//!
//! std-only (TcpStream + short timeouts); no extra crates. TLS (443) is skipped
//! for now — it needs a TLS handshake we don't want to pull a dep for yet.

use std::io::{Read, Write};
use std::net::{TcpStream, ToSocketAddrs};
use std::time::Duration;

const CONNECT_TIMEOUT: Duration = Duration::from_millis(700);
const IO_TIMEOUT: Duration = Duration::from_millis(700);
const MAX_READ: usize = 512;

/// Ports worth probing and how to label them. HTTP gets an active request; the
/// rest send nothing and read the greeting the server pushes on connect.
fn label_for(port: u16) -> Option<&'static str> {
    match port {
        22 => Some("ssh"),
        21 => Some("ftp"),
        23 => Some("telnet"),
        25 | 587 => Some("smtp"),
        110 => Some("pop3"),
        143 => Some("imap"),
        80 | 8080 => Some("http"),
        _ => None,
    }
}

/// Grab a single banner from `ip:port`. Returns e.g. "ssh: OpenSSH_8.9p1".
/// None when the port isn't a known banner port or nothing useful came back.
pub fn grab(ip: &str, port: u16) -> Option<String> {
    let label = label_for(port)?;
    let addr = format!("{ip}:{port}");
    let sock = addr.to_socket_addrs().ok()?.next()?;
    let mut stream = TcpStream::connect_timeout(&sock, CONNECT_TIMEOUT).ok()?;
    stream.set_read_timeout(Some(IO_TIMEOUT)).ok()?;
    stream.set_write_timeout(Some(IO_TIMEOUT)).ok()?;

    let is_http = matches!(port, 80 | 8080);
    if is_http {
        // Minimal HEAD so we don't pull a body; Host header keeps vhosts happy.
        let req = format!("HEAD / HTTP/1.0\r\nHost: {ip}\r\nUser-Agent: Watchclaw\r\n\r\n");
        stream.write_all(req.as_bytes()).ok()?;
    }

    let mut buf = [0u8; MAX_READ];
    let n = stream.read(&mut buf).ok()?;
    if n == 0 {
        return None;
    }
    let text = String::from_utf8_lossy(&buf[..n]);

    let value = if is_http {
        // Pull the Server: header if present.
        header_value(&text, "server").unwrap_or_else(|| {
            text.lines().next().unwrap_or("").trim().to_string()
        })
    } else {
        // First non-empty line is the greeting banner.
        text.lines()
            .map(str::trim)
            .find(|l| !l.is_empty())
            .unwrap_or("")
            .to_string()
    };

    let value = sanitize(&value);
    if value.is_empty() {
        return None;
    }
    Some(format!("{label}: {value}"))
}

/// Case-insensitive HTTP header lookup.
fn header_value(response: &str, name: &str) -> Option<String> {
    let want = format!("{}:", name.to_lowercase());
    response.lines().find_map(|line| {
        let lower = line.to_lowercase();
        if lower.starts_with(&want) {
            Some(line[want.len()..].trim().to_string())
        } else {
            None
        }
    })
}

/// Strip control chars and clamp length so a hostile banner can't bloat the
/// payload or break the dashboard.
fn sanitize(s: &str) -> String {
    let cleaned: String = s
        .chars()
        .filter(|c| !c.is_control())
        .take(120)
        .collect();
    cleaned.trim().to_string()
}

/// A device's most identifying banner, if any (prefers ssh/http over the rest).
pub fn primary<'a>(banners: &'a [String]) -> Option<&'a str> {
    let pref = |b: &str| b.starts_with("ssh:") || b.starts_with("http:");
    banners
        .iter()
        .find(|b| pref(b))
        .or_else(|| banners.first())
        .map(String::as_str)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn http_server_header_parsed() {
        let resp = "HTTP/1.1 200 OK\r\nServer: nginx/1.24.0\r\nDate: x\r\n\r\n";
        assert_eq!(header_value(resp, "server").as_deref(), Some("nginx/1.24.0"));
    }
    #[test]
    fn header_lookup_is_case_insensitive() {
        let resp = "HTTP/1.0 200 OK\r\nSERVER: Apache\r\n\r\n";
        assert_eq!(header_value(resp, "server").as_deref(), Some("Apache"));
    }
    #[test]
    fn sanitize_strips_control_and_clamps() {
        assert_eq!(sanitize("SSH-2.0-OpenSSH_8.9\r\n"), "SSH-2.0-OpenSSH_8.9");
        assert_eq!(sanitize(&"x".repeat(200)).len(), 120);
    }
    #[test]
    fn non_banner_port_is_none() {
        assert_eq!(grab("127.0.0.1", 9), None); // discard port, not a banner port
    }
    #[test]
    fn primary_prefers_ssh_http() {
        let b = vec!["ftp: vsftpd".to_string(), "ssh: OpenSSH_9".to_string()];
        assert_eq!(primary(&b), Some("ssh: OpenSSH_9"));
        assert_eq!(primary(&[]), None);
    }
}
