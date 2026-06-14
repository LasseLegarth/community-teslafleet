# HA entity-mapping — per-felt-beslutninger

Hvordan hvert streamet fleet-telemetry-felt eksponeres i Home Assistant via MQTT discovery.

**Metode:** (1) hvad vi streamer (verificeret fra `telemetry.jsonl`, 177 felter), (2) konventioner fra
`tesla_fleet` (HA core) + `tesla_custom` (HACS) som reference, (3) vores egen dom pr. felt.

## Beslutninger (truffet)

1. **Enheder er konfigurerbare via `units.system` (`metric` | `imperial`, default `metric`).** Ét valg
   dækker distance, speed, temp og tryk (samme model som HA selv; brugere kan stadig override pr.
   entity i HA's UI). Gateway'en konverterer FRA fleet-telemetry's faste stream-enheder (distance/speed
   i miles/mph — Teslas konvention, verificeret; temp °C; tryk bar) TIL det valgte system:
   - **metric** → `km`, `km/h`, `°C`, `bar`
   - **imperial** → `mi`, `mph`, `°F`, `psi`
   `vehicle_data` (TeslaMate) er afkoblet — altid miles (TeslaMate konverterer selv via sit gui_setting).
   `%`, `V`, `A`, `kW`, `kWh`, `g`, `Nm`, `°` er enhedsløse/SI og sendes råt uanset system.
2. **Primær + diagnostic-kategori.** Daglige felter som normale entities; dyb diagnostik som
   `entity_category: diagnostic` (synlig men gemt).
3. **Sammensatte felter splittes** til per-position entities + en afledt aggregat-binary.

## Tværgående regler

- **Enum-normalisering:** string-enums følger `<Type><Værdi>`. Generisk normalizer fjerner prefix
  (split efter sidste `State`, ellers per-felt-prefix-tabel for de få uden `State`:
  `ChargePortLatch`, `CableType`, `ChargePort`, `FollowDistance`, `BMSState`). Resultat:
  `device_class: enum` med rene snake_case-options.
