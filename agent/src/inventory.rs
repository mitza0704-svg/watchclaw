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
    use crate::model::{DiskDrive, HardwareInventory, InstalledApp, UsbDevice};
    use serde::Deserialize;
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

        Ok(inv)
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
