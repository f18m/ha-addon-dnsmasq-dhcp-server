ARG BUILD_FROM

# --- BACKEND BUILD
FROM golang:1.23 AS builder-go

WORKDIR /app/backend
#COPY backend/go.mod backend/go.sum ./
#RUN go mod download
COPY dhcp-clients-webapp-backend .
RUN CGO_ENABLED=0 go build -o /dhcp-clients-webapp-backend .


# --- Actual ADDON layer

FROM $BUILD_FROM

# Add env
ENV LANG C.UTF-8

# Setup base
RUN apk add --no-cache dnsmasq nginx && mkdir -p /run/nginx

# Copy data
COPY rootfs /
COPY dhcp-clients-webapp-backend/templates/ /opt/web/

# Copy backend and frontend
COPY --from=builder-go /dhcp-clients-webapp-backend /opt/bin/

LABEL org.opencontainers.image.source=https://github.com/f18m/ha-addon-dnsmasq-dhcp-server
