# community-teslafleet

A self-hosted, open-source gateway that turns a **free Tesla Fleet Telemetry**
stream into:

1. a **local Tesla Fleet API** that [TeslaMate](https://github.com/teslamate-org/teslamate)
   can poll — **for free**, never waking the car, and
2. **Home Assistant MQTT auto-discovery** — your Tesla appears in HA with zero config.

It is the open, self-hosted equivalent of the closed `api.myteslamate.com`.

## Who is this for

**Use this if** you self-host TeslaMate, want to own all your data, won't pay a
hosted service, and **already run public-facing infra** (a domain + a way to be
publicly reachable). Tesla requires a public domain and a publicly-reachable
telemetry endpoint — that part can't be removed, only made as easy as the platform
allows (this project includes a guided onboarding wizard).

**Use something else if** you just want easy Tesla data in HA → the
[Teslemetry](https://teslemetry.com) integration (~$2.50/mo, no partner onboarding),
or you can't be publicly reachable at all.

## Why

Tesla's Fleet API bills `vehicle_data` polling, and TeslaMate refuses to let a
**Sentry-on** car sleep — so it polls forever and the bill climbs. Fleet
Telemetry streaming is essentially free, but TeslaMate can only ingest ~13 fields
over the legacy streaming protocol, and Home Assistant needs polling or a bridge.

`community-teslafleet` consumes the telemetry stream once and re-serves it as a
local Fleet API (so TeslaMate polls *you*, free, Sentry irrelevant) and as MQTT
for Home Assistant.

```
            Tesla car ──mTLS──▶ fleet-telemetry ──ZMQ──▶ community-teslafleet
                                              (brokerless)     │
                          Fleet API (HTTP)  ◀── TeslaMate ─────┤  (free, local polling)
                          MQTT auto-discovery ──▶ Home Assistant (gateway is an MQTT client)
```

Ingest from fleet-telemetry is **brokerless** (ZMQ) — no MQTT broker to run. The
gateway is only an MQTT *client* when publishing to Home Assistant's broker.

## Profiles

- **HA-only** — `fleetapi.enabled: false`. Just publishes your Tesla to Home
  Assistant. Also packaged as a **Home Assistant add-on** (`ha-addon/`).
- **Full** — `fleetapi.enabled: true`. Also serves TeslaMate; set TeslaMate's
  `TESLA_API_HOST=http://<gateway-host>:4460`.

## Install

### Home Assistant add-on (easiest for HA users)
Add this repo as an add-on repository (Settings → Add-ons → ⋮ → Repositories →
`https://github.com/LasseLegarth/community-teslafleet`), install **Community TeslaFleet**,
and configure it in the **Configuration** tab. The MQTT broker is auto-detected from
HA. Requires HA OS / Supervised. (HA Container/Core → use standalone below.)

### Standalone — full stack (`docker-compose.yml`)
Brings up `fleet-telemetry` + `vehicle-command-proxy` + the gateway:
```bash
cp .env.example .env        # fill in domain, VIN, HA broker
cp fleet-telemetry-config.example.json fleet-telemetry-config.json
docker compose up -d
```
Then run the **onboarding wizard** (token, partner key, per-car pairing) — see below.

**TeslaMate**: set `TESLA_API_HOST=http://<gateway-host>:4460` and
`TESLA_WSS_HOST=ws://<gateway-host>:4460` (same port serves the Fleet API and the
legacy `/streaming/` WebSocket). Keep `TESLA_AUTH_HOST` + your token. Polling is now
local and free.

## Connectivity (lowering the public-HTTPS barrier)

Tesla needs a public domain + a reachable telemetry endpoint. You don't need to run
Caddy or use port 443:

- **Custom port** — set `TELEMETRY_PORT` (e.g. `8443`); the car connects to `host:port`.
- **TLS with no open ports** — obtain the telemetry cert via **certbot DNS-01** (no
  HTTP challenge), e.g.:
  ```bash
  docker run --rm -v "$PWD/certs:/out" -v "$PWD/cf.ini:/cf.ini:ro" \
    certbot/dns-cloudflare certonly --dns-cloudflare \
    --dns-cloudflare-credentials /cf.ini -d telemetry.example.com \
    --config-dir /out --work-dir /out --logs-dir /out
  ```
  …then point `./certs/{fullchain,privkey}.pem` at the issued files.
- **The `.well-known` public key** is a static file — host it on **any** HTTPS host on
  your domain (Cloudflare/GitHub Pages), not necessarily this server.
- **No open ports / CGNAT** — put the telemetry endpoint behind a **Cloudflare Tunnel**.

## Configuration

All options are in [`config.example.yaml`](config.example.yaml) and mirrored as
`TGW_*` environment variables (env wins). Minimum: `mqtt.broker`,
`mqtt.topic_base`, one `vehicles[].vin`.

## Onboarding wizard

A guided wizard handles the hard Tesla partner setup. Enable it (`TGW_ONBOARD_ENABLED=true`,
served on `:8099`; in the HA add-on it appears in the sidebar via ingress). It:
generates the signing keypair, renders the `.well-known` public key and **verifies it's
reachable**, registers the partner account, saves your refresh token, lists vehicles +
shows the per-car pairing link, lets you pick a stream **profile** (eco/balanced/live)
with a cost estimate, and enrolls telemetry. Standalone: set `TGW_ONBOARD_PASSWORD` (the
UI holds keys/tokens — it binds with basic-auth; never expose it unauthenticated).

## Backup & restore

Everything stateful lives in **`/data`**: `config.yaml`, the rotating refresh token,
`ftc.json`, and `onboard/` (keys + progress). Back up that one directory.
- **HA add-on**: `/data` is included in Home Assistant's add-on backups automatically.
- **Standalone**: back up the `./data` volume (e.g. `tar czf data-backup.tgz ./data`).
Restore = drop the directory back and restart.

## Status

Working: Fleet API + WSS emulation (TeslaMate), Home Assistant discovery for all
streamed telemetry on proper HA domains (sensors, covers, climate, device_tracker),
metric/imperial units, and a full command relay (HA → car: charging, climate, covers,
seats, navigation, software update, …). One image runs as an HA add-on or standalone.

Roadmap: a guided **onboarding wizard** (partner key, OAuth token, per-car virtual-key
pairing, telemetry enrollment) and a per-field stream-interval + cost-estimate UI.
See [`docs/design-distribution-config-ui.md`](docs/design-distribution-config-ui.md).

## License

MIT — see [LICENSE](LICENSE). Not affiliated with Tesla, Inc.
