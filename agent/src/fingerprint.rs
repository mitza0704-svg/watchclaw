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

/// Classify a device from its hostname naming convention alone.
/// Returns None when the name carries no signal. These are widely-used,
/// vendor-published conventions (e.g. Windows' `WIN-` auto-names, UniFi `UDM`),
/// not reverse-engineered from any tool — pure heuristic over public knowledge.
pub fn classify_hostname(hostname: &str) -> Option<String> {
    let h = hostname.to_lowercase();
    if h.is_empty() {
        return None;
    }
    let has = |s: &str| h.contains(s);

    if h.starts_with("win-") || has("server") || has("srv") || has("-dc") || h.starts_with("dc0")
        || has("vmhost") || has("esxi") || has("proxmox") || has("hyperv")
    {
        return Some("server".into());
    }
    if has("truenas") || has("freenas") || has("synology") || has("diskstation")
        || has("qnap") || has("nas")
    {
        return Some("nas".into());
    }
    if has("printer") || has("officejet") || has("laserjet") || has("mfp") || has("-print")
    {
        return Some("printer".into());
    }
    if has("ipcam") || has("ipc-") || has("nvr") || has("dvr") || has("camera") {
        return Some("camera".into());
    }
    if has("unifi") || has("udm") || has("usw") || has("uap") || has("switch")
        || h.starts_with("sw-") || has("router") || h.starts_with("rt-") || h.starts_with("ap-")
        || has("gateway") || has("mikrotik") || has("openwrt")
    {
        return Some("network".into());
    }
    if has("iphone") || has("ipad") || has("android") || has("galaxy") || has("pixel")
        || has("redmi")
    {
        return Some("phone".into());
    }
    if has("chromecast") || has("appletv") || has("firetv") || has("roku") || has("shield-") {
        return Some("media".into());
    }
    if h.starts_with("esp") || has("shelly") || has("tasmota") || has("sonoff") || has("tuya") {
        return Some("iot".into());
    }
    if h.starts_with("desktop-") || h.starts_with("laptop-") || h.starts_with("pc-")
        || has("precision") || has("thinkpad") || has("latitude") || has("macbook") || has("imac")
    {
        return Some("workstation".into());
    }
    None
}

/// Fold a hostname signal into an existing classification. A hostname only
/// overrides when it is more specific than what port+vendor produced: it
/// upgrades the generic "device", and promotes a "workstation" to "server"
/// when the name clearly says so (e.g. WIN-*, *srv*). Strong port-derived
/// classes (printer/nas/camera/...) are left untouched.
pub fn refine_with_hostname(current: &str, hostname: &str) -> String {
    match classify_hostname(hostname) {
        None => current.to_string(),
        Some(h) => {
            if current == "device" {
                h
            } else if current == "workstation" && h == "server" {
                h
            } else {
                current.to_string()
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::{classify, classify_hostname, refine_with_hostname};

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

    #[test]
    fn hostname_win_prefix_is_server() {
        assert_eq!(classify_hostname("WIN-A3K9DLO2").as_deref(), Some("server"));
    }
    #[test]
    fn hostname_udm_is_network() {
        assert_eq!(classify_hostname("UDM-Pro-Max").as_deref(), Some("network"));
    }
    #[test]
    fn hostname_truenas_is_nas() {
        assert_eq!(classify_hostname("truenas.local").as_deref(), Some("nas"));
    }
    #[test]
    fn hostname_desktop_is_workstation() {
        assert_eq!(classify_hostname("DESKTOP-P43N3LK").as_deref(), Some("workstation"));
    }
    #[test]
    fn hostname_no_signal_is_none() {
        assert_eq!(classify_hostname("box42"), None);
        assert_eq!(classify_hostname(""), None);
    }
    #[test]
    fn refine_upgrades_device_only() {
        // a generic device named like a NAS becomes a nas
        assert_eq!(refine_with_hostname("device", "synology-ds"), "nas");
        // but a port-proven printer is never downgraded by a vague name
        assert_eq!(refine_with_hostname("printer", "win-server"), "printer");
        // workstation -> server promotion when the name says server
        assert_eq!(refine_with_hostname("workstation", "WIN-DC01"), "server");
        // no hostname signal keeps the original
        assert_eq!(refine_with_hostname("workstation", "box42"), "workstation");
    }
}
