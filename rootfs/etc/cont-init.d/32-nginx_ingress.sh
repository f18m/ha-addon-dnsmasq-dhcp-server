#!/usr/bin/with-contenv bashio
# shellcheck shell=bash
set -e

#################
# NGINX SETTING #
#################
bashio::log.info "Configuring nginx ingress..."

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

sed -i "s/%%port%%/${ingress_port}/g" /etc/nginx/servers/ingress.conf
sed -i "s/%%interface%%/${ingress_interface}/g" /etc/nginx/servers/ingress.conf
sed -i "s|%%ingress_entry%%|${ingress_entry}|g" /etc/nginx/servers/ingress.conf
sed -i "s|%%ingress_entry%%|${ingress_entry}|g" /etc/nginx/servers/ssl.conf

sed -i "s|%%web_ui_port%%|${web_ui_port}|g" /etc/nginx/servers/ingress.conf
sed -i "s|%%web_ui_port%%|${web_ui_port}|g" /etc/nginx/servers/ssl.conf
