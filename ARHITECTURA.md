# Arhitectură & Stack — RMM European (nume de lucru: Watchclaw)

> Proiect NOU, de la zero. NU refolosește MeterMind/copier-cloud (acelea au fost teste).
> Principiu: **limbajul potrivit fiecărei componente** — fiecare alegere justificată, nu mono-limbaj forțat, nici buzzword-uri sci-fi.

## Decizii de limbaj (CTO)

| Componentă | Limbaj | De ce ACEST limbaj (nu altul) |
|---|---|---|
| **`agent/`** — agent de endpoint | **Rust** | Rulează cu privilegii pe mii de mașini ale clienților. Memory-safety (fără clase întregi de CVE-uri), **single static binary** fără runtime/GC, footprint mic, fără pauze GC. Pentru software de sistem pe endpoint terț, Rust e alegerea de securitate + fiabilitate. Go ar fi mai rapid de scris, dar agentul e exact locul unde siguranța > viteza de dev. |
| **`control-plane/`** — API, ingestion telemetrie, services | **Go** | Concurență ieftină (goroutines) pentru mii de conexiuni de agenți simultan, throughput mare la ingestion, ecosistem cloud-native matur (gRPC, k8s, OpenTelemetry), compilare rapidă, deploy ca binar. Sweet-spot pentru servicii de rețea cu I/O intens. |
| **`web/`** — dashboard / consolă RMM | **TypeScript + Next.js** | Standard de facto pentru console SaaS complexe; SSR pentru încărcare rapidă, ecosistem de componente, type-safety pe frontend. |
| **`ai/`** — copilot, RAG, automation, NL→query | **Python** | Ecosistemul AI/ML trăiește în Python; integrare directă cu LiteLLM, biblioteci RAG/embeddings. Izolat ca serviciu, comunică prin API. |

## Infrastructură (nu „limbaj", dar parte din stack)

| Rol | Tehnologie | De ce |
|---|---|---|
| Time-series (metrici endpoint) | **TimescaleDB** | Postgres + hypertables; SQL familiar, compresie, retenție automată |
| Relational (PSA, tenants, billing, CMDB) | **PostgreSQL** | tranzacțional, matur, suveran |
| Message bus (telemetrie real-time) | **NATS** | lightweight, Go-native, mult mai simplu de operat decât Kafka la start |
| Remote desktop | **MeshCentral + RustDesk** (combinat — vezi mai jos) | management + remote premium, orchestrate de agentul nostru |
| Secrets | **HashiCorp Vault** | suveran, self-hostable |
| Auth / zero-trust | **Authelia** | suveran |

## Remote access — model combinat (decizie 2026-06-01, opțiunea A)
NU fuziune de cod; **orchestrare la nivel de produs**. Cele două sunt complementare:
- **MeshCentral (Apache 2.0)** = coloana de **management** (terminal, file transfer, scripting, inventar, Intel AMT, grupare). 90% din munca zilnică. Integrat strâns, modificabil liber.
- **RustDesk (AGPLv3)** = transport de **remote desktop premium** (AV1/H265, P2P <60ms), lansat **on-demand** prin server self-host propriu, **nemodificat** → zero friction AGPL. Dacă vom vrea rebranding profund → RustDesk Server Pro (comercial).
- **Agentul Watchclaw (Rust)** = orchestratorul de pe endpoint: singurul serviciu permanent; ține mesh agent pt management și lansează clientul RustDesk *portabil, efemer* la cerere. UI-ul nostru = un singur buton „Conectează", noi alegem transportul.
- Capcană gestionată: footprint dublu evitat (un agent „greu" + restul on-demand).

## Comunicare între componente
- Agent (Rust) → ingestion (Go): mTLS, payload compact (protobuf sau JSON la start), store-and-forward local pentru rezistență la căderi de rețea.
- Ingestion (Go) → NATS → persistors → TimescaleDB/PostgreSQL.
- Web (Next.js) → API (Go) prin REST/gRPC-web.
- AI (Python) ← API (Go) pentru context; răspunde prin endpoint dedicat.

## Structura monorepo
```
rmm-europe/
├── ARHITECTURA.md          # acest fișier
├── STRATEGIE-FONDARE.md    # teza de business / moat / GTM
├── agent/                  # Rust — agent de endpoint
├── control-plane/          # Go — API + ingestion + services
├── web/                    # TypeScript/Next.js — dashboard
├── ai/                     # Python — copilot/automation
└── infra/                  # docker-compose, migrations, deploy
```

## Principiu de suveranitate (moat)
Control plane regional, datele EU nu părăsesc regiunea EU. Self-hostable + SaaS (open-core).
Vezi `STRATEGIE-FONDARE.md` pentru de ce asta bate Atera/NinjaOne (NIS2 + US CLOUD Act).
