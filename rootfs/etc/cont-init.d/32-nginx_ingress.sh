#!/usr/bin/with-contenv bashio
# shellcheck shell=bash
set -e

#################
# NGINX SETTING #
#################

declare ingress_interface
declare ingress_port
declare ingress_entry
declare web_ui_port

ingress_port=$(bashio::addon.ingress_port)
ingress_interface=$(bashio::addon.ip_address)
ingress_entry=$(bashio::addon.ingress_entry)
web_ui_port=$(bashio::config 'web_ui_port')

if [ -z "$ingress_port" ]; then
    ingress_port=8100
fi
if [ -z "$ingress_interface" ]; then
    ingress_interface=0.0.0.0
fi
if [ "$web_ui_port" = "null" ]; then
    web_ui_port=8976
fi

bashio::log.info "Configuring nginx ingress..."
bashio::log.info "ingress_port=${ingress_port}"
bashio::log.info "ingress_interface=${ingress_interface}"
bashio::log.info "ingress_entry=${ingress_entry}"
bashio::log.info "web_ui_port=${web_ui_port}"

sed -i "s/%%port%%/${ingress_port}/g" /etc/nginx/servers/ingress.conf
sed -i "s/%%interface%%/${ingress_interface}/g" /etc/nginx/servers/ingress.conf
sed -i "s|%%ingress_entry%%|${ingress_entry}|g" /etc/nginx/servers/ingress.conf
sed -i "s|%%web_ui_port%%|${web_ui_port}|g" /etc/nginx/servers/ingress.conf

bashio::log.info "nginx ingress config complete."
