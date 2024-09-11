ARG BUILD_FROM
FROM $BUILD_FROM

# Add env
ENV LANG C.UTF-8

# Setup base
RUN apk add --no-cache dnsmasq nginx && mkdir -p /run/nginx && rm -f /etc/nginx/http.d/default.conf

# Copy data
COPY rootfs /
