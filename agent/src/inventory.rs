//! Deep hardware inventory — "what you'd see standing at the machine".
//!
//! Windows uses WMI (system model, BIOS/baseboard serials, disk serials,
//! USB devices with VID/PID/serial). Linux/macOS collectors land later
//! (dmidecode/udev/lsusb, ioreg/system_profiler).

use crate::model::HardwareInventory;

#[cfg(windows)]
pub fn collect() -> Option<HardwareInventory> {
    match windows::collect() {
        Ok(inv) => Some(inv),
        Err(e) => {
            eprintln!("hardware inventory failed: {e}");
            None
        }
    }
}

#[cfg(not(windows))]
pub fn collect() -> Option<HardwareInventory> {
    None // TODO: Linux (dmidecode/udev/lsusb), macOS (ioreg/system_profiler)
}

#[cfg(windows)]
mod windows {
    use crate::model::{DiskDrive, HardwareInventory, InstalledApp, ServiceInfo, StartupItem, UsbDevice};
    use serde::Deserialize;
    use std::process::Command;

    #[derive(Deserialize)]
    #[serde(rename = "Win32_Service")]
    #[serde(rename_all = "PascalCase")]
    struct Win32Service {
        name: Option<String>,
        display_name: Option<String>,
        state: Option<String>,
        start_mode: Option<String>,
        path_name: Option<String>,
    }
    use wmi::{COMLibrary, WMIConnection};
    use winreg::enums::*;
    use winreg::RegKey;

    #[derive(Deserialize)]
    #[serde(rename = "Win32_QuickFixEngineering")]
    #[serde(rename_all = "PascalCase")]
    struct Qfe {
        #[serde(rename = "HotFixID")]
        hot_fix_id: Option<String>,
    }

    #[derive(Deserialize)]
    #[serde(rename = "MSStorageDriver_FailurePredictStatus")]
    #[serde(rename_all = "PascalCase")]
    struct FailurePredict {
        instance_name: Option<String>,
        predict_failure: Option<bool>,
    }

    #[derive(Deserialize)]
    #[serde(rename = "Win32_ComputerSystem")]
    #[serde(rename_all = "PascalCase")]
    struct ComputerSystem {
        manufacturer: Option<String>,
        model: Option<String>,
    }

    #[derive(Deserialize)]
    #[serde(rename = "Win32_BIOS")]
    #[serde(rename_all = "PascalCase")]
    struct Bios {
        serial_number: Option<String>,
    }

    #[derive(Deserialize)]
    #[serde(rename = "Win32_BaseBoard")]
    #[serde(rename_all = "PascalCase")]
    struct BaseBoard {
        serial_number: Option<String>,
    }

    #[derive(Deserialize)]
    #[serde(rename = "Win32_DiskDrive")]
    #[serde(rename_all = "PascalCase")]
    struct DiskDriveRaw {
        model: Option<String>,
        serial_number: Option<String>,
        size: Option<u64>,
        interface_type: Option<String>,
    }

    #[derive(Deserialize)]
    #[serde(rename = "Win32_PnPEntity")]
    #[serde(rename_all = "PascalCase")]
    struct PnpEntity {
        name: Option<String>,
        manufacturer: Option<String>,
        #[serde(rename = "PNPDeviceID")]
        pnp_device_id: Option<String>,
    }

