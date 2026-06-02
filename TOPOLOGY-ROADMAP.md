# Topology — cum o facem mai bună și mai detaliată decât UniFi

> Sinteză din research paralel (4 agenți: L2/L3 discovery, fingerprinting, vizualizare, diferențiatori). 2026-06-02.
> Surse: web, GitHub, StackOverflow, NirSoft, docs vendor. Licențe verificate pentru embed comercial.

## Adevărul de bază (poziționare onestă)
- **Topologia la nivel de port-switch necesită un switch manageabil** (SNMP sau LLDP pe el). Niciun protocol nu o expune de pe un switch „prost". UniFi o are doar pentru că *e* firmware-ul switch-ului. Poziționăm clar: „port-level unde există switch manageabil; L2-adjacency + inventar bogat pe segmente unmanaged."
- **Moat-ul nostru structural** (ce UniFi NU poate face): colectăm deja **conexiuni TCP per-proces** + **inventar host la nivel de agent**. Astea dau **application-dependency mapping** + detaliu host — capabilități pe care un controller de rețea nu le poate adăuga fără agent. Ăsta e wedge-ul.

## Coloana vertebrală: FUZIUNE multi-sursă keyed pe MAC (modelul BADUC)
Un singur asset reconciliat din: agent + SNMP/LLDP + DHCP + ARP + passive sniff + import manual. Reguli (lecții din runZero):
- Cheie pe **(site, MAC) → IP → hostname → fingerprint**, NU pe IP gol.
- **Provenance per-atribut** (`source:<name>`); agent/L2-adjacent > observație rutată L3.
- MAC randomizat / VM clonate → scoring multi-atribut, nu cheie unică.
- Filtru „phantom" (routere/firewalls răspund la tot).
- **Merge/split reversibil de la început** — duplicatele nu se auto-vindecă.

## Roadmap etapizat (fiecare etapă livrează un win vizibil + alimentează următoarea)

### Etapa 1 — „Harta pe care UniFi n-o poate desena" (DOAR date pe care le avem deja)
- **1a. Detaliu host pe fiecare nod**: click → porturi listening per-proces, conexiuni stabilite, software/servicii. (avem deja datele)
- **1b. Edge-uri de dependență aplicație** ⭐ semnătura: agregăm conexiunile TCP per-proces între agenți → overlay serviciu-la-serviciu pe hartă. UniFi = zero vizibilitate proces.
- **1c. Badge risc port expus**: colorăm nodurile după riscul porturilor listening (din datele agent).
- **1d. Reconciliere MAC v0**: un asset per MAC, agent autoritar, first/last-seen + câmpuri provenance puse din start.

### Etapa 2 — „Timp + blast-radius" (tot date existente)
- **2a. Time-travel**: snapshot append-only al grafului la fiecare ciclu → feed „ce s-a schimbat", alerte device nou, link-flaps. (UniFi = doar prezent)
- **2b. Blast-radius / SPOF**: articulation points + bridges (Tarjan, ~50 linii, fără dependență) → „dacă pică switch-ul X, ce **servicii** cad" (combinat cu graful app din 1b).
- **2c. Rogue-device v0**: orice observat (ARP/agent) care nu e în asset DB → flag.

### Etapa 3 — „Vezi ce NU rulează agent" (surse de discovery noi, în agentul Rust)
- **3a. Discovery multicast FUSION** ⭐ cel mai bun value×feasibility: mDNS (`mdns-sd`, MIT) + SSDP/UPnP + WS-Discovery/ONVIF (`onvif-rs`, MIT/Apache) + SNMP `sysObjectID`/`sysDescr` + NetBIOS → **model + firmware exact** pt printere/camere/NAS/TV pe care UniFi le arată ca „unknown". Toate crate-uri permisive.
- **3b. LLDP/CDP passive sniff în agent** ⭐ paritate UniFi fără credențiale: ascultăm frame-uri 0x88CC/CDP (`pcap`/`pnet`, MIT/Apache, BPF filter) → vecinul direct = **switch + portul exact** unde e host-ul. Necesită admin/`CAP_NET_RAW` (Npcap pe Win).
- **3c. DHCP fingerprinting**: Option-55 (`dhcproto`, MIT) → identifică orice device DHCP. DB: Fingerbank (licență plătită) SAU DB proprie.
- **3d. OUI/MAC vendor** (IEEE public) + handling MAC randomization (bit U/L).

