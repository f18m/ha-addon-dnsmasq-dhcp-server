#!/usr/bin/with-contenv bashio
# ==============================================================================
# DNSMASQ config
# ==============================================================================

ADDON_DHCP_SERVER_START_COUNTER="/data/startcounter"
ADDON_DHCP_SERVER_START_EPOCH="/data/startepoch"
ADDON_CONFIG="/data/options.json"
ADDON_CONFIG_RESOLVED="/data/options.resolved.json"
DNSMASQ_CONFIG="/etc/dnsmasq.conf"

function ipvalid() {
  # Set up local variables
  local ip=${1:-NO_IP_PROVIDED}
  local IFS=.; local -a a=($ip)
  # Start with a regex format test
  [[ $ip =~ ^[0-9]+(\.[0-9]+){3}$ ]] || return 1
  # Test values of quads
  local quad
  for quad in {0..3}; do
    [[ "${a[$quad]}" -gt 255 ]] && return 1
  done
  return 0
}

function dnsresolve() {
    NTP_SERVERS="$(jq --raw-output '.network.ntp[]' ${ADDON_CONFIG} 2>/dev/null)"
    if [[ ! -z "${NTP_SERVERS}" ]]; then
        bashio::log.info "NTP servers are ${NTP_SERVERS/$'\n'/,}"

        NTP_SERVERS_RESOLVED=""
        for srv in ${NTP_SERVERS}; do
            if ipvalid "$srv"; then
                # no need to carry out any DNS resolution
                echo "Using NTP IP $srv without any DNS resolution"
                NTP_SERVERS_RESOLVED+="\"${srv}\","
            else
                # run the DNS resolution and pick the first IP address
                ip_addr="$(dig +short $srv | head -1)"
                if [[ ! -z "${ip_addr}" ]]; then
                    echo "DNS resolved $srv -> $ip_addr"
                    NTP_SERVERS_RESOLVED+="\"${ip_addr}\","
                fi
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
}

function bump_dhcp_server_start_counter() {
    # Check if the file exists
    if [[ -f "$ADDON_DHCP_SERVER_START_COUNTER" ]]; then
        # Read the number from the file & increment it
        NUMBER=$(cat "$ADDON_DHCP_SERVER_START_COUNTER")
        NUMBER=$((NUMBER + 1))
    else
        # If the file doesn't exist, set NUMBER to 0
        NUMBER=0
    fi

    # Write the new value to the file
    echo "$NUMBER" > "$ADDON_DHCP_SERVER_START_COUNTER"
    bashio::log.info "Updated DHCP start counter is: $NUMBER"

    # Write also the current timestamp as Unix epoch
    date +%s > "$ADDON_DHCP_SERVER_START_EPOCH"
}


bashio::log.info "Advancing the DHCP server start counter by one..."
bump_dhcp_server_start_counter

bashio::log.info "Resolving NTP hostnames eventually provided..."
dnsresolve

bashio::log.info "Configuring dnsmasq..."
tempio \
    -conf ${ADDON_CONFIG_RESOLVED} \
    -template /usr/share/tempio/dnsmasq.config \
    -out "${DNSMASQ_CONFIG}"

bashio::log.info "Full dnsmasq config:"
cat -n $DNSMASQ_CONFIG
