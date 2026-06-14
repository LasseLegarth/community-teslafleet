#!/usr/bin/with-contenv bashio
# Map HA add-on options → TGW_* env, then run the gateway.
set -e

export TGW_INGEST_ZMQ_ADDR="$(bashio::config 'zmq_addr')"
export TGW_INGEST_NAMESPACE="$(bashio::config 'namespace')"
export TGW_FLEETAPI_ENABLED="$(bashio::config 'fleetapi_enabled')"
export TGW_HA_ENABLED="true"
export TGW_HA_BROKER="$(bashio::config 'ha_broker')"
export TGW_VINS="$(bashio::config 'vins')"
export TGW_TEMPLATE_DIR="/config/community_teslafleet/templates"
export TGW_UNITS_RANGE_INPUT="$(bashio::config 'range_input')"
export TGW_UNITS_SPEED_INPUT="$(bashio::config 'speed_input')"
export TGW_UNITS_ODOMETER_INPUT="$(bashio::config 'odometer_input')"
export TGW_LOG_LEVEL="$(bashio::config 'log_level')"
export TGW_CONFIG=""   # pure env-driven; no yaml file

exec /usr/bin/gateway
