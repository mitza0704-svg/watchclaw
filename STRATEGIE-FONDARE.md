# RMM European — Teza de Fondare
## „Suveranul" — RMM-ul #1 din Europa, cu cap global

> Document de strategie v0.1 · 2026-06-01 · nume de lucru, brand de confirmat (candidat: Watchclaw)
> Autor: CEO/CTO/CFO (Claude) pe baza experienței de tehnician IT a lui Mihai Zamfir
> Status: TEZĂ — necesită input experiențial Mihai pe secțiunea wedge

---

## 0. Teza într-o frază

> **Reglementarea europeană (NIS2, DORA) transformă suveranitatea datelor dintr-un „nice-to-have" într-o obligație legală cu amenzi de €10M — exact în 2026 — iar liderii de piață (Atera, NinjaOne) sunt structural americani și nu pot rezolva asta. Construim RMM-ul nativ-suveran european care le ia segmentul reglementat, apoi ne extindem global cu același avantaj de încredere.**

Nu concurăm pe „mai multe feature-uri". Concurăm pe **suveranitate + preț predictibil + automatizare reală + AI inclus** — fix cele 4 dureri pe care piața le urăște la incumbents.

---

## 1. De ce ACUM (why now) — fereastra care se deschide

Trei forțe converg în 2026. Aici e tot jocul.

### a) NIS2 — deadline de conformitate octombrie 2026
- NIS2 reglementează **direct MSP-urile** ca furnizori ICT critici.
- Se aplică oricui operează în EU sau **servește clienți EU** peste prag (50+ angajați SAU €10M cifră).
- Amenzi: **până la €10M sau 2% din cifra globală**.
- Crucial: NIS2 face organizația răspunzătoare de **suveranitatea întregului lanț de furnizori**. Adică: dacă MSP-ul tău folosește un RMM american, expunerea aia devine *problema lui de conformitate*.

### b) US CLOUD Act + FISA — vulnerabilitatea structurală a incumbents
- Hyperscalerii US controlează **>70% din cloud-ul EU** și sunt supuși legilor extrateritoriale US (CLOUD Act, FISA) — autoritățile US pot cere acces la date **indiferent unde sunt serverele fizic**.
- Atera (HQ Israel/US), NinjaOne (US) — chiar dacă pun date în EU, **entitatea juridică e expusă**. Nu pot rezolva asta fără să-și mute sediul. Nu o vor face.
- **Asta nu se poate copia.** E un moat juridic, nu tehnic.

### c) Open-source RMM a ajuns matur
- În 2026, self-hosting „nu mai e compromis, e alegere operațională legitimă" (Tactical RMM, MeshCentral).
- Înseamnă că **nu pornim de la zero pe partgrea** (agent, remote desktop) — accelerăm pe fundație existentă (cu atenție la licențe — vezi §6).

**Concluzia:** fereastra „EU-sovereign RMM" e deschisă acum și se închide când incumbents reacționează sau când apare alt challenger european finanțat. Avantajul primului care construiește credibil = enorm.

---

## 2. Durerea reală a pieței (din r/msp + analize 2026)

Ce urăsc tehnicienii și MSP-urile la lideri — astea sunt wedge-urile noastre:

| Durere | Atera | NinjaOne | Soluția noastră |
|---|---|---|---|
| **Preț imprevizibil la scară** | per-tehnician, urcă cu echipa | per-device, urcă mai repede ca venitul | **Preț predictibil, plafonat, fără surprize** |
| **Add-on tax** | AI = add-on separat | backup/remote/monitoring se adună | **Totul inclus, un singur preț** |
| **Lipsă PSA nativ** | superficial | inexistent | **PSA nativ de la bază** |
| **Automatizare/alerting prost** | fără conditional logic, doar praguri | complex de învățat | **Automatizare cu logică reală, AI-assisted** |
| **Suveranitate** | expus CLOUD Act | expus CLOUD Act | **EU-sovereign by design** |
| **MFA/UX fricțiune** | — | logout-uri dese | **Sesiuni lungi, UX de tehnician** |

Aici intră experiența ta de tehnician: tu ai trăit durerile astea. Lista de mai sus e din research; **lista REALĂ o validezi tu** (vezi §11).

---

## 3. Ce SUNTEM și ce NU suntem

