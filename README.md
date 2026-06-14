# community-teslafleet

**Bring your Tesla into Home Assistant — fully, for free, self-hosted.**

A small self-hosted gateway that turns a free Tesla **Fleet Telemetry** stream into
**Home Assistant MQTT auto-discovery**. Every signal your car emits shows up in HA as
proper entities — sensors, binary sensors, **covers** (frunk/trunk/charge port/windows),
**climate**, **device_tracker** (live GPS / zones) — plus full **command control**:
charging, climate, locks, trunk/frunk, windows, seat heaters, sentry, navigation,
software updates and more. It **never wakes the car** and costs nothing beyond Tesla's
free monthly API credit.

> **Also for TeslaMate users.** The gateway serves a local Tesla **Fleet API + streaming
> WebSocket**, so an unmodified [TeslaMate](https://github.com/teslamate-org/teslamate)
> keeps logging — polling *you* (free, local, and Sentry-mode no longer matters) instead
> of Tesla's billed API.

One container. Install it as a **Home Assistant add-on** or run it **standalone** (Docker/LXC).

## Who is this for

- **Home Assistant users** who want their Tesla in HA — all data *and* full control —
  self-hosted, free, owning their own data (no monthly subscription).
- **TeslaMate users** especially: keep TeslaMate running on the Fleet API without paying
  for polling, and without Sentry-mode keeping the car awake and racking up a bill.

Tesla requires a public domain + a publicly-reachable telemetry endpoint. The built-in
**onboarding wizard** makes that as easy as the platform allows (it generates keys,
verifies hosting, registers your partner account, and walks you through pairing). If you
can't host that — or just want the absolute easiest path — a hosted service like
[Teslemetry](https://teslemetry.com) (~$2.50/mo) is fine; **this project is for
self-hosters who want free + full ownership.**

## How it works

```
   Tesla car ──mTLS──▶ fleet-telemetry ──ZMQ──▶ community-teslafleet
                                     (brokerless)        │
        ┌───────────────────────────────────────────────┤
        ▼                                                ▼
  Home Assistant  ◀── MQTT auto-discovery        Fleet API + WSS ──▶ TeslaMate
  (all entities + commands)                      (free local polling, optional)
```

Tesla bills `vehicle_data` polling, and TeslaMate keeps a **Sentry-on** car awake and
polling forever — so the bill climbs. Fleet Telemetry *streaming* is essentially free.
`community-teslafleet` consumes that stream once and re-serves it to Home Assistant
(MQTT) and, optionally, to TeslaMate (a local Fleet API it polls for free). Ingest is
**brokerless** (ZMQ) — no MQTT broker to run; the gateway is only an MQTT *client* when
publishing to Home Assistant's broker.

## Profiles

- **HA-only** (default) — just publishes your Tesla to Home Assistant.
- **Full** — also serves TeslaMate (`fleetapi.enabled: true`); point TeslaMate at
  `TESLA_API_HOST=http://<gateway-host>:4460`.

## Requirements

**Always required** (the Tesla side — needed no matter how you use it):

| | Why |
|---|---|
| **A Tesla developer app** | free at [developer.tesla.com](https://developer.tesla.com) → `client_id` + `client_secret`. |
| **A public domain with HTTPS** | Tesla fetches your partner public key at `https://<domain>/.well-known/...` and the car connects to a TLS telemetry hostname. |
| **A publicly-reachable telemetry endpoint** | the car dials *in* — a public port (any port, not just 443), or a **Cloudflare Tunnel** if you can't open ports / are behind CGNAT. |
| **A Fleet-Telemetry-capable Tesla** | most vehicles on recent firmware (≈2021+). One car or many. |
| **A place to run it** | Docker/LXC (standalone — bundles `fleet-telemetry` + `vehicle-command-proxy`) **or** HA OS/Supervised (the add-on). |

**Then pick what you want to feed — at least one:**

| You want… | You also need |
|---|---|
| **Home Assistant integration** | Home Assistant + an MQTT broker (Mosquitto). The add-on auto-detects HA's broker; standalone points at any broker. *HA is only required for this — not for TeslaMate-only setups.* |
| **TeslaMate logging (no HA)** | a running [TeslaMate](https://github.com/teslamate-org/teslamate); point `TESLA_API_HOST`/`TESLA_WSS_HOST` at the gateway. No HA or MQTT needed. |

So: run it **standalone for TeslaMate only**, **as an HA add-on for Home Assistant only**,
or **both at once**.

> The domain + public reachability are Tesla's requirements, not this project's — they
> can't be removed, only eased (the onboarding wizard + the connectivity options below).
> No public infra? A hosted service like [Teslemetry](https://teslemetry.com) is simpler.

## Install

### Home Assistant add-on (recommended)
Settings → Add-ons → ⋮ → Repositories → add
`https://github.com/LasseLegarth/community-teslafleet`, install **Community TeslaFleet**,
and configure it in the **Configuration** tab. The MQTT broker is auto-detected from
Home Assistant. Requires HA OS / Supervised. (HA Container/Core → use standalone below.)

### Standalone (Docker/LXC)
Brings up `fleet-telemetry` + `vehicle-command-proxy` + the gateway:
```bash
cp .env.example .env        # fill in domain, VIN, HA broker
cp fleet-telemetry-config.example.json fleet-telemetry-config.json
docker compose up -d
```
Then run the onboarding wizard (`TGW_ONBOARD_ENABLED=true`, on `:8099`).

**TeslaMate** (if used): `TESLA_API_HOST=http://<gateway-host>:4460` and
`TESLA_WSS_HOST=ws://<gateway-host>:4460` (one port serves the Fleet API and the legacy
`/streaming/` WebSocket). Keep `TESLA_AUTH_HOST` + your token. Polling is now local + free.

## Onboarding wizard

A guided wizard handles the hard Tesla partner setup (HA add-on: in the sidebar via
ingress; standalone: `:8099`, set `TGW_ONBOARD_PASSWORD`). It generates the signing
keypair, renders the `.well-known` public key and **verifies it's reachable**, registers
the partner account, saves your refresh token, lists vehicles + shows the per-car pairing
link, lets you pick a stream **profile** (eco/balanced/live) with a cost estimate, and
enrolls telemetry.

## Connectivity (you don't need Caddy or port 443)

- **Custom port** — set `TELEMETRY_PORT` (e.g. `8443`); the car connects to `host:port`.
- **TLS, no open ports** — get the cert via **certbot DNS-01** (no HTTP challenge).
- **`.well-known` key** is a static file — host it on any HTTPS host on your domain.
- **No open ports / CGNAT** — put the telemetry endpoint behind a **Cloudflare Tunnel**.

## Units & privacy

Pick `metric` or `imperial` (`units.system`) — HA shows km/°C/bar or mi/°F/psi. The VIN
stays out of entity_ids (`device_identifier: name` → `sensor.tesla_<name>_*`); it's only
the device serial number.

## Backup & restore

Everything stateful lives in **`/data`** (config, token, `ftc.json`, onboarding keys).
HA add-on backups include it automatically; standalone — back up the `./data` volume.

## License

MIT — see [LICENSE](LICENSE). Not affiliated with Tesla, Inc.
