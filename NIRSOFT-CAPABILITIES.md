# NirSoft → backlog de capabilități agent (filtrat RMM)

> Catalog complet scanat: 183 tool-uri (subagent, 2026-06-01). Aici = verdictul de CTO pe fiecare grup.
> Regulă: NirSoft = HARTĂ de capabilități, reimplementate NATIV în Rust (Win32/WMI/registry/wevtapi). NICIODATĂ bundling de binare.

## 🚫 INTERZIS — extragere credențiale/parole (linie roșie: malware/GDPR)
Toată secțiunea Password Recovery (~36): WebBrowserPassView, ChromePass, PasswordFox, Mail PassView, LSASecretsView/Dump, VaultPasswordView, CredentialsFileView, CredHistView, DataProtectionDecryptor, EncryptedRegView, Dialupass, VNCPassView, RouterPassView, Network Password Recovery, Remote Desktop PassView, MessenPass, SecurityQuestionsView, Protected Storage PassView etc.
Din network: **SniffPass, WirelessKeyView, WebCookiesSniffer** (fură parole/cookies/sesiuni).
→ Un RMM care extrage credențiale = clasificat malware de EDR + GDPR. NENEGOCIABIL.

## ⬜ SKIP — privacy / irelevant RMM
- Internet/Web browser (~46): tot ce e history/cache/cookies/bookmarks/autofill (BrowsingHistoryView, *CacheView, *CookiesView, *HistoryView, MyLastSearch). Browsing-ul userului NU e treaba RMM.
- Video/Audio (8): VideoCacheView, WebVideoCap etc. (SoundVolume — irelevant).
- Outlook content (NK2Edit, OutlookAttachView, OutlookStatView): privacy.
- Desktop tweaks: Volumouse, CustomExplorerToolbar, NirExt, TurnFlash, CustomizeIE.
- Programmer low-level: DLL Export Viewer, GDIView, HeapMemView, RuntimeClassesView (irelevant RMM).

## ✅ REIMPLEMENTĂM — RMM-relevant (backlog prioritizat)

### Deja DONE (sesiunea 2026-06-01)
USB inventory (USBDeview-style), software (UninstallView/InstalledAppView), updates (WinUpdatesView), SMART (DiskSmartView), network discovery (Wireless Network Watcher/LANIPScanner-style), OUI vendor (MACAddressView-style).

### Prioritate ÎNALTĂ (diagnostic + securitate core)
| Capabilitate | NirSoft ref | Sursă nativă | Valoare |
|---|---|---|---|
| Windows Event Log | FullEventLogView, EventLogChannelsView | wevtapi | alerting core RMM |
| Active connections | CurrPorts, LiveTcpUdpWatch, ProcessTCPSummary | GetExtendedTcpTable | securitate (ce vorbește afară) |
| Scheduled Tasks | TaskSchedulerView | Task Scheduler COM | persistence/securitate |
| Shell extensions | ShellExView, ShellMenuView | registry | persistence/malware |
| Services + drivers | (ServiWin), DriverView | WMI Win32_Service/SystemDriver | inventar + diagnostic |
| **NTFS Alternate Data Streams** | AlternateStreamView | FindFirstStreamW | **detecție malware ascuns** |
| Logon history | WinLogOnView | Event Log security | audit |
| Boot/uptime history | TurnedOnTimesView | Event Log 6005/6006/6008 | **crash detection (Kernel-Power 41!)** |
| App crashes / hangs | WinCrashReport, WhatIsHang, BlueScreenView | WER + minidumps | diagnostic proactiv |

### Prioritate MEDIE (inventar + monitoring extins)
| Capabilitate | NirSoft ref | Valoare |
|---|---|---|
| **Wake-on-LAN** | WakeMeOnLan | pornire remote stații (feature RMM clasic) |
| USB history/audit | USBDriveLog, USBLogView | ce s-a conectat (securitate DLP) |
| Disk I/O monitoring | DiskCountersView, FileActivityWatch, AppReadWriteCounter | diagnostic perf |
| Disk usage | FoldersReport, FreeSpaceLogView, DriveLetterView | capacity planning |
| DNS monitoring | DNSQuerySniffer | securitate (detecție C2/exfil) |
| Network per-app bandwidth | AppNetworkCounter, NetworkUsageView | monitoring |
| Battery health | BatteryInfoView | laptops |
| Activity timeline | LastActivityView | audit forensic |
| Device manager complet | (DevManView), DeviceIOView | inventar device-uri |
| License inventory | ProduKey (Windows/Office keys din registry) | asset/license mgmt — LEGITIM RMM (≠ furt parole) |
| Registry diagnostic | RegistryChangesView, RegScanner, RegFromApp | troubleshooting |
| Open file handles | OpenedFilesView | diagnostic (fișier blocat) |
| Remote command exec | NirCmd-style | avem nevoie oricum pt automatizări |

### 🟡 GRI — doar la cerere/diagnostic explicit (risc privacy, NU continuu)
Packet capture: SmartSniff, NetworkTrafficView, HTTPNetworkSniffer. Util pt troubleshooting avansat, dar captează date → activabil doar manual, cu consimțământ, audit.

## Net-net
Din 183 tools: ~40 capabilități RMM-relevante de reimplementat, ~36 interzise (parole), ~50 skip (privacy browser), restul irelevante. NirSoft confirmă că „a vedea tot pe Windows" e fezabil din surse native — nu avem nevoie de NirSoft, doar de aceleași API-uri.
