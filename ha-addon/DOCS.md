# Community TeslaFleet — Home Assistant add-on

Publishes your Tesla into Home Assistant via MQTT auto-discovery, fed by a
self-hosted Tesla Fleet Telemetry stream. Optionally also serves a local Tesla
Fleet API for TeslaMate.

> **Status: scaffold / WIP.** The standalone Docker image is the primary supported
> path today (see the repo README). This add-on wrapper maps add-on options to the
> gateway's `TGW_*` env vars via `run.sh` and is intended for the HA-only profile.

## Prerequisites

- Tesla [fleet-telemetry](https://github.com/teslamotors/fleet-telemetry) running
  with an MQTT sink (route `V` and `connectivity` to `mqtt`, `transmit_decoded_records: true`).
- An MQTT broker reachable from HA (e.g. the Mosquitto add-on, `core-mosquitto`).

## Options

| Option | Meaning |
|---|---|
| `mqtt_broker` | Broker to CONSUME telemetry from |
| `mqtt_topic_base` | Must match fleet-telemetry `namespace` |
| `fleetapi_enabled` | `true` only if you also run TeslaMate |
| `ha_broker` | Broker to PUBLISH HA discovery to (usually the same) |
| `vins` | Comma-separated VIN list |
| `range_input` / `speed_input` / `odometer_input` | Units your telemetry emits |

Captured `vehicle_data` templates (optional, improves Fleet API fidelity) go in
`/config/community_teslafleet/templates/<VIN>.json`.
