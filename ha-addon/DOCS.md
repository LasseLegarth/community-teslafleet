# Community TeslaFleet — Home Assistant add-on

Publishes your Tesla into Home Assistant via MQTT auto-discovery, fed by a
self-hosted Tesla Fleet Telemetry stream. Optionally also serves a local Tesla
Fleet API + WSS for TeslaMate (free polling that never wakes the car).

This add-on runs the **same image** as the standalone Docker build; the gateway reads
the add-on options directly. Configure everything in the **Configuration** tab.

## Prerequisites

- A self-hosted Tesla [fleet-telemetry](https://github.com/teslamotors/fleet-telemetry)
  server (it streams to the gateway over **brokerless ZMQ**), reachable from this add-on.
- HA's MQTT integration / a broker — **auto-detected** from the Supervisor, so you
  normally don't enter broker details.
- Tesla partner onboarding (public domain, key, virtual-key pairing). See the repo
  README — this is the real setup work; the gateway can't remove it.

## Options

| Option | Meaning |
|---|---|
| `zmq_addr` | fleet-telemetry ZMQ address, e.g. `tcp://<host>:5284` |
| `namespace` | must match fleet-telemetry `namespace` |
| `units_system` | `metric` (km/°C/bar) or `imperial` (mi/°F/psi) — HA display |
| `device_identifier` | `name` (slug → VIN out of entity_ids) or `vin` |
| `telemetry_profile` | `eco` / `balanced` / `live` / `custom` — how often signals are fetched |
| `fleetapi_enabled` | `true` only if you also run TeslaMate |
| `vins` | **optional** — leave empty to auto-discover every car on the stream; set a comma-separated list only to filter to specific VINs |
| `commands_enabled` | enable HA → car commands (needs a token via onboarding) |
| `log_level` | debug / info / warn / error |

The MQTT broker is auto-configured from the Supervisor (`services: mqtt:want`). To
override, set `TGW_HA_BROKER` etc. — but normally leave it.

## Port

`4460/tcp` exposes the Fleet API + WSS to your LAN so TeslaMate (on another host/LXC)
can poll `http://<ha-host>:4460`. **LAN-only — never expose to the internet.** If you
don't use TeslaMate, leave the port unmapped. Ensure host `:4460` is free.

## Data

Config, token cache and `ftc.json` persist in the add-on's `/data` (included in HA
backups). Optional captured `vehicle_data` templates improve Fleet API fidelity.