    pub fn collect() -> Result<HardwareInventory, Box<dyn std::error::Error>> {
        let com = COMLibrary::new()?;
        let wmi = WMIConnection::new(com)?;
        let mut inv = HardwareInventory::default();

        if let Ok(rows) = wmi.query::<ComputerSystem>() {
            if let Some(cs) = rows.into_iter().next() {
                inv.system.manufacturer = cs.manufacturer.unwrap_or_default();
                inv.system.model = cs.model.unwrap_or_default();
            }
        }
        if let Ok(rows) = wmi.query::<Bios>() {
            if let Some(b) = rows.into_iter().next() {
                inv.system.bios_serial = b.serial_number.unwrap_or_default().trim().to_string();
            }
        }
        if let Ok(rows) = wmi.query::<BaseBoard>() {
            if let Some(b) = rows.into_iter().next() {
                inv.system.baseboard_serial =
                    b.serial_number.unwrap_or_default().trim().to_string();
            }
        }

        if let Ok(rows) = wmi.query::<DiskDriveRaw>() {
            for d in rows {
                let size_gb = d
                    .size
                    .map(|s| (s as f64 / 1e9 * 100.0).round() / 100.0)
                    .unwrap_or(0.0);
                inv.disks.push(DiskDrive {
                    model: d.model.unwrap_or_default().trim().to_string(),
                    serial: d.serial_number.unwrap_or_default().trim().to_string(),
                    size_gb,
                    interface: d.interface_type.unwrap_or_default(),
                });
            }
        }

        let usb: Vec<PnpEntity> = wmi
            .raw_query(
                "SELECT Name, Manufacturer, PNPDeviceID FROM Win32_PnPEntity \
                 WHERE PNPDeviceID LIKE 'USB%'",
            )
            .unwrap_or_default();
        for e in usb {
            let pnp = e.pnp_device_id.unwrap_or_default();
            let (vid, pid, serial) = parse_usb_id(&pnp);
            inv.usb_devices.push(UsbDevice {
                name: e.name.unwrap_or_default(),
                manufacturer: e.manufacturer.unwrap_or_default(),
                vid,
                pid,
                serial,
                pnp_id: pnp,
            });
        }

        // Installed software (registry Uninstall hive — NOT Win32_Product, which
        // is slow and can trigger MSI self-repair).
        inv.software = read_software();

        // Installed Windows updates.
        if let Ok(rows) = wmi.query::<Qfe>() {
            inv.hotfixes = rows.into_iter().filter_map(|q| q.hot_fix_id).collect();
        }

        // SMART predictive failure (root\WMI namespace).
        if let Ok(smart) = WMIConnection::with_namespace_path("root\\WMI", com) {
            if let Ok(rows) = smart.query::<FailurePredict>() {
                for r in rows {
                    if r.predict_failure == Some(true) {
                        inv.disk_health_warnings
                            .push(r.instance_name.unwrap_or_else(|| "unknown disk".into()));
                    }
                }
            }
        }

        // Pending package updates (winget). Best-effort; empty on any failure.
        inv.available_updates = read_winget_updates();

        // Auto-start services (persistence/ops surface) with binary path.
        let svcs: Vec<Win32Service> = wmi
            .raw_query(
                "SELECT Name, DisplayName, State, StartMode, PathName \
                 FROM Win32_Service WHERE StartMode='Auto'",
            )
            .unwrap_or_default();
        for s in svcs {
            inv.services.push(ServiceInfo {
                name: s.name.unwrap_or_default(),
                display_name: s.display_name.unwrap_or_default(),
                state: s.state.unwrap_or_default(),
                start_mode: s.start_mode.unwrap_or_default(),
                path: s.path_name.unwrap_or_default(),
            });
        }

        // Autorun entries (registry Run/RunOnce) — classic persistence surface.
        inv.startup_items = read_startup_items();

        Ok(inv)
    }

    /// Read autorun entries from the standard Run/RunOnce registry keys across
    /// HKLM (64-bit + WOW6432) and HKCU. These are the locations Autoruns and
    /// most persistence techniques use; documented Windows registry paths.
    fn read_startup_items() -> Vec<StartupItem> {
        const KEYS: &[(isize, &str, &str)] = &[
            (HKEY_LOCAL_MACHINE, r"SOFTWARE\Microsoft\Windows\CurrentVersion\Run", "HKLM\\Run"),
            (HKEY_LOCAL_MACHINE, r"SOFTWARE\Microsoft\Windows\CurrentVersion\RunOnce", "HKLM\\RunOnce"),
            (HKEY_LOCAL_MACHINE, r"SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Run", "HKLM\\WOW6432\\Run"),
            (HKEY_CURRENT_USER, r"SOFTWARE\Microsoft\Windows\CurrentVersion\Run", "HKCU\\Run"),
            (HKEY_CURRENT_USER, r"SOFTWARE\Microsoft\Windows\CurrentVersion\RunOnce", "HKCU\\RunOnce"),
        ];
        let mut out = Vec::new();
        for &(hive, path, label) in KEYS {
            let Ok(key) = RegKey::predef(hive).open_subkey(path) else {
                continue;
            };
            for (name, val) in key.enum_values().flatten() {
                out.push(StartupItem {
                    name,
                    command: val.to_string(),
                    location: label.to_string(),
                });
            }
        }
        out
    }

