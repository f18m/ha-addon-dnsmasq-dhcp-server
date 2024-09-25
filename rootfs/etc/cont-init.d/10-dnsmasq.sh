#!/usr/bin/with-contenv bashio
# ==============================================================================
# DNSMASQ config
# ==============================================================================

ADDON_CONFIG="/data/options.json"
ADDON_CONFIG_RESOLVED="/data/options.resolved.json"
DNSMASQ_CONFIG="/etc/dnsmasq.conf"

cp ${ADDON_CONFIG} ${ADDON_CONFIG_RESOLVED}

bashio::log.info "Resolving NTP hostnames eventually provided..."
NTP_SERVERS="$(jq --raw-output '.network.ntp[]' ${ADDON_CONFIG} 2>/dev/null)"
if [[ ! -z "${NTP_SERVERS}" ]]; then
    bashio::log.info "NTP servers are ${NTP_SERVERS}"

    NTP_SERVERS_RESOLVED=""
    for srv in ${NTP_SERVERS}; do
        ip_addr="$(dig +short $srv)"
        if [[ ! -z "${ip_addr}" ]]; then
            echo "DNS resolved $srv -> $ip_addr"
            NTP_SERVERS_RESOLVED+="\"${ip_addr}\","
        fi
    done

    if [[ ! -z "${NTP_SERVERS_RESOLVED}" ]]; then
        # pop last comma:
        NTP_SERVERS_RESOLVED=${NTP_SERVERS_RESOLVED::-1}

        # add DNS-resolved IP addresses
        jq -c ".network.ntp_resolved=[$NTP_SERVERS_RESOLVED]" ${ADDON_CONFIG} >${ADDON_CONFIG_RESOLVED}
    fi
else
    cp ${ADDON_CONFIG} ${ADDON_CONFIG_RESOLVED}
fi

bashio::log.info "Configuring dnsmasq..."
tempio \
    -conf ${ADDON_CONFIG_RESOLVED} \
    -template /usr/share/tempio/dnsmasq.config \
    -out "${DNSMASQ_CONFIG}"

bashio::log.info "Full dnsmasq config:"
cat -n $DNSMASQ_CONFIG
