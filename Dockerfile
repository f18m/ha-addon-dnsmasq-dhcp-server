ARG BUILD_FROM

# --- BACKEND BUILD
# About base image: we need to use a musl-based docker image since the actual HomeAssistant addon
# base image will be musl-based as well. This is required since we depend from "github.com/mattn/go-sqlite3"
# which is a CGO library; so that's why we select the -alpine variant
FROM golang:1.24-alpine AS builder-go

WORKDIR /app/backend
COPY dhcp-clients-webapp-backend .
RUN --mount=type=cache,target=/root/.cache/apk \
    apk add build-base
RUN --mount=type=cache,target=/root/.cache/go \
    CGO_ENABLED=1 go build -o /dhcp-clients-webapp-backend .


# --- Actual ADDON layer

FROM $BUILD_FROM

# Add env
ENV LANG=C.UTF-8

# Setup base
RUN apk add --no-cache dnsmasq nginx-debug sqlite socat && mv /etc/nginx /etc/nginx-orig

# Copy data
COPY rootfs /
COPY config.yaml /opt/bin/addon-config.yaml

# Copy web frontend
COPY dhcp-clients-webapp-backend/templates/*.html /opt/web/templates/
COPY dhcp-clients-webapp-backend/templates/*.js /opt/web/static/
COPY dhcp-clients-webapp-backend/templates/*.css /opt/web/static/
COPY dhcp-clients-webapp-backend/templates/libs/*.js /opt/web/static/
COPY dhcp-clients-webapp-backend/templates/libs/*.css /opt/web/static/

# Copy backend and frontend
COPY --from=builder-go /dhcp-clients-webapp-backend /opt/bin/

LABEL org.opencontainers.image.source=https://github.com/f18m/ha-addon-dnsmasq-dhcp-server
