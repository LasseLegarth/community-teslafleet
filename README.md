# community-teslafleet

A self-hosted, open-source gateway that turns a **free Tesla Fleet Telemetry**
stream into:

1. a **local Tesla Fleet API** that [TeslaMate](https://github.com/teslamate-org/teslamate)
   can poll — **for free**, never waking the car, and
2. **Home Assistant MQTT auto-discovery** — your Tesla appears in HA with zero config.

It is the open, self-hosted equivalent of the closed `api.myteslamate.com`.

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

## Quick start (standalone)

1. Run Tesla's [fleet-telemetry](https://github.com/teslamotors/fleet-telemetry)
   with a **zmq** dispatcher: `transmit_decoded_records: true`, route `V` and
   `connectivity` to `zmq`, and set the zmq `addr` to `tcp://0.0.0.0:5284`.
2. Capture a vehicle_data template — see [`templates/README.md`](templates/README.md).
3. `cp config.example.yaml config.yaml`, edit broker/VIN, then:
   ```bash
   docker compose -f docker-compose.example.yml up -d
   ```
4. **TeslaMate**: set `TESLA_API_HOST=http://<gateway-host>:4460` and
   `TESLA_WSS_HOST=ws://<gateway-host>:4460` (same port; the gateway serves both the
   Fleet API and the legacy `/streaming/` WebSocket for near-instant drive
   detection). Keep `TESLA_AUTH_HOST` and your token. Polling is now local and free.

## Configuration

All options are in [`config.example.yaml`](config.example.yaml) and mirrored as
`TGW_*` environment variables (env wins). Minimum: `mqtt.broker`,
`mqtt.topic_base`, one `vehicles[].vin`.

## Status

v1: Fleet API emulation + Home Assistant discovery. Roadmap: command relay
(HA → car), a config UI for per-field stream intervals + enrollment, and a
guided onboarding/token wizard.

## License

MIT — see [LICENSE](LICENSE). Not affiliated with Tesla, Inc.
