ARG BUILD_FROM

# --- BACKEND BUILD
FROM golang:1.23 AS builder-go

WORKDIR /app/backend
#COPY backend/go.mod backend/go.sum ./
#RUN go mod download
COPY dhcp-clients-webapp-backend .
RUN CGO_ENABLED=0 go build -o /dhcp-clients-webapp-backend .




# --- FRONTEND build
FROM node:18 AS builder-angular

WORKDIR /app/frontend
COPY dhcp-clients-webapp-frontend/package*.json ./

RUN npm install -g @angular/cli
RUN npm install
COPY dhcp-clients-webapp-frontend/ .
#RUN npm run build --prod
RUN ng build --configuration=production
    


# --- Actual ADDON layer

FROM $BUILD_FROM

# Add env
ENV LANG C.UTF-8

# Setup base
RUN apk add --no-cache \
    dnsmasq=2.90-r2 \
    nginx=1.24.0-r16 && \
        rm -fr /tmp/* /etc/nginx

# Copy data
COPY rootfs /

# Copy backend and frontend
COPY --from=builder-go /dhcp-clients-webapp-backend /opt/bin/
COPY --from=builder-angular /app/frontend/dist/dhcp-clients-webapp-frontend/browser/ /opt/www/

LABEL org.opencontainers.image.source=https://github.com/f18m/ha-addon-dnsmasq-dhcp-server

