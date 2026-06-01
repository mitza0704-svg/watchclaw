# Stack de referință — tool-uri & proiecte adoptabile (GitHub/SO)

> Research 2026-06-01. Pentru fiecare componentă: ce adoptăm, licență (critic pt produs comercial), de ce.
> Regulă: **permisiv (MIT/Apache) = putem folosi/integra; AGPL/GPL = doar studiu de design, nu cod.**

## Agent (Rust)

| Nevoie | Adoptăm | Licență | Note |
|---|---|---|---|
| Raw ARP scan (înlocuiește TCP-liveness) | [`pnet`](https://github.com/libpnet/libpnet) (pattern [arp-scan-rs](https://github.com/kongbytes/arp-scan-rs)) | MIT/Apache ✅ | prinde TOATE device-urile L2 |
| SNMP (switch CAM/LLDP) | [`modern_snmp`](https://github.com/davedufresne/modern_snmp) | permisiv | pure-Rust SNMPv3 |
| Windows API | [`windows-rs`](https://github.com/microsoft/windows-rs) | MIT/Apache ✅ | oficial Microsoft |
| Windows service | [`windows-service`](https://crates.io/crates/windows-service) | MIT/Apache ✅ | event handlers stop/start |
| Hardware inventory | `wmi` + `winreg` | ✅ | **deja folosite** |
| **Auto-update agent** | [`self_update`](https://github.com/jaemk/self_update) sau [`patchify`](https://github.com/danwilliams/patchify) | MIT ✅ | self_update = matur (GitHub releases); patchify = server+client, canary/rolling |

## Control-plane (Go)

| Nevoie | Adoptăm | Licență | Note |
|---|---|---|---|
| Postgres/TimescaleDB client | [`pgx`](https://github.com/jackc/pgx) (NU lib/pq) | MIT ✅ | batch insert, perf |
| Identitate/mTLS agent enroll | [SPIFFE/SPIRE](https://spiffe.io) pattern | Apache 2.0 ✅ | SVID X.509 short-lived (1h), auto-rotate, scoped pe tenant |
| API | stdlib net/http (Go 1.22 routing) | ✅ | **deja folosit**; chi dacă scalează |

## TimescaleDB — best practices (din research)
- **Hypertables** chunk 1-day (optim pt compresie). Partiționare pe timp + device_id.
- **Batch insert 1000+ rânduri/INSERT** → throughput sute de mii rânduri/s.
- **Compresie** 90-95% pe chunk-uri vechi; `segmentby` = cheia de acces (ex device_id).
- **Retention policies**: high-res 30 zile, agregate (continuous aggregates) ani.
- Go: `pgx` + `COPY` pentru bulk.

## Web dashboard (Next.js)

| Nevoie | Adoptăm | Licență | De ce |
|---|---|---|---|
| **Vizualizare topologie** | [Cytoscape.js](https://js.cytoscape.org/) | MIT ✅ | cel mai bun pt grafuri mari: layout-uri multiple, clustering, canvas/WebGL. Standard pt network maps |
| Node-based UI (workflow editor automatizări) | [React Flow](https://reactflow.dev) | MIT ✅ | dacă facem editor vizual de automatizări |
| Alternativă WebGL grafuri uriașe | Sigma.js | MIT ✅ | dacă topologia depășește mii de noduri |
| _Evităm_ | vis-network | — | cel mai lent la scară |

## AI / copilot (Python)

| Nevoie | Adoptăm | Licență | Note |
|---|---|---|---|
| LLM gateway | [LiteLLM](https://github.com/BerriAI/litellm) | MIT ✅ | **deja în infra** (multi-model, cost-routing) |
| NL→SQL (interogare metrici/tichete în limbaj natural) | [nl2sql-agent](https://github.com/cmcouto-silva/nl2sql-agent) referință | studiu | LangChain/LangGraph/PGVector |
| RAG (knowledge base, docs) | [RAGFlow](https://github.com/infiniflow/ragflow) | Apache 2.0 ✅ | engine RAG matur + agent |

## Patch management (cap. RMM-core)

| Nevoie | Adoptăm | Licență | Note |
|---|---|---|---|
| Patching OS + third-party | [winget COM API](https://github.com/microsoft/winget-cli) | MIT ✅ | oficial; agentul orchestrează winget |
| Referință automation silent | [Winget-AutoUpdate (WAU)](https://github.com/Romanitho/Winget-AutoUpdate) | studiu | allowlist/blocklist, GPO |

## Referințe de arhitectură RMM (studiu de design, nu copiere)
- [breeze](https://github.com/lanternops/breeze) — AGPL, agent Go single-binary, **AI-native (agent SDK core)** → cel mai aproape de viziunea noastră. DOAR design.
- [jetrmm/rmm-agent](https://github.com/jetrmm/rmm-agent) — **MIT** ✅ → singurul de unde am putea reutiliza cod legal.
- [rusty-rmm](https://github.com/mtelahun/rusty-rmm) — RMM în Rust, referință.
- Topologie L2: [natlas](https://github.com/MJL85/natlas), [secure_cartography](https://github.com/scottpeterman/secure_cartography) — algoritm SNMP+LLDP+CDP → graf.

## Insight tehnic cheie (din StackOverflow/research)
> ARP table = doar device active ultimele ~4h. **SNMP la CAM table switch** (`dot1dTpFdbTable`/BRIDGE-MIB) + **LLDP-MIB** = vedere completă L2 (port/VLAN/MAC). Combinare L2+L3 = topologie 360°. → confirmă de ce collectorul trebuie cablat + SNMP.

## Verdict licențe — ce putem face
- **Integrăm liber** (MIT/Apache): pnet, modern_snmp, windows-rs/service, self_update, pgx, SPIRE, Cytoscape, React Flow, pgx, winget-cli, LiteLLM, RAGFlow, jetrmm.
- **Doar studiu de design** (AGPL/GPL): breeze, eventual arp-scan-rs (verifică).
- **Niciodată bundle** binare NirSoft (licență + EDR-flag).
