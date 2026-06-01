# 🔴 LIVE PROGRESS — Watchclaw

> Jurnal actualizat în timp real de Claude. Ține-l deschis (auto-refresh). Newest pe sus.
> Început mod autonom: 2026-06-01

---

## ✅ #2 Auto-topologie — DONE & DOVEDIT (2026-06-01)
Pipeline complet: agent scan (5 device) → POST /v1/discovery (202) → store SQLite → builder → GET /v1/topology = graf JSON. Internet→gateway(192.168.1.1)→4 device-uri, clasificare (endpoint/device/printer din porturi), vendor (TP-Link). Toți 7 pașii ✅.
Note: topologie L3-stea (corect fără SNMP); L2 port-level vine cu collectorul SNMP. Mic cosmetic: em-dash în label (UTF-8 ok în JSON/browser, doar PowerShell îl afișează greșit).

## ✅ #5 Dashboard topologie (prototip) — DONE & DOVEDIT VIZUAL (2026-06-01)
Control-plane servește dashboard la `/` (Cytoscape embedat local — NU CDN, suveran/offline). Randează graful /v1/topology: Internet→gateway→device-uri, colorat pe tip, dark, legendă. Screenshot trimis user. Blocker rezolvat: CDN unpkg blocat în sandbox Playwright → descărcat cytoscape.min.js (365KB) + go:embed + servit la /cytoscape.min.js.
Note: e PROTOTIP de vizualizare. Dashboard-ul premium Next.js (design Apple/Antigravity) = faza `web/` serioasă, cu echipa de design.

## 📱 ACCES MOBIL LIVE (confirmat pe 5G ✅)
- Topologie: http://100.106.15.60:8787  (auto-refresh 10s)
- Progres (pagina asta): http://100.106.15.60:8787/progress  (auto-refresh 15s)
Server persistent (watchclaw-cp.exe), Tailscale, de oriunde.

## ✅ OUI complet — DONE (2026-06-01)
Baza IEEE completă (57k prefixe, Wireshark manuf) embedată local în agent (offline/suveran). Descoperiri reale pe rețeaua ta:
- Gateway 192.168.1.1 = **TP-Link** (confirmă routerul)
- 192.168.1.205 = **GD Midea Air-Conditioning** (AC smart / IoT!)
- DESKTOP-P43N3LK = **ASUSTek** (Asus-ul tău)
Mic TODO cosmetic: folosesc short-name Wireshark („TPLink"); pot trece la long-name („TP-Link Corporation") pt lizibilitate.

## ✅ Corecție topologie — DONE & DOVEDIT (feedback Mihai)
- [x] Agent: gateway real din routing table (netdev) ✅ → 192.168.1.1
- [x] Agent: hostname reverse DNS ✅ → .221=DESKTOP-P43N3LK (Asus!), .229=Precision5550
- [x] Label: hostname principal, OUI doar „NIC vendor" metadata ✅
- [x] Builder folosește gateway real ✅
- [x] Rebuild + restart + rescan → hartă corectată live ✅ (screenshot trimis)
Lecție aplicată: OUI = producător NIC, NU identitate device. Hostname (reverse DNS/NetBIOS) = identitatea reală.

## ✅ Agent LOOP — agentul raportează continuu (2026-06-01)
`watchclaw-agent loop` rulează la 30s: telemetry + scan → control-plane. Dashboard-ul se actualizează SINGUR (vezi timestamp-ul schimbându-se pe telefon, fără să rulez eu nimic).
**2 bug-uri reale prinse de loop & reparate:**
1. Server respingea câmpul `hardware` (DisallowUnknownFields → HTTP 400). Fix: `Hardware json.RawMessage` în model + decode tolerant (forward-compat agent↔server).
2. Agentul murea la prima eroare de POST (`exit(1)` în loop). Fix: `deliver` returnează succes, loop-ul tolerează erori și continuă — un hiccup de rețea nu mai omoară agentul.
✅ Loop STABIL confirmat (ciclu complet, proces viu, 0 erori). Cadență optimizată: telemetry 30s, scan 300s (scanul greu rar). Footprint: agent 22.8 MB + control-plane 11.9 MB.

## 🟢 SISTEM LIVE & AUTONOM (2026-06-01)
control-plane + agent loop rulează persistent pe Dell. Dashboard pe telefon se actualizează singur:
- telemetrie (CPU/RAM/disk) la 30s
- topologie + inventar la 300s
Vezi: http://100.106.15.60:8787 (topologie) · /progress (jurnalul ăsta)

## Coadă următoare
#3 SNMP collector (Npcap+admin, rețea birou completă) · Event Log (alerting) · store-and-forward queue local · multi-tenancy+enroll+mTLS · dashboard premium Next.js (Apple/Antigravity).

---

## ✅ DONE (sesiunea 2026-06-01)
- Pipeline metrici end-to-end (agent Rust → control-plane Go → store → API)
- Inventar hardware profund (USB/serial/disk/BIOS) — dovedit pe Dell
- #1 software + updates + SMART (clean-room, nativ)
- Network discovery etapa 1 (ARP-union, 5 device pe LAN real)
- Decizii: stack polyglot, remote MeshCentral+RustDesk, proprietar (Breeze=ref), clean-room
- Backlog NirSoft filtrat (~40 capabilități de reimplementat)
