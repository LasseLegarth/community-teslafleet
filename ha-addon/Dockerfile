# HA add-on image: alpine base + the prebuilt gateway binary + a run script that
# maps add-on options (/data/options.json) to TGW_* env vars.
ARG BUILD_FROM=ghcr.io/home-assistant/amd64-base:latest
FROM ghcr.io/legarth/community-teslafleet:latest AS gw

FROM ${BUILD_FROM}
COPY --from=gw /gateway /usr/bin/gateway
COPY run.sh /run.sh
RUN chmod +x /run.sh
CMD ["/run.sh"]
