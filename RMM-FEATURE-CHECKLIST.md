# RMM feature checklist & gap analysis — Watchclaw vs. industry bar

> Sursa: documentul Mihai „Remote Monitoring and Management.docx" (2026-06-02) — capabilități standard RMM/MSP + Top 100 tool-uri + checklist.
> Legendă: ✅ avem · 🟡 parțial · ❌ lipsește. Wedge-ul nostru = vizibilitate+securitate suveran; restul e backlog prioritizat.

## RMM / endpoint management (bara din doc, linia „RMM decent trebuie să aibă")
| Feature | Status Watchclaw |
|---|---|
| Agent Windows/macOS/Linux | 🟡 Windows + Linux (Rust); **macOS TODO** |
| Asset inventory (HW/serial/software) | ✅ profund (USB/seriale/software/servicii/SMART) |
| Health monitoring | ✅ (CPU/RAM/disk + praguri) |
| **Agentless credentialed scan** | ✅ WinRM (139 software/118 svc dovedit) — *peste mulți competitori* |
| Patch management OS + third-party | 🟡 winget **detectează** update-uri; **deploy ❌** |
| Software deployment | ❌ |
| Scripting PowerShell/Bash | ❌ |
| Automation policies / self-healing | ❌ |
| **Remote access** (unattended/attended) | ❌ (plan: MeshCentral + RustDesk — validat de doc) |
| File/process/service manager (live) | 🟡 inventar da; management live ❌ |
| Registry/terminal access | ❌ |
| Alerting + ticket auto-gen | ✅ alerting (resurse+offline); 🟡 „tickets" = alerte, PSA ❌ |
| SNMP / network discovery | 🟡 discovery 4-strat ✅; SNMP deep researched, neimpl |
| **Topology maps** | ✅ UniFi-grade (hub+iconițe+Connections layer) |
| Reporting / executive reports | ❌ |
| Multi-tenant management | ❌ (researched) |
| RBAC / audit logs | ❌ |
| 2FA/SSO | ❌ |
| API / webhooks | 🟡 REST API da; webhooks ❌ |
| PSA integration | ❌ |

## Verdict onest
**Suntem puternici pe VIZIBILITATE** (monitor + inventar profund + agentless + topologie + alerting) — chiar peste default-ul multor RMM-uri pe adâncimea datelor.
**Slabi pe ACȚIUNE** (remote access, scripting, patch/software deploy, self-heal) și pe **MANAGEMENT** (multi-tenant, RBAC, PSA, reporting). Astea două definesc „RMM complet".

## Ce validează documentul
- **Shortlist self-hosted din doc:** „Tactical RMM + **MeshCentral/RustDesk** + Hudu self-hosted + Comet Backup + Zabbix/Checkmk" → exact direcția noastră (remote = MeshCentral+RustDesk, self-hosted, suveran). Construim alternativa **integrată + EU-suverană** la acel stack DIY.
- Atera (flat per-tech pricing) + agentless = modelul pe care-l țintim.

## Prioritizare propusă (peste wedge-ul de vizibilitate)
1. **Remote access** (MeshCentral + RustDesk) — cel mai mare gap „RMM real"; fără el nu ești RMM.
2. **Scripting/remediation** (rulează PS/Bash remote — avem deja WinRM!) + **self-healing** (alertă→script).
3. **Patch deploy** (winget install remote — avem deja detect).
4. **Multi-tenant + RBAC + audit** (fundație vandabilă).
5. **Reporting** (executive PDF).
PSA/ticketing complet = mai târziu (sau integrare cu unul existent, nu-l reconstruim).

## Top 100 (referință competitivă) — păstrată în doc-ul sursă
NinjaOne, Datto, Kaseya, N-able, ConnectWise, Atera, Syncro, SuperOps, Action1, Auvik, Domotz... + categorii PSA/BDR/EDR/IAM/docs.
