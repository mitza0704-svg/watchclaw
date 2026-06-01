//! MAC OUI -> NIC vendor lookup, backed by the full IEEE registry
//! (Wireshark `manuf`, embedded at build time → offline, suveran).
//!
//! NOTE: this resolves the network-card maker, NOT the device identity.

use std::collections::HashMap;
use std::sync::OnceLock;

static MANUF: &str = include_str!("manuf.txt");

fn table() -> &'static HashMap<String, String> {
    static T: OnceLock<HashMap<String, String>> = OnceLock::new();
    T.get_or_init(|| {
        let mut m = HashMap::new();
        for line in MANUF.lines() {
            let line = line.trim_end();
            if line.is_empty() || line.starts_with('#') {
                continue;
            }
            let mut parts = line.split('\t');
            let prefix = parts.next().unwrap_or("").trim();
            let short = parts.next().unwrap_or("").trim();
            if prefix.len() < 8 || short.is_empty() {
                continue;
            }
            // Key on the 24-bit OUI (first "XX:XX:XX"). Longer (/28,/36) blocks
            // collapse onto their base OUI — fine for vendor display.
            let key = prefix[..8].to_uppercase();
            m.entry(key).or_insert_with(|| short.to_string());
        }
        m
    })
}

/// Vendor for a MAC like "50:EB:F6:D0:FC:92". Empty if unknown.
pub fn vendor(mac: &str) -> String {
    if mac.len() < 8 {
        return String::new();
    }
    table().get(&mac[..8].to_uppercase()).cloned().unwrap_or_default()
}
