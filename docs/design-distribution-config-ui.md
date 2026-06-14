# Design: distribution + onboarding (revideret efter multi-agent review)

Hvordan `community-teslafleet` pakkes så det er **nemmest, bedst og mest robust** at
installere — som **Home Assistant add-on** eller **standalone** (Docker/LXC) — fra
**ét** multi-arch image.

> **Revideret 2026-06-14** efter tre kritiske reviews (HA-add-on, arkitektur,
> OSS-produkt). Hovedændringer vs. v1: (1) **intet custom config-UI** — HA's native
> options-formular + env er nemmere og mere robust; (2) den ene UI vi bygger er en
> **Tesla-onboarding-wizard** (den faktiske friktion); (3) `config.Load` skal
> refaktoreres (ikke "lille"); (4) konkrete HA-manifest-fixes; (5) ærlig
> positionering + connectivity-muligheder (custom port, static key-host, bundlet TLS,
> Cloudflare Tunnel).

## Mål
1. **Ét multi-arch GHCR-image**, der kører **både** som HA add-on og standalone.
2. **Nemmest mulig config**: HA-brugere via HA's native add-on-formular; standalone via
   `docker-compose` + env (valgfri `config.yaml`). Ingen skrøbelig selvbygget config-UI.
3. **Onboarding-wizard** der automatiserer/verificerer det svære (Tesla partner-setup).
4. **Lav connectivity-barriere**: custom telemetry-port, static-hostet public key,
   bundlet auto-TLS, Cloudflare-Tunnel-vej.

## Non-mål
- Custom config/settings-UI (HA-formular + env dækker det robust).
- At fjerne Teslas partner-onboarding (umuligt — vi gør den så nem som platformen tillader).
- Multi-bruger/RBAC.

---

## 1. Hvem er dette til (positionering — vigtigst for en credible release)

Vær ærlig i README + wizard, ellers drukner projektet i "virker ikke på mit LAN":

> **Brug dette hvis:** du self-hoster TeslaMate, vil eje alle data, ikke vil betale en
> hosted service, og **allerede kører offentlig infra** (domæne + en måde at være
> offentligt tilgængelig på).
>
> **Brug noget andet hvis:** du bare vil have Tesla-data i HA nemt → **Teslemetry**
> HA-integration (~$2.50/md, ingen partner-onboarding). Eller hvis du ikke kan være
> offentligt tilgængelig overhovedet.

Den differentierede værdi: **gratis, fuld ejerskab, og bevarer en uændret TeslaMate**
(vi emulerer Fleet API'et) — for power-brugere der allerede har infraen.

---

## 2. Arkitektur: ét image, begge modes

Samme **distroless Go-binary** læser config i denne rækkefølge:
```
defaults → config.yaml (hvis monteret) → /data/options.json (HA add-on) → env TGW_* (override)
```
- **HA add-on**: HA's options-formular skriver `/data/options.json`; binaryen læser den
  direkte (ingen shell/run.sh → forbliver distroless). MQTT auto fra Supervisor.
- **Standalone**: env (compose) eller monteret `config.yaml`.

Add-on-manifest og compose er **tynde wrappers om samme image**.

```
GHCR multi-arch image (gateway + onboarding-wizard, distroless)
├── standalone/  docker-compose.yml (gateway + fleet-telemetry + vehicle-command-proxy + TLS)
└── ha-addon/    config.yaml-manifest (options/schema + ingress-wizard + ports + mqtt:want)
```

---

## 3. Deployment-modes

### 3a. HA add-on (`ha-addon/`)
Config sker i **HA's native options-formular** (gratis, valideret, backup'et). Wizarden
vises kun via **ingress** (til onboarding).

