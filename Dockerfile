ARG BUILD_FROM

# --- BACKEND BUILD
FROM golang:1.23 AS builder-go

WORKDIR /app/backend
#COPY backend/go.mod backend/go.sum ./
#RUN go mod download

# Copia tutto il codice Go
COPY dhcp-clients-webapp-backend .

# Compila l'applicazione Go
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /dhcp-clients-webapp-backend .




# --- FRONTEND build
FROM node:18 AS builder-angular

WORKDIR /app/frontend
COPY dhcp-clients-webapp-frontend/package*.json ./

RUN npm install
COPY dhcp-clients-webapp-frontend/ .
RUN npm run build --prod
    


# --- Actual ADDON layer

FROM $BUILD_FROM

# Add env
ENV LANG C.UTF-8

# Setup base
RUN apk add --no-cache dnsmasq nginx && mkdir -p /run/nginx

# Copy data
COPY rootfs /

# Copy backend and frontend
COPY --from=builder-go /dhcp-clients-webapp-backend /opt/bin/
COPY --from=builder-angular /app/frontend/dist /app/frontend