**SUNTEM:** RMM+PSA unificat, EU-sovereign, AI-native, preț predictibil, construit de un tehnician pentru tehnicieni. Open-core (transparență = încredere, parte din pitch-ul de suveranitate).

**NU SUNTEM:** încă-un-RMM-generalist care încearcă paritate feature-cu-feature. Nu vindem „lățime", vindem **încredere + simplitate + cost previzibil**.

---

## 4. Arhitectura tehnică pentru scară planetară (viziune CTO)

Proiectată din ziua 1 pentru multi-region și suveranitate, nu retrofitat.

```
                    ┌─────────────────────────────────────────┐
                    │   CONTROL PLANE (per-regiune suverană)    │
                    │   EU-Central (RO/DE) · EU-West · ...       │
                    │   Date NU traversează granițe regionale   │
                    └─────────────────────────────────────────┘
                                      │
        ┌─────────────┬───────────────┼───────────────┬─────────────┐
        ▼             ▼               ▼               ▼             ▼
   ┌─────────┐  ┌──────────┐   ┌────────────┐  ┌──────────┐  ┌──────────┐
   │  Agent  │  │ Remote   │   │ Automation │  │   PSA    │  │ AI Copilot│
   │ (Rust/  │  │ (Mesh-   │   │ + Patch    │  │ (nativ)  │  │ (LiteLLM  │
   │  Go)    │  │ Central) │   │ engine     │  │          │  │ multi-mdl)│
   └─────────┘  └──────────┘   └────────────┘  └──────────┘  └──────────┘
        │                                                          │
   Telemetrie → time-series DB        RAG pe tichete/KB → vector DB │
```

**Decizii de arhitectură:**
- **Agent:** Rust sau Go, single binary, multi-OS (Win/macOS/Linux), low-footprint. Baza = experiența MeterMind (agent Windows deja existent — pattern reutilizabil, nu fuziune de produs).
- **Suveranitate by design:** control plane regional izolat. Datele unui client EU nu părăsesc regiunea EU. Asta e *feature-ul de vânzare*, nu o constrângere.
- **Remote:** MeshCentral (Apache 2.0 — licență liberă) ca strat de remote desktop/shell. Nu reinventăm asta.
- **AI-native:** copilot inclus din bază (nu add-on) — sumarizare tichete, scripting asistat, NL→query, triere alerte cu logică. Costul tău e fracțiune (LiteLLM multi-model, rutezi pe modele ieftine/locale).
- **Self-hostable + SaaS:** open-core. Clientul paranoid de suveranitate poate self-host; restul iau SaaS-ul nostru EU. Ambele = venit (suport/licență enterprise).

---

## 5. Stiva (pragmatică, nu sci-fi)

| Strat | Tehnologie | De ce |
|---|---|---|
| Agent | Rust/Go single-binary | footprint mic, cross-platform, fără runtime |
| Backend | (de decis: Go sau Laravel) | viteză vs. ecosistem |
| Time-series | TimescaleDB | metrici device |
| Relational | PostgreSQL | PSA, tenants, billing |
| Vector | Qdrant | RAG copilot |
| Remote | MeshCentral | Apache 2.0, matur |
| AI | LiteLLM multi-model | cost-routing, modele locale/cloud |
| Auth | Authelia / zero-trust | suveran, self-hostable |
| Multi-tenancy | pattern row-level scoping | (pattern tehnic reutilizabil din experiență internă) |

---

## 6. ⚠️ Capcana de licență (decizie CTO critică)

**Tactical RMM NU e liber de revândut ca SaaS.** Licența lui restrânge oferirea comercială ca serviciu concurent. Deci:
- ❌ Nu luăm Tactical RMM și-l revindem SaaS — risc juridic.
- ✅ Folosim **MeshCentral (Apache 2.0)** pentru remote — liber.
- ✅ Ne **inspirăm** din arhitectura Tactical (Django+Vue+agent Go) dar scriem agentul + control plane proprii.
- ✅ Sau: negociem licență comercială Tactical pentru accelerare, dacă ROI-ul o cere.

Aceasta e o decizie de fondare — o validăm înainte de prima linie de cod.

---

## 7. Model economic (cum batem durerea de preț)

Principiu: **preț predictibil, totul inclus, fără add-on tax.**

