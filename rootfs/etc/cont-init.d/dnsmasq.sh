#!/usr/bin/with-contenv bashio
# ==============================================================================
# DNSMASQ config
# ==============================================================================

bashio::log.info "Configuring dnsmasq..."

CONFIG="/etc/dnsmasq.conf"
tempio \
    -conf /data/options.json \
    -template /usr/share/tempio/dnsmasq.config \
    -out "${CONFIG}"

bashio::log.info "Dnsmasq template successfully rendered as ${CONFIG}"