    /// Run winget and return the list of pending updates. winget's table is the
    /// only stable interface (no JSON for `upgrade`), so we disable the progress
    /// bar and hand the text to the cross-platform parser.
    fn read_winget_updates() -> Vec<crate::model::PackageUpdate> {
        // winget.exe is a WindowsApps App Execution Alias (a zero-byte reparse
        // point); spawning it directly via CreateProcess fails. Going through
        // `cmd /C` resolves the alias and PATH the way a shell would.
        let output = Command::new("cmd")
            .args([
                "/C",
                "winget",
                "upgrade",
                "--include-unknown",
                "--disable-interactivity",
            ])
            .env("WINGET_DISABLE_PROGRESS_BAR", "1")
            .output();
        match output {
            Ok(out) => super::parse_winget_table(&String::from_utf8_lossy(&out.stdout)),
            Err(_) => Vec::new(),
        }
    }

    /// Read installed applications from the Uninstall registry hive across the
    /// 64-bit HKLM, 32-bit (WOW6432Node) HKLM, and per-user HKCU views.
    fn read_software() -> Vec<InstalledApp> {
        const PATHS: &[(isize, &str)] = &[
            (
                HKEY_LOCAL_MACHINE,
                r"SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall",
            ),
            (
                HKEY_LOCAL_MACHINE,
                r"SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall",
            ),
            (
                HKEY_CURRENT_USER,
                r"SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall",
            ),
        ];
        let mut apps: Vec<InstalledApp> = Vec::new();
        for &(hive, path) in PATHS {
            let root = RegKey::predef(hive);
            let Ok(key) = root.open_subkey(path) else {
                continue;
            };
            for sub in key.enum_keys().flatten() {
                let Ok(app) = key.open_subkey(&sub) else {
                    continue;
                };
                let name: String = app.get_value("DisplayName").unwrap_or_default();
                if name.is_empty() {
                    continue; // updates/components without a display name
                }
                // Skip system component entries (SystemComponent = 1).
                if app.get_value::<u32, _>("SystemComponent").unwrap_or(0) == 1 {
                    continue;
                }
                apps.push(InstalledApp {
                    name,
                    version: app.get_value("DisplayVersion").unwrap_or_default(),
                    publisher: app.get_value("Publisher").unwrap_or_default(),
                });
            }
        }
        apps.sort_by(|a, b| a.name.to_lowercase().cmp(&b.name.to_lowercase()));
        apps.dedup_by(|a, b| a.name == b.name && a.version == b.version);
        apps
    }

    /// Parse `USB\VID_046D&PID_C52B\5&abc...` into (vid, pid, last-segment).
    /// The last segment is the device instance id; it equals the real serial
    /// when the device exposes one, otherwise a bus-enumerated path.
    fn parse_usb_id(pnp: &str) -> (String, String, String) {
        let upper = pnp.to_uppercase();
        let vid = extract4(&upper, "VID_");
        let pid = extract4(&upper, "PID_");
        let serial = pnp.rsplit('\\').next().unwrap_or("").to_string();
        (vid, pid, serial)
    }

    fn extract4(s: &str, key: &str) -> String {
        match s.find(key) {
            Some(i) => s[i + key.len()..].chars().take(4).collect(),
            None => String::new(),
        }
    }
}