- Tier-uri simple, plafonate (ex: per-tehnician cu endpoint-uri generoase incluse, SAU flat per-tenant).
- AI inclus (nu separat ca Atera).
- PSA inclus (nu lipsește ca NinjaOne).
- **Argument de vânzare #1:** „Factura ta nu va crește surprinzător luna viitoare." MSP-urile urăsc imprevizibilitatea mai mult decât prețul.
- Suveranitate ca **premium justificat** pentru segmentul reglementat (finanțe/sănătate/public sub NIS2/DORA).

CFO note: costul nostru marginal real = infra (mic, open-source) + AI tokens (fracțiune via LiteLLM cost-routing + Ollama local pentru task-uri ieftine). Marjă structural mare dacă scalăm pe SaaS.

---

## 8. Go-to-market: Land Europa → Expand global

**Fază geografică:**
1. **România** (teren propriu, primul client validat, cost-test gratis pe infra proprie)
2. **DACH + EU reglementat** (Germania = piață RMM mare + obsedată de conformitate = ICP perfect pt suveranitate)
3. **EU-wide** (UK, FR, Nordics)
4. **Global** (US, APAC) — dar cu un twist: vindem „EU-grade sovereignty" și companiilor non-EU care vor să servească clienți EU (NIS2 extrateritorial)

**Canal:**
- **Founder-led + product-led.** Tu, tehnician, vinzi altor tehnicieni. Credibilitate de breaslă.
- Conținut tehnic (cum am scăpat de durerea X), comunitate r/msp, open-core ca magnet de încredere.
- Parteneri: integratori de conformitate NIS2 (ei caută unelte suverane pentru clienții lor).

---

## 9. Fazare cu milestone-uri (realist, vandabil la fiecare pas)

| Fază | Țintă | DoD | Venit posibil |
|---|---|---|---|
| **F0 — Wedge** | agent telemetrie + dashboard + alerting pe infra ta + 1-2 clienți | rulează pe sisteme reale, nu Acme Corp | primul €/lună |
| **F1 — RMM core** | remote (MeshCentral) + patch + automation cu logică | înlocuiește un RMM plătit la un client real | înlocuire incumbent |
| **F2 — PSA + AI** | ticketing nativ + copilot inclus | un tehnician își rulează ziua întreagă în el | upsell |
| **F3 — Suveran multi-region** | control plane regional + pitch NIS2 | primul client „cumpără pentru conformitate" | premium reglementat |
| **F4 — Scale EU** | self-host + SaaS, onboarding, billing | 10+ tenants plătitori | recurring serios |
| **F5 — Global** | regiuni non-EU, „EU-grade" ca brand | clienți internaționali | scalare |

**Nu** construim F3-F5 înainte de F0-F1. Fiecare fază trebuie să facă bani sau să dovedească ceva înainte de următoarea.

---

## 10. Riscuri (onest)

1. **Resurse:** bootstrap contra companii finanțate. Mitigare: wedge îngust + open-core + founder-led, nu război total.
2. **Licențe open-source** (§6) — risc juridic dacă greșim fundația. Mitigare: MeshCentral Apache + cod propriu.
3. **Trust/securitate:** un RMM compromis = coșmar (acces la toate endpoint-urile clienților). Securitatea NU e opțională — e produsul. Audit din ziua 1.
4. **Conformitate noi înșine:** ca să vindem suveranitate, TREBUIE să fim conformi (ISO 27001, SOC2 path). Cost real, dar e și moat.
5. **Timpul lui Mihai** — singura resursă rară. De aceea fazăm strict.

---

## 11. Ce am nevoie de la TINE (experiență de tehnician)

Strategia de mai sus e fundamentată pe piață + reglementare. **Ce-i lipsește e stratul tău experiențial** — și ăla face diferența între un RMM generic și unul pe care tehnicienii îl iubesc:

1. **Ce folosești ACUM** ca să administrezi sisteme? (RMM plătit / Tactical / RDP+scripturi+Excel)
2. **Care e durerea ta #1 zilnică** — momentul în care înjuri unealta actuală?
3. **Ce vertical / tip de client** administrezi cel mai des? (e candidatul de wedge inițial)
4. **Ce-ai construi PRIMA** dacă ai avea unealta ideală — feature-ul care ți-ar schimba ziua?

Răspunsurile astea decid F0. Nu construim ce-a halucinat un LLM; construim ce-ți lipsește ȚIE, pentru că tu ești primul ICP.