### Etapa 4 — „Topologie reală L2 din switch-uri manageabile" (control-plane Go)
- **4a. SNMP FDB + bridge-port** (`gosnmp`, BSD): `dot1dTpFdbTable → dot1dBasePortIfIndex → ifName` = MAC→port fizic. Q-BRIDGE pt VLAN. **Harta port-level reală.**
- **4b. SNMP LLDP-MIB/CDP crawl** recursiv (algoritm din natlas/secure_cartography — NU codul lor, sunt GPL) → backbone switch↔switch.
- **4c. Fuziune**: port cu 1 MAC = edge endpoint; port cu N MAC + LLDP = uplink switch↔switch. Confidence tier (LLDP-confirmed > FDB > inferred).

### Etapa 5 — „Securitate & flow intelligence" (moat-ul greu)
- **5a. CVE overlay**: feed-uri **NVD JSON** (public, offline, suveran) → CPE/firmware→CVE pe noduri. Nmap ca **subprocess** (NPSL — NU embed) pt versionare adâncă unde nu există agent; sau probe proprii Rust.
- **5b. Traffic-flow topology**: `goflow2` (BSD) ingest NetFlow/sFlow/IPFIX → edge-uri who-talks-to-whom cu bandwidth → ClickHouse (Apache). (ntopng/Akvorado = GPL/AGPL, doar referință)
- **5c. Segmentation violations**: flow + zone → alertă trafic cross-VLAN interzis.
- **5d. Attack-path / blast-radius fuzionat**: porturi expuse + CVE + reachability flow + SPOF = „ce atinge un atacator de aici" (pitch-ul Armis/runZero, dar self-hosted EU).

## Vizualizare (rămânem pe Cytoscape.js MIT)
Extensii (toate MIT): `fcose` (force), `expand-collapse` (grupuri VLAN/site colapsabile), `navigator` (minimap), `cytoscape-popper` + `node-html-label` (tooltip/panel bogat), `cytoscape-layers` (heatmap overlay). `ELK.js` ca backend de layout pt rutare ortogonală (diagramă reală, nu pânză). `sigma.js` (WebGL) = escape hatch pt >3-5k noduri.
- **Layer toggle L2 / L3 / app-dependency** (același model, swap edges).
- **Path trace A→B** (Dijkstra built-in, highlight + dim restul).
- **Edge heatmap** (lățime+culoare = load; dash animat = direcție).
- **Grupuri colapsabile** pe VLAN/site (compound nodes).
- **Prag map↔listă ~200 noduri** → peste, default tabel filtrabil cu „show on map".
- Modele de furat: Datadog NPM (click-edge → metrici), Auvik (toggle L2/L3 live), LibreNMS weathermap (doar ideea de edge-utilization).

## Licențe — cheat-sheet pt EMBED (link în binar)
| ✅ EMBED-SAFE | ⚠️ Verifică / sidecar | ❌ NU embed (GPL/AGPL/NPSL/proprietar) |
|---|---|---|
| gosnmp (BSD), pcap/pnet (MIT/Apache), mdns-sd (MIT), onvif-rs (MIT/Apache), dhcproto (MIT/Apache), windows-rs (MIT/Apache), JA3 + JA4-original (BSD), goflow2 (BSD), ClickHouse (Apache), NVD feeds (public), IEEE OUI (public), Cytoscape + ext (MIT), ELK.js (EPL—verifică), sigma.js (MIT) | Netdisco (BSD per un agent / GPL per altul — **VERIFICĂ înainte**), ssdp-client/csnmp/mac_oui/passivetcp-rs (verifică Cargo.toml), Zeek (BSD dar greu — sidecar) | nmap (NPSL→subprocess ok, NU link), masscan (AGPL), RustScan (GPL3), p0f-cod (GPL2), JA4+ (FoxIO), ntopng (GPL3), Akvorado (AGPL3), Suricata (GPL2), Fingerbank-DB (licență plătită), NirSoft (closed — doar referință clean-room) |

## Poziționare într-o frază
„UniFi îți arată firele. Noi îți arătăm firele, host-urile de pe ele, aplicațiile care vorbesc între ele, ușile deschise și găurile cunoscute, ce s-a schimbat de ieri, și ce cade dacă oricare pică — totul fuzionat într-o hartă keyed pe MAC, găzduită pe sol EU."

## Surse cheie
- LLDP: lldpd (ISC), mdlayher/lldp (MIT). SNMP FDB: Cisco mac-to-port, Q-BRIDGE-MIB, gosnmp (BSD).
- Fingerprint: nmap (NPSL), p0f→passivetcp-rs, JA3 (BSD), Fingerbank, dhcproto (MIT), onvif-rs (MIT).
- Viz: Cytoscape.js + fcose/expand-collapse/navigator (MIT), sigma.js, ELK.js, Datadog NPM, Auvik.
- Diferențiatori: runZero (understanding-assets, dedup pitfalls), goflow2 (BSD), NVD feeds, articulation points (Tarjan).