/// Parse winget's fixed-width `upgrade` table into structured updates.
///
/// Cross-platform (pure string work) so it is unit-testable anywhere. Column
/// boundaries are taken from the *character* offsets of the header titles, not
/// fixed widths — robust to long names that winget truncates with an ellipsis,
/// and to the Unicode (®, …) that appears in product names. Progress/spinner
/// lines and the trailing summary are filtered out.
pub(crate) fn parse_winget_table(text: &str) -> Vec<crate::model::PackageUpdate> {
    use crate::model::PackageUpdate;

    // winget draws its progress bar with carriage returns (\r) instead of
    // newlines, so the spinner frames and the "Name Id Version" header land on
    // one logical line when captured. Normalize \r to \n so each frame becomes
    // its own (then-filtered) line and the header stands alone with correct
    // column offsets.
    let text = text.replace('\r', "\n");
    let text = text.as_str();

    let is_noise = |l: &&str| {
        let t = l.trim();
        t.is_empty()
            || l.contains('█')
            || l.contains('▒')
            // single-char progress spinner frames: "-", "\", "|", "/"
            || (t.len() <= 2 && t.chars().all(|c| matches!(c, '-' | '\\' | '|' | '/')))
    };
    let lines: Vec<&str> = text.lines().filter(|l| !is_noise(l)).collect();

    let header_idx = match lines.iter().position(|l| {
        l.contains("Name") && l.contains("Id") && l.contains("Version") && l.contains("Available")
    }) {
        Some(i) => i,
        None => return Vec::new(),
    };
    let header = lines[header_idx];

    // Character offset (not byte) where a column title begins in the header.
    let col = |title: &str| header.find(title).map(|b| header[..b].chars().count());
    let (id_off, ver_off, avail_off) = match (col("Id"), col("Version"), col("Available")) {
        (Some(a), Some(b), Some(c)) => (a, b, c),
        _ => return Vec::new(),
    };
    let src_off = col("Source");

    let slice = |chars: &[char], start: usize, end: Option<usize>| -> String {
        if start >= chars.len() {
            return String::new();
        }
        let e = end.unwrap_or(chars.len()).min(chars.len());
        if e <= start {
            return String::new();
        }
        chars[start..e].iter().collect::<String>().trim().to_string()
    };

    let mut out = Vec::new();
    for line in lines.iter().skip(header_idx + 1) {
        if line.trim_start().starts_with('-') {
            continue; // separator row
        }
        let chars: Vec<char> = line.chars().collect();
        if chars.len() <= id_off {
            continue; // summary line ("N upgrades available", etc.)
        }
        let name = slice(&chars, 0, Some(id_off));
        let id = slice(&chars, id_off, Some(ver_off));
        let current = slice(&chars, ver_off, Some(avail_off));
        let available = slice(&chars, avail_off, src_off);
        // A real update row has both an id and an available version.
        if id.is_empty() || available.is_empty() {
            continue;
        }
        out.push(PackageUpdate { name, id, current, available });
    }
    out
}

#[cfg(test)]
mod tests {
    use super::parse_winget_table;

    // winget's table is fixed-width. We reproduce that with format! widths so
    // the test exercises the parser, not hand-aligned spaces. Names carry the
    // same Unicode winget emits (®, the … truncation marker) to prove the
    // char-offset slicing survives multi-byte characters in the Name column.
    const W: (usize, usize, usize, usize) = (51, 34, 14, 14); // name,id,ver,avail
    fn row(name: &str, id: &str, cur: &str, avail: &str) -> String {
        format!("{:<n$}{:<i$}{:<v$}{:<a$}{}", name, id, cur, avail, "winget",
            n = W.0, i = W.1, v = W.2, a = W.3)
    }
    fn sample() -> String {
        let header = format!("{:<n$}{:<i$}{:<v$}{:<a$}{}", "Name", "Id", "Version", "Available",
            "Source", n = W.0, i = W.1, v = W.2, a = W.3);
        [
            header.as_str(),
            &"-".repeat(119),
            &row("7-Zip 26.00 (x64)", "7zip.7zip", "26.00", "26.01"),
            &row("HWiNFO\u{00ae} 64", "REALiX.HWiNFO", "8.16", "8.48"),
            &row("ImageMagick Q16 (2026-05-1\u{2026}", "ImageMagick.ImageMagick", "7.1.2.23", "7.1.2.24"),
            "13 upgrades available.",
        ]
        .join("\n")
    }

    #[test]
    fn parses_rows_and_skips_summary() {
        let ups = parse_winget_table(&sample());
        assert_eq!(ups.len(), 3, "should parse 3 update rows, not the summary");
        assert_eq!(ups[0].id, "7zip.7zip");
        assert_eq!(ups[0].current, "26.00");
        assert_eq!(ups[0].available, "26.01");
    }

    #[test]
    fn handles_unicode_in_name() {
        let ups = parse_winget_table(&sample());
        // ® and the … truncation must not desync the column slicing
        assert_eq!(ups[1].id, "REALiX.HWiNFO");
        assert_eq!(ups[1].available, "8.48");
        assert_eq!(ups[2].id, "ImageMagick.ImageMagick");
        assert_eq!(ups[2].available, "7.1.2.24");
    }

    #[test]
    fn empty_or_garbage_yields_nothing() {
        assert!(parse_winget_table("").is_empty());
        assert!(parse_winget_table("no table here\njust text").is_empty());
    }
}
