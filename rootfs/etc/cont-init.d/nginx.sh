#!/usr/bin/with-contenv bashio
# ==============================================================================
# Configure NGINX
# ==============================================================================

ingress_port=$(bashio::addon.ingress_port)
ingress_entry=$(bashio::addon.ingress_entry)
ingress_interface=$(bashio::addon.ip_address)

# Retrieve the ingress_entry query path so that nginx can perform rewrites accordingly
bashio::log.info "Configuring nginx with ingress_entry=${ingress_entry} and ingress_interface=${ingress_interface}"

CONFIG=/etc/nginx/nginx.conf
sed -i "s#%%ingress_entry%%#${ingress_entry}#g" ${CONFIG}
sed -i "s/%%interface%%/${ingress_interface}/g" ${CONFIG}

bashio::log.info "Dnsmasq template successfully rendered as ${CONFIG}"