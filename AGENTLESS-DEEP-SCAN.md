# Agentless deep scan — tool-uri & librării (research 2026-06-02)

> Cât de detaliat poate vedea un PC "din rețea" + ce tool-uri folosim. 3 agenți research, licențe verificate.
> Regula de aur: **embed doar MIT/Apache/BSD**; GPL/AGPL doar ca **subprocess** (JSON in/out, fără linking); **nmap NU se livrează** (NPSL interzice bundling comercial fără OEM).

## Răspunsul scurt
- **Fără credențiale:** vezi exteriorul (porturi, OS fingerprint, servicii, mDNS/SSDP/SNMP). NU procese/software/user.
- **CU credențiale (WinRM/WMI/SSH/SNMP):** ~80-90% din ce vede un agent instalat — procese, software, servicii, patch-uri, user logat, event logs — **remote, poll-based**. (Dovedit deja: Asus via WinRM.)
- Agentul instalat rămâne necesar pt: real-time continuu, NAT/internet, offline, acțiuni pe host.

## Windows — agentless credentialed (control-plane Go)
- **PRIMAR: WinRM/WS-Man** — `masterzen/winrm` (**Apache-2.0** ✅), rulează de pe Linux, port unic 5985/5986. Execută PowerShell `Get-CimInstance`/registry/event-log → JSON. **Dovedit pe Asus.**
- **SECUNDAR: SMB/admin shares** — `hirochachacha/go-smb2` (**BSD-2** ✅) pt fișiere + hive-uri registry.
- **Raw MSRPC (opțional):** `oiweiwei/go-msrpc` (Apache-2.0) — SCMR/TSCH/winreg/EVEN6 fără PowerShell.
- ⚠️ **Win32_Product NU** (declanșează MSI self-repair, doar MSI) → citește registry Uninstall keys. **Win32_QuickFixEngineering** doar CBS → combină cu WUA/`Get-HotFix`.
- ⚠️ Workgroup: `LocalAccountTokenFilterPolicy=1` (l-am setat pe Asus) + firewall (WinRM port unic >> WMI port 135+dinamice). Preferă WS-Man (quiet, fără service/task creation ca PsExec).
- Referință tehnică (NU linka): Impacket (wmiexec/smbexec/atexec/reg), NetExec. Lansweeper/PDQ folosesc exact WMI+registry.
- Rust: ecosistem mai subțire; `sspi-rs` (MIT/Apache) = auth NTLM/Kerberos. **Fă scanarea în Go.**

## Linux/macOS — SSH (control-plane Go / agent Rust)
- Go: `golang.org/x/crypto/ssh` (BSD-3) + `knownhosts` (NU InsecureIgnoreHostKey). Rust: **russh** (Apache-2.0, pur-Rust).
- Baterie comenzi read-only: `os-release/uname`, `dpkg/rpm/apk/pacman` (software), `systemctl`, `ps`, `ss -tulpn`, `getent passwd`, `lshw -json`/`dmidecode`, cron, SUID. macOS: `system_profiler -json`, `csrutil/spctl` (postură securitate).
- Cont scan dedicat + sudoers NOPASSWD îngust pt binarele exacte.

## Network/IoT — SNMP deep (Go)
- `gosnmp` (BSD) + `gosmi` (MIT, parser MIB — NU hardcoda OID-uri). SNMPv3 authPriv.
- **HOST-RESOURCES-MIB** (`.1.3.6.1.2.1.25`) = procese + software instalat pe host-uri SNMP! `hrStorage/hrProcessor` = disk/CPU.
- **Printer-MIB** (RFC 3805) = page counts (`prtMarkerLifeCount`) + toner (`prtMarkerSuppliesLevel`) → **exact datele MeterMind/Copiatoare, prin SNMP.**
- **ENTITY-MIB** = hardware/seriale/firmware. IF-MIB = interfețe/trafic. BRIDGE/Q-BRIDGE = CAM→topologie L2.

## Servere OOB + camere
- **Redfish** (BMC, vede hardware chiar cu OS-ul oprit): `gofish` (BSD-3) ✅ / `bmclib` (Apache, + fallback IPMI). 
- **ONVIF/WS-Discovery** camere: `use-go/onvif` (MIT) → model/firmware/serial.

## Securitate / adâncime maximă (vuln + config + SBOM)
- **SBOM→CVE:** `Syft`+`Grype` (Anchore, **Apache** ✅, air-gap) = backbone supply-chain. **#1 value/effort.**
- **All-in-one:** `Trivy` (Aqua, **Apache** ✅) — vuln host/fs + SBOM + secrets + misconfig. Scanner default pe agent.
- **Active network checks:** `Nuclei` (ProjectDiscovery, **MIT** ✅, SDK Go) — panouri expuse, default-creds, CVE network. (template-urile: licență separată, pull la runtime.)
- **CVE correlation suveran (IP-ul nostru):** date publice → engine propriu Go. **NVD API 2.0** (feed-urile JSON legacy DEPRECATE 2023; mirror `fkie-cad/nvd-json-data-feeds`) + **CISA KEV** (CC0, „patch now") + **EPSS** (prioritizare). NU single-source NVD (backlog enrichment). OVAL distro via `goval-dictionary` (subprocess) pt acuratețe Linux.
- **Compliance CIS/STIG:** OpenSCAP + SSG content (subprocess, LGPL/BSD content) pt Linux; Windows CIS = checks native în agent Rust (registry/secedit/auditpol). `Lynis` (GPL, subprocess) audit rapid.
- **Deep authenticated network:** OpenVAS/Greenbone (GPL, **appliance/subprocess** — deep-scan node opțional). `Vuls` (GPL, subprocess, offline Linux/Win).
- ⛔ **nmap = capcană licență** (NPSL, interzice bundling comercial). Înlocuim cu Nuclei+Trivy+scanner Rust mic. Sau OEM plătit.

## Arhitectura recomandată
**Firewall arhitectural:** scanerele copyleft rulează ca **subprocese/sidecar** (emit JSON); codul nostru proprietar Go doar consumă JSON → zero contaminare GPL/AGPL.
- **Agent Rust (unde poți instala):** inventar local adânc + Trivy/Syft local + real-time.
- **Control-plane Go (agentless, unde nu poți):** WinRM (Win) + SSH (Linux/mac) + gosnmp (network/printer) + gofish (servere) + onvif (camere) + Nuclei (active) + engine CVE (NVD2/KEV/EPSS).
- **Fuziune** keyed-MAC: aceeași mașină văzută agent + agentless = un asset, cu provenance.

## Licențe — ledger
✅ EMBED: masterzen/winrm, go-smb2, go-msrpc, x/crypto/ssh, russh, gosnmp, gosmi, gofish, bmclib, use-go/onvif, Syft, Grype, Trivy, Nuclei, sspi-rs, NetBox-schema, osquery-schema, NVD/KEV/EPSS (date publice).
⚠️ SUBPROCESS (GPL/AGPL, fără linking): Vuls, Lynis, OpenSCAP-CLI, OpenVAS/gvmd, goval-dictionary.
⛔ NU livra: nmap (NPSL), CIS-CAT Pro/Nessus/Qualys (comercial), Fingerbank-DB (licență), NirSoft (closed). Impacket/NetExec = doar referință tehnică.