- **Bool → binary_sensor.** `payload_on: "true"`, `payload_off: "false"` (Go-JSON bool).
- **Afledte entities** (ikke et råt felt — beregnet i gateway'en):
  `binary_sensor.charging` (battery_charging) fra `DetailedChargeState == Charging`;
  `binary_sensor.plugged_in` (plug) fra `ChargingCableType`/`ChargePortLatch`;
  `binary_sensor.doors`/`windows` (aggregat OR).
- **device_tracker:** `Location`→primær (attrs: heading, speed), `DestinationLocation`→primær,
  `OriginLocation`→diagnostic.
- **Device:** `identifiers: [["teslafleet", VIN]]`, `manufacturer: "Tesla"`,
  `model: <VIN[3]→S/3/X/Y>`, `sw_version: <Version>`, `name: <display_name>`, `serial_number: VIN`.
  unique_id pr. entity: `<VIN>_<key>`.

Kolonner: **felt** | platform | device_class | unit (konv.) | state_class | kategori | navn | note
`P`=primær, `D`=diagnostic.

---

## Batteri & energi

| felt | platform | device_class | unit | state_class | kat | navn | note |
|---|---|---|---|---|---|---|---|
| BatteryLevel | sensor | battery | % | measurement | P | Batteri | brugbar % |
| Soc | sensor | battery | % | measurement | D | Rå SoC | |
| EnergyRemaining | sensor | energy_storage | kWh | measurement | P | Energi tilbage | |
| LifetimeEnergyUsed | sensor | energy | kWh | total_increasing | D | Energi brugt (levetid) | |
| ChargeLimitSoc | sensor | battery | % | — | P | Ladegrænse | (kan blive `number` via relay) |

## Rækkevidde & distance (mi→km)

| felt | platform | device_class | unit | state_class | kat | navn | note |
|---|---|---|---|---|---|---|---|
| RatedRange | sensor | distance | km | measurement | P | Rækkevidde | precision 0 |
| IdealBatteryRange | sensor | distance | km | measurement | D | Ideel rækkevidde | precision 0 |
| Odometer | sensor | distance | km | total_increasing | D | Kilometertal | precision 0 |

## Hastighed (mph→km/h)

| felt | platform | device_class | unit | state_class | kat | navn | note |
|---|---|---|---|---|---|---|---|
| VehicleSpeed | sensor | speed | km/h | measurement | P | Hastighed | `null`→0 |
| ChargeRateMilePerHour | sensor | speed | km/h | measurement | D | Laderate | −1 = n/a |
| CruiseSetSpeed | sensor | speed | km/h | measurement | D | Cruise sat-hastighed | |
| CurrentLimitMph | sensor | speed | km/h | — | D | Hastighedsgrænse | |

## Kørsel & lokation

| felt | platform | device_class | unit | state_class | kat | navn | note |
|---|---|---|---|---|---|---|---|
| Gear | sensor | enum | — | — | P | Gear | options P/D/R/N, `null`→P |
| Location | device_tracker | — | — | — | P | Lokation | attrs: heading, speed |
| DestinationLocation | device_tracker | — | — | — | P | Destination | |
| OriginLocation | device_tracker | — | — | — | D | Oprindelse | |
| GpsHeading | sensor | — | ° | measurement | D | Retning | også tracker-attr |
| GpsState | binary_sensor | connectivity | — | — | D | GPS | |
| LocatedAtHome | binary_sensor | — | — | — | P | Hjemme | |
| LocatedAtWork | binary_sensor | — | — | — | D | På arbejde | |
| LocatedAtFavorite | binary_sensor | — | — | — | D | Ved favorit | |
| RouteTrafficMinutesDelay | sensor | duration | min | — | D | Trafikforsinkelse | |
| RouteLastUpdated | — | — | — | — | — | (drop) | dict h/m/s, lav værdi |
| RouteLine | — | — | — | — | — | (drop) | encoded polyline |
| SuperchargerSessionTripPlanner | binary_sensor | — | — | — | D | SC trip-planner | |

## Temperatur (°C, råt)

| felt | platform | device_class | unit | state_class | kat | navn | note |
|---|---|---|---|---|---|---|---|
| InsideTemp | sensor | temperature | °C | measurement | P | Kabinetemp | precision 1 |
| OutsideTemp | sensor | temperature | °C | measurement | P | Udetemp | precision 1 |
| HvacLeftTemperatureRequest | sensor | temperature | °C | — | D | Førersæde-temp ønsket | |
| HvacRightTemperatureRequest | sensor | temperature | °C | — | D | Passager-temp ønsket | |

## Opladning

| felt | platform | device_class | unit | state_class | kat | navn | note |
|---|---|---|---|---|---|---|---|
| DetailedChargeState | sensor | enum | — | — | P | Ladestatus | |
| *(afledt)* | binary_sensor | battery_charging | — | — | P | Oplader | `DetailedChargeState==Charging` |
| *(afledt)* | binary_sensor | plug | — | — | P | Tilsluttet | fra cable/latch |
| ChargeState | sensor | enum | — | — | D | Ladestatus (rå) | Idle/ClearFaults |
| ACChargingPower | sensor | power | kW | measurement | P | AC-ladeeffekt | |
| DCChargingPower | sensor | power | kW | measurement | P | DC-ladeeffekt | |
| ACChargingEnergyIn | sensor | energy | kWh | total | D | AC-energi ind | resetter pr. session |
| DCChargingEnergyIn | sensor | energy | kWh | total | D | DC-energi ind | |
| ChargerVoltage | sensor | voltage | V | measurement | D | Ladespænding | |
| ChargeAmps | sensor | current | A | measurement | D | Ladestrøm | |
| ChargeCurrentRequest | sensor | current | A | — | D | Ladestrøm ønsket | |
| ChargeCurrentRequestMax | sensor | current | A | — | D | Ladestrøm max | |
| ChargeEnableRequest | binary_sensor | — | — | — | D | Ladning anmodet | |
| ChargePortDoorOpen | binary_sensor | door | — | — | P | Ladeport | (alternativt cover) |
| ChargePortLatch | sensor | enum | — | — | D | Ladeport-lås | Engaged/Disengaged |
| ChargePort | sensor | enum | — | — | D | Ladeport-type | CCS/... |
| ChargePortColdWeatherMode | binary_sensor | — | — | — | D | Koldvejrs-mode | |
| ChargingCableType | sensor | enum | — | — | D | Kabeltype | IEC/SAE |
| FastChargerPresent | binary_sensor | — | — | — | D | Lynlader til stede | |
| ScheduledChargingMode | sensor | enum | — | — | D | Planlagt ladning | |
| ScheduledChargingPending | binary_sensor | — | — | — | D | Planlagt ladning afventer | |
| BmsFullchargecomplete | binary_sensor | — | — | — | D | Fuld-ladning komplet | |
| SettingChargeUnit | sensor | enum | — | — | D | Ladeenhed | %/range |
| TimeToFullCharge | sensor | timestamp | — | — | P | Færdig med ladning | **verificér enhed** (streamede ikke) |

## Lukninger & lås

| felt | platform | device_class | unit | state_class | kat | navn | note |
|---|---|---|---|---|---|---|---|
| Locked | lock | — | — | — | P | Lås | (binary lock) |
| DoorState.DriverFront | binary_sensor | door | — | — | D | Dør førerside for | |
| DoorState.DriverRear | binary_sensor | door | — | — | D | Dør førerside bag | |
| DoorState.PassengerFront | binary_sensor | door | — | — | D | Dør passager for | |
| DoorState.PassengerRear | binary_sensor | door | — | — | D | Dør passager bag | |
| *(afledt)* | binary_sensor | door | — | — | P | Døre | OR af de 4 |
| FdWindow | binary_sensor | window | — | — | D | Vindue førerside for | enum→open/closed |
| FpWindow | binary_sensor | window | — | — | D | Vindue passager for | |
| RdWindow | binary_sensor | window | — | — | D | Vindue førerside bag | |
| RpWindow | binary_sensor | window | — | — | D | Vindue passager bag | |
| *(afledt)* | binary_sensor | window | — | — | P | Vinduer | OR af de 4 |

## Dæktryk (TPMS, bar råt)

| felt | platform | device_class | unit | state_class | kat | navn | note |
|---|---|---|---|---|---|---|---|
| TpmsPressureFl | sensor | pressure | bar | measurement | D | Dæktryk FV | precision 1 |
| TpmsPressureFr | sensor | pressure | bar | measurement | D | Dæktryk FH | |
| TpmsPressureRl | sensor | pressure | bar | measurement | D | Dæktryk BV | |
| TpmsPressureRr | sensor | pressure | bar | measurement | D | Dæktryk BH | |
| TpmsHardWarnings.* | binary_sensor | problem | — | — | D | Dæk hard-advarsel ×4 | split per hjul |
| TpmsSoftWarnings.* | binary_sensor | problem | — | — | D | Dæk soft-advarsel ×4 | split per hjul |
| TpmsLastSeenPressureTime{Fl..Rr} | sensor | timestamp | — | — | D | Dæktryk sidst set ×4 | epoch→ts |

## Klima & HVAC

| felt | platform | device_class | unit | state_class | kat | navn | note |
|---|---|---|---|---|---|---|---|
| HvacPower | binary_sensor | running | — | — | P | Klima | enum On/Off→on/off |
| HvacACEnabled | binary_sensor | — | — | — | D | AC | |
| HvacAutoMode | sensor | enum | — | — | D | Auto-mode | |
| HvacFanSpeed | sensor | — | — | measurement | D | Blæser sat | |
| HvacFanStatus | sensor | — | — | measurement | D | Blæser status | |
| ClimateKeeperMode | sensor | enum | — | — | D | Climate keeper | |
| CabinOverheatProtectionMode | sensor | enum | — | — | D | Kabine-overophedningsbeskyttelse | |
| CabinOverheatProtectionTemperatureLimit | sensor | enum | — | — | D | COP temp-grænse | |
| DefrostMode | sensor | enum | — | — | D | Defrost | |
| DefrostForPreconditioning | binary_sensor | — | — | — | D | Defrost til precond. | |
| RearDefrostEnabled | binary_sensor | — | — | — | D | Bagrudevarme | |
| PreconditioningEnabled | binary_sensor | — | — | — | D | Precondition | |
| BatteryHeaterOn | binary_sensor | heat | — | — | D | Batterivarmer | |
| HvacSteeringWheelHeatAuto | binary_sensor | — | — | — | D | Rat-varme auto | |
| HvacSteeringWheelHeatLevel | sensor | — | — | — | D | Rat-varme niveau | |
| WiperHeatEnabled | binary_sensor | heat | — | — | D | Vinduesvisker-varme | |
| SeatHeaterLeft | sensor | enum | — | — | D | Sædevarme førersæde | read-only (off/low/med/high) |
| SeatHeaterRight | sensor | enum | — | — | D | Sædevarme passager | |
| SeatHeaterRearLeft | sensor | enum | — | — | D | Sædevarme bag V | |
| SeatHeaterRearCenter | sensor | enum | — | — | D | Sædevarme bag midt | |
| SeatHeaterRearRight | sensor | enum | — | — | D | Sædevarme bag H | |
| AutoSeatClimateLeft | binary_sensor | — | — | — | D | Auto-sædeklima V | |
| AutoSeatClimateRight | binary_sensor | — | — | — | D | Auto-sædeklima H | |
| ClimateSeatCoolingFrontLeft | sensor | — | — | — | D | Sædekøling for V | |
| ClimateSeatCoolingFrontRight | sensor | — | — | — | D | Sædekøling for H | |

> HvacPower bliver en simpel on/off-binary nu (read-only). Fuld `climate`-entity (sætbar temp/mode)
> venter til kommando-integration via relay.

## Powertrain & batteri-diagnostik (alle D)

| felt | platform | device_class | unit | state_class | navn | note |
|---|---|---|---|---|---|---|
| PackVoltage | sensor | voltage | V | measurement | Pakkespænding | |
| PackCurrent | sensor | current | A | measurement | Pakkestrøm | |
| BrickVoltageMax | sensor | voltage | V | measurement | Celle-spænding max | |
| BrickVoltageMin | sensor | voltage | V | measurement | Celle-spænding min | |
| NumBrickVoltageMax | sensor | — | — | — | Celle-index max | ingen device_class |
| NumBrickVoltageMin | sensor | — | — | — | Celle-index min | |
| ModuleTempMax | sensor | temperature | °C | measurement | Modultemp max | |
| ModuleTempMin | sensor | temperature | °C | measurement | Modultemp min | |
| NumModuleTempMax | sensor | — | — | — | Modul-index max temp | |
| NumModuleTempMin | sensor | — | — | — | Modul-index min temp | |
| IsolationResistance | sensor | — | kΩ | measurement | Isolationsmodstand | verificér enhed |
| BMSState | sensor | enum | — | — | BMS-tilstand | |
| Hvil | sensor | enum | — | — | HV-interlock | HvilStatusOK |
| DCDCEnable | binary_sensor | — | — | — | DC/DC aktiv | |
| DriveRail | binary_sensor | — | — | — | Drive-rail | |

## Motor (drive inverter, front/rear — alle D)

| felt | platform | device_class | unit | state_class | navn |
|---|---|---|---|---|---|
| DiInverterTF / TR | sensor | temperature | °C | measurement | Inverter-temp for/bag |
| DiHeatsinkTF / TR | sensor | temperature | °C | measurement | Køleplade-temp for/bag |
| DiStatorTempF / R | sensor | temperature | °C | measurement | Stator-temp for/bag |
| DiMotorCurrentF / R | sensor | current | A | measurement | Motorstrøm for/bag |
| DiTorqueActualF / R | sensor | — | Nm | measurement | Moment for/bag |
| DiTorquemotor | sensor | — | Nm | measurement | Moment (motor) |
| DiAxleSpeedF / R | sensor | — | rpm | measurement | Akselhastighed for/bag (verificér enhed) |
| DiVBatF / R | sensor | voltage | V | measurement | Inverter-batterispænding for/bag |
| DiStateF / R | sensor | enum | — | — | Inverter-tilstand for/bag |

## Dynamik & pedaler (alle D)

| felt | platform | device_class | unit | state_class | navn |
|---|---|---|---|---|---|
| LateralAcceleration | sensor | — | g | measurement | Lateral acceleration |
| LongitudinalAcceleration | sensor | — | g | measurement | Longitudinal acceleration |
| BrakePedal | binary_sensor | — | — | — | Bremsepedal |
| BrakePedalPos | sensor | — | % | measurement | Bremsepedal-position |
| PedalPosition | sensor | — | % | measurement | Speeder-position |

## Lys & sikkerhed

| felt | platform | device_class | unit | kat | navn |
|---|---|---|---|---|---|
| DriverSeatOccupied | binary_sensor | occupancy | — | P | Fører til stede |
| SentryMode | sensor | enum | — | P | Sentry-tilstand |
| LightsTurnSignal | sensor | enum | — | D | Blinklys |
| LightsHighBeams | binary_sensor | light | — | D | Fjernlys |
| LightsHazardsActive | binary_sensor | — | — | D | Havariblink |
| DriverSeatBelt | binary_sensor | safety | — | D | Sele fører (on=spændt? verificér polaritet) |
| PassengerSeatBelt | sensor | enum | — | D | Sele passager |
| BrakePedal | (se dynamik) | | | | |
| AutomaticBlindSpotCamera | binary_sensor | — | — | D | Blindvinkel-kamera |
| BlindSpotCollisionWarningChime | binary_sensor | — | — | D | Blindvinkel-advarsel |
| AutomaticEmergencyBrakingOff | binary_sensor | — | — | D | AEB slået fra |
| EmergencyLaneDepartureAvoidance | binary_sensor | — | — | D | Nødvognbaneassist |
| ForwardCollisionWarning | sensor | enum | — | D | Frontkollisionsadvarsel |
| LaneDepartureAvoidance | sensor | enum | — | D | Vognbaneassist |
| SpeedLimitMode | binary_sensor | — | — | D | Hastighedsgrænse-mode |
| SpeedLimitWarning | sensor | enum | — | D | Hastighedsadvarsel |
| CruiseFollowDistance | sensor | enum | — | D | Cruise følgeafstand |

## Sikkerhed & adgang

| felt | platform | device_class | kat | navn |
|---|---|---|---|---|
| CenterDisplay | sensor | enum | D | Skærm-tilstand |
| ValetModeEnabled | binary_sensor | — | D | Valet-mode |
| PinToDriveEnabled | binary_sensor | — | D | PIN-to-drive |
| GuestModeEnabled | binary_sensor | — | D | Gæste-mode |
| GuestModeMobileAccessState | sensor | enum | D | Gæste-mobiladgang |
| ServiceMode | binary_sensor | — | D | Service-mode |
| RemoteStartEnabled | binary_sensor | — | D | Fjernstart aktiv |
| PairedPhoneKeyAndKeyFobQty | sensor | — | D | Parrede nøgler |
| HomelinkDeviceCount | sensor | — | D | Homelink-enheder |
| HomelinkNearby | binary_sensor | — | D | Homelink i nærheden |

## Medie

| felt | platform | device_class | kat | navn |
|---|---|---|---|---|
| MediaPlaybackStatus | sensor | enum | P | Medie-status |
| MediaNowPlayingTitle | sensor | — | P | Nu spiller (titel) |
| MediaNowPlayingArtist | sensor | — | P | Nu spiller (kunstner) |
| MediaNowPlayingAlbum | sensor | — | D | Nu spiller (album) |
| MediaNowPlayingStation | sensor | — | D | Station |
| MediaPlaybackSource | sensor | — | D | Kilde |
| MediaNowPlayingDuration | sensor | duration | D | Varighed (ms) |
| MediaNowPlayingElapsed | sensor | duration | D | Forløbet (ms) |
| MediaAudioVolume | sensor | — | D | Lydstyrke |
| MediaAudioVolumeMax | sensor | — | D | Lydstyrke max |
| MediaAudioVolumeIncrement | sensor | — | D | Lydstyrke-trin |

## Software & settings (alle D, config-agtige)

| felt | platform | device_class | navn | note |
|---|---|---|---|---|
| Version | sensor + device.sw_version | — | Softwareversion | også på device |
| SoftwareUpdateVersion | sensor | — | Opdaterings-version | |
| SoftwareUpdateDownloadPercentComplete | sensor | — | Download % | unit % |
| SoftwareUpdateInstallationPercentComplete | sensor | — | Installation % | unit % |
| SoftwareUpdateExpectedDurationMinutes | sensor | duration | Forventet varighed | min |
| Setting24HourTime | binary_sensor | — | 24-timers tid | |
| SettingDistanceUnit | sensor | enum | Distanceenhed (bil) | |
| SettingTemperatureUnit | sensor | enum | Temp-enhed (bil) | |
| SettingTirePressureUnit | sensor | enum | Dæktryk-enhed (bil) | |
| WheelType | sensor | enum | Fælgtype | |
| EfficiencyPackage | sensor | enum | Effektivitetspakke | |
| RearSeatHeaters | sensor | enum | Bagsædevarme-config | |
| ChargePortColdWeatherMode | (se opladning) | | | |

---

## Felter vi dropper (lav/ingen HA-værdi)

`RouteLine` (encoded polyline), `RouteLastUpdated` (dict h/m/s). Resten af de 177 er dækket ovenfor.

## Implementeringsnoter

- `entities.go` bygges om fra "curated + generisk" til en **katalog-drevet** model: ét `fieldSpec` pr.
  felt (platform, device_class, unit, conv-funktion, state_class, category, navn, precision,
  enum-options-flag). Felter uden katalog-entry får en sikker generisk fallback (sensor, diagnostic).
- Konvertering pr. felt via en lille `convert`-funktion (`mi2km`, `mph2kmh`, `identity`).
- Composite-felter (dicts) ekspanderes til flere entities ud fra spec'ens `subfields`.
- Afledte entities beregnes i `buildState` (charging, plugged_in, door/window-aggregat).
- HA-siden vi allerede byggede (manuel km kun på range/speed) erstattes af denne katalog-konvertering
  så ALT er konsistent metrisk.
