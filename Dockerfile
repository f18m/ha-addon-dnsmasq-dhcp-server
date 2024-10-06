ARG BUILD_FROM

# --- BACKEND BUILD
# About golang version: downgrade Go to 1.22.7 to avoid https://github.com/golang/go/issues/68976
FROM golang:1.22.7 AS builder-go

WORKDIR /app/backend
COPY dhcp-clients-webapp-backend .
RUN CGO_ENABLED=0 go build -o /dhcp-clients-webapp-backend .


# --- Actual ADDON layer

FROM $BUILD_FROM

# Add env
ENV LANG=C.UTF-8

# Setup base
RUN apk add --no-cache dnsmasq nginx-debug && mv /etc/nginx /etc/nginx-orig

# Copy data
COPY rootfs /
COPY dhcp-clients-webapp-backend/templates/ /opt/web/templates/

# Copy backend and frontend
COPY --from=builder-go /dhcp-clients-webapp-backend /opt/bin/

LABEL org.opencontainers.image.source=https://github.com/f18m/ha-addon-dnsmasq-dhcp-server
