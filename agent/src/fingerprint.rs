//! Device classification from open-port + NIC-vendor fingerprinting.
//!
//! Coarse but useful: turns "IP + MAC + ports" into a device class the dashboard
//! can icon/colour. Strong port signals win first, then vendor signals, then weak
//! port hints. Refined later by mDNS/SSDP service types and HTTP banners.

/// Classify a device into: router | printer | nas | camera | phone | workstation
/// | server | media | iot | network | device.
pub fn classify(ports: &[u16], vendor: &str) -> String {
    let v = vendor.to_lowercase();
    let has = |p: u16| ports.contains(&p);

    // --- strong port signals ---
    if has(9100) || has(515) || has(631) {
        return "printer".into();
    }
    if has(8009) || has(32400) {
        return "media".into(); // Chromecast / Plex
    }
    if has(554) || has(8554) {
        return "camera".into(); // RTSP
    }
    if has(5000) && has(5001) {
        return "nas".into();
    }
    if has(62078) {
        return "phone".into(); // iOS lockdownd
    }
    if has(445) || has(3389) || has(135) || has(139) {
        return "workstation".into();
    }

    // --- vendor signals ---
    let any = |list: &[&str]| list.iter().any(|s| v.contains(s));
    if any(&["hp", "canon", "epson", "brother", "xerox", "toshiba", "lexmark", "kyocera", "ricoh"]) {
        return "printer".into();
    }
    if any(&["hikvision", "dahua", "axis", "reolink", "amcrest", "uniview"]) {
        return "camera".into();
    }
    if any(&["synology", "qnap", "buffalo", "western digital"]) {
        return "nas".into();
    }
    if any(&["ubiquiti", "mikrotik", "cisco", "tp-link", "tplink", "netgear", "asus", "zyxel", "aruba", "ruckus", "fortinet", "draytek", "routerboard"]) {
        if has(80) || has(443) || has(53) {
            return "router".into();
        }
        return "network".into();
    }
    if any(&["apple", "samsung", "xiaomi", "huawei", "oneplus"]) {
        return "phone".into();
    }
    if any(&["espressif", "tuya", "sonos", "amazon", "nest", "midea", "broadlink", "shelly", "raspberry"]) {
        return "iot".into();
    }

    // --- weak port hints ---
    if has(80) || has(443) {
        return "device".into();
    }
    if has(22) {
        return "server".into();
    }
    "device".into()
}

#[cfg(test)]
mod tests {
    use super::classify;

    #[test]
    fn printer_by_port() {
        assert_eq!(classify(&[80, 9100], ""), "printer");
    }
    #[test]
    fn workstation_by_port_wins_over_vendor() {
        // an Asus PC with a TP-Link NIC and SMB open is a workstation, not a router
        assert_eq!(classify(&[135, 139, 445], "TP-Link"), "workstation");
    }
    #[test]
    fn router_by_vendor_and_web() {
        assert_eq!(classify(&[80, 443], "TP-Link"), "router");
    }
    #[test]
    fn iot_midea_ac() {
        assert_eq!(classify(&[], "GD Midea Air-Conditioning"), "iot");
    }
    #[test]
    fn printer_by_vendor() {
        assert_eq!(classify(&[80], "Toshiba"), "printer");
    }
    #[test]
    fn unknown_is_device() {
        assert_eq!(classify(&[], ""), "device");
    }
}