```yaml
# ha-addon/config.yaml (manifest)
name: Community TeslaFleet
version: <skrives af release-workflow = image-tag>
slug: community_teslafleet
image: "ghcr.io/legarth/community-teslafleet"   # ÉT multi-arch manifest (ingen {arch})
arch: [aarch64, amd64]
init: false
hassio_api: true            # KRÆVET for at hente MQTT fra Supervisor (ellers 403)
ingress: true               # onboarding-wizard i sidebaren
ingress_port: 8099
panel_icon: mdi:car-electric
ports:
  "4460/tcp": 4460          # Fleet API/WSS → TeslaMate (kan sættes null hvis HA-only)
ports_description:
  "4460/tcp": "Fleet API + WSS til TeslaMate (LAN-only; sæt null hvis ikke brugt)"
services: [mqtt:want]       # MQTT auto fra Supervisor (want = bloker ikke uden)
options:                    # ← den nemme config-UI, gratis fra HA
  units_system: metric
  telemetry_profile: balanced
  device_identifier: name
  # ... + commands-toggles
schema:
  units_system: "list(metric|imperial)"
  telemetry_profile: "list(eco|balanced|live|custom)"
  device_identifier: "list(name|vin)"
```
Fixes ift. v1 (fra review):
- `hassio_api: true` tilføjet (uden den fejler MQTT-auto med 403).
- **`{arch}` droppet** → ét multi-arch manifest-image.
- **Config i `/data`** (auto, backup'et) — *ikke* `addon_config`/`/config` (v1 forvekslede dem).
- **Root `repository.yaml`** kræves for HA's "add repository" (tilføjes til repo-roden).
- Læs `port` + `ssl` + (valgfri) `username/password` fra `/services/mqtt` — hardcode ikke 1883.
- `host_network` sættes **ikke** (ville bryde ingress-isolationen).

### 3b. Standalone (`docker-compose.yml`)
**Full-stack reference-compose** (review: "uden den er det halvt produkt") — bringer hele
stakken op med én kommando:
- `gateway` (dette image)
- `fleet-telemetry` (Teslas modtager)
- `vehicle-command-proxy` (kommando-signering)
- **TLS**: bundlet Caddy/auto-cert (Let's Encrypt) → ingen separat reverse-proxy nødvendig
- cert/key-generering til onboarding

Config via env (`TGW_*`) eller monteret `config.yaml`. Profiler (HA-only / Full) via flag.

---

## 4. Config-model

### Den nemme vej (ingen custom UI)
- **HA add-on**: HA's options-formular (units, profil, device_identifier, command-toggles).
- **Standalone**: `docker-compose` env eller `config.yaml`.

### `config.Load`-refaktor (review: den reelle risiko — IKKE "lille")
Nuværende `Load` kan ikke round-trippes sikkert. Skal opdeles:
1. **Raw-load** (til redigering/persistering) vs **resolve** (til kørsel) adskilt.
2. **`validate()` må ikke mutere** — i dag udleder den `ID`/`VehicleID` og sætter
   `DisplayName = VIN` (præcis den PII vi undgår). Derivering flyttes ud af persisteret config.
3. **Provenance**: spor hvilke felter der kom fra env (til "read-only/sat via env").
4. **Secrets**: redact-on-read, merge-on-write (et redacted `••••` må ikke nulstille).
5. **Atomic write** (`.tmp` + rename — genbrug token-cache-mønsteret).
6. **Schema-version** i config + migrering-on-load (til upgrades).

### Anvend-strategi (review-fix)
- **Hot-reload de sikre felter**: `units`, `publish_interval`, enrollment, log-level
  (rene funktionelle — `catalog(cfg.Units)` + ticker). Ingen genstart.
- **Genstart kun** for socket-bindende felter (broker, ZMQ-addr, listen-port, vehicles).
- **Add-on-mode**: stol IKKE på "exit → Supervisor genstarter" (det gør den ikke
  pålideligt for en rent-afsluttet add-on). Brug in-process reload.

---

## 5. Tesla onboarding-wizard (den ene UI vi bygger)

Den eneste UI, fordi den gør ting config ikke kan: generere nøgler, probe URL'er, kalde
Tesla-API'er, vise pairing-QR, **verificere hvert trin med grønt flueben**. Bruges **én
gang**. HA add-on: via ingress (HA-autentificeret). Standalone: på port (loopback/password).

### Trin 0 — Connectivity-valg (se §6)
"Hvordan er du offentligt tilgængelig?" → eget domæne+port / Cloudflare Tunnel /
ekstern static-host af key. Resten af wizarden tilpasser sig + prober den valgte vej.

### Trin 1–7
| Trin | Hvem | Wizardens rolle |
|---|---|---|
| 1. Opret dev-app på developer.tesla.com | **Du** | Viser **eksakt** redirect-URI/scopes/domæne; du indsætter client_id+secret |
| 2. Generér EC-nøglepar (prime256v1) | **Wizard** | Laver keypair, gemmer privat nøgle (proxy) |
| 3. Host public key på `/.well-known/.../com.tesla.3p.public-key.pem` | **Du** | Genererer PEM + sti; **prober URL'en** → grønt |
| 4. Registrér partner-account | **Wizard** | Partner-token + `POST /partner_accounts` |
| 5. OAuth refresh-token (cmd-scope) | **Wizard** | Auth-code-flow **eller indsæt-token** (MVP: indsæt); gemmer + roterer |
| 6. Par virtual key pr. bil | **Du** (tap i app) | `tesla.com/_ak/<domæne>`-link **+ QR pr. VIN**; **poller** til paret → grønt pr. bil |
| 7. Enroll telemetry pr. bil | **Wizard** | `POST fleet_telemetry_config` + **poll `synced:true`** → grønt |

> **Ærligt:** wizarden gør Tesla-partner-onboarding **nem og verificeret**, men kan ikke
> fjerne den. README leder med dette (§1). OAuth-redirect under ingress har ingen stabil
> ekstern URL → MVP = indsæt-token; fuld redirect-flow er v2 (kræver public HTTPS alligevel).

---

## 6. Connectivity-muligheder (sænk 443/Caddy-barrieren)

| Behov | Løsning | Note |
|---|---|---|
| **Telemetry-port ≠ 443** | `fleet_telemetry_config.port` er konfigurerbar | bilen ringer ind til `hostname:<port>` (fx 8443) |
| **Ingen egen Caddy til public key** | Host den **statiske** PEM hvor som helst med HTTPS på dit domæne | Cloudflare/GitHub Pages osv. Wizarden prober uanset hvor |
| **Ingen separat reverse-proxy** | **Bundlet auto-TLS** (Caddy/Go-autocert, Let's Encrypt) i reference-stakken | domæne + åben port → cert automatisk |
| **Ingen åbne porte / CGNAT / kan ikke bruge 443** | **Cloudflare Tunnel** (gratis) | offentligt HTTPS-hostname uden port-forward/statisk IP |

**Irreducibelt:** offentligt domæne + offentligt tilgængelig telemetry-endpoint kræves af
Tesla. Kan brugeren slet ikke → README peger på Teslemetry (§1).

---

## 7. Enrollment: intervaller + presets + kost-estimat

(Settes via HA-formularens `telemetry_profile` eller wizardens enrollment-side.)

- **Globale profiler**: Eco/billigst · Balanced (default 1/15/300) · Live · Custom.
- **Per-felt/-gruppe tier** (Custom): Realtid 1s · Responsiv 15s · Normal 60s · Langsom
  300s · Kun-ændringer · Heartbeat · Fra. "Avanceret" viser rå tal.
- **Kost-estimat (review-fixet):**
  - Vis **interval** (ikke falsk-præcist tal): "~X–Y signaler/md afhængig af kørsel".
  - Vis **post-kredit**: "sandsynligvis $0 efter det månedlige kredit" for typiske setups.
  - Kredit = **konfigurerbar konstant** (omstridt $10 vs $14 — flag "verificér hos Tesla").
  - Nævn at **wakes ($1/50) og kommandoer ($1/1000) er separate akser** — estimatet dækker
    kun streaming-signaler.
- Gem → skriv `ftc.json` → `POST /admin/enroll`.

---

## 8. Sikkerhed
- **HA add-on**: ingress er HA-autentificeret. UI-listeneren skal **kun acceptere
  `172.30.32.2`** + alle URL'er prefixes med `X-Ingress-Path` (htmx-faldgrube).
- **Standalone wizard-port**: bind **loopback medmindre password sat** (ikke-valgfrit;
  ikke bare en advarsel). Separat port fra den auth-løse Fleet API.
- **Fleet API/WSS (4460)**: token-løst (TeslaMate-kompat) → **LAN-only, eksponér aldrig**
  (tydelig advarsel i docs).
- Secrets aldrig i klartekst til browseren (redacted, skriv-kun).

---

## 9. Paketering & release

```
community_teslafleet/
├── Dockerfile                  # multi-stage → distroless, embedder wizard-assets
├── docker-compose.yml          # FULL stack: gateway + fleet-telemetry + proxy + TLS + cert-gen
├── repository.yaml             # ← KRÆVET i repo-roden for HA "add repository"
├── ha-addon/
│   ├── config.yaml             # manifest (§3a, med fixes)
│   └── DOCS.md                 # add-on-instruktioner + port-4460-konflikt-note
├── config.example.yaml
├── .github/workflows/release.yml  # buildx multi-arch GHCR + skriv tag i ha-addon/config.yaml
└── README.md                   # §1-positionering + onboarding + connectivity + cost
```
- **Multi-arch** (amd64+aarch64) via `docker buildx`, ét manifest-tag.
- Same-repo `ha-addon/` + root `repository.yaml` → HA tilføjer via repo-URL.

---

## 10. Backup/restore + upgrade (review: mangler for credible release)
- **Backup/restore** af `/data` (config.yaml + token-cache + ftc.json) — export/import-knap
  i wizarden + dokumenteret `/data`-backup. (Tokens/enrollment er smertefulde at genskabe.)
- **Upgrade**: `version`-felt i config + migrering-on-load (§4).
- **Multi-vehicle**: hele modellen er per-VIN (pairing-status + enroll + cost pr. bil).

---

## 11. Implementerings-etaper (revideret, slankere)

1. **`config.Load`-refaktor** (§4) — raw/resolve-split, ikke-muterende validate,
   provenance, atomic write, secrets, options.json-læsning. **Med tests.** Fundament.
2. **HA add-on: native options/schema** (units + telemetry_profile + commands) + manifest-
   fixes (hassio_api, /data, repository.yaml, multi-arch, mqtt port/ssl). → ship HA-config gratis.
3. **Standalone full-stack compose** (gateway + fleet-telemetry + proxy + bundlet TLS + cert-gen).
4. **Onboarding-wizard** (§5) — connectivity-valg + 7 trin med generér/probe/poll/verificér.
   MVP token = indsæt; OAuth-redirect = v2.
5. **Enrollment-side + kost-estimat** (§7) — interval/profiler + post-kredit-estimat.
6. **Release-pipeline** (§9) + README (§1) + backup/restore (§10).

Riskoen ligger i **etape 1** (config-round-trip) og **ingress base-path** i etape 4 — ikke
i UI-rendering. Hver etape er selvstændigt deploybar.

---

## 12. Beslutninger (afgjort efter review)
- **Config-UI**: HA native options-formular + env. **Intet custom config-UI.** ✅
- **Den ene UI**: onboarding-wizard (web, ingress/port, engangs). ✅
- **UI-teknik**: html/template + embed + htmx. ✅
- **Anvend**: hot-reload sikre felter, restart for socket-felter; in-process i add-on. ✅
- **Wizard-port**: separat fra Fleet API; loopback medmindre password. ✅
- **Add-on-repo**: i samme repo (`ha-addon/` + root `repository.yaml`). ✅
- **Token MVP**: indsæt-token; fuld OAuth-redirect = v2. ✅
- **Image**: ét multi-arch manifest (ingen `{arch}`). ✅
