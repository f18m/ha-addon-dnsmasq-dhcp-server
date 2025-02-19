#!/usr/bin/with-contenv bashio
# ==============================================================================
# DNSMASQ config
# ==============================================================================

ADDON_DHCP_SERVER_START_EPOCH="/data/startepoch"
ADDON_CONFIG="/data/options.json"
ADDON_CONFIG_RESOLVED="/data/options.resolved.json"
DNSMASQ_CONFIG="/etc/dnsmasq.conf"
DNSMASQ_LEASE_DATABASE="/data/dnsmasq.leases"

# 5min is a reasonable threshold
JUST_REBOOTED_THRESHOLD_SEC=300

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

function resolve_ntp_servers() {
    NTP_SERVERS="$(jq --raw-output '.dhcp_network.ntp_servers[]' ${ADDON_CONFIG_RESOLVED} 2>/dev/null)"
    if [[ ! -z "${NTP_SERVERS}" ]]; then
        bashio::log.info "NTP servers are ${NTP_SERVERS//$'\n'/,}"

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
            jq --compact-output ".dhcp_network.ntp_resolved=[$NTP_SERVERS_RESOLVED]" \
                ${ADDON_CONFIG_RESOLVED} >${ADDON_CONFIG_RESOLVED}.tmp
            mv ${ADDON_CONFIG_RESOLVED}.tmp ${ADDON_CONFIG_RESOLVED}
        fi
    fi
}

function process_dns_servers() {
    DNS_SERVERS="$(jq --raw-output '.dhcp_network.dns_servers[]' ${ADDON_CONFIG} 2>/dev/null)"
    if [[ ! -z "${DNS_SERVERS}" ]]; then
        bashio::log.info "DNS servers are ${DNS_SERVERS//$'\n'/,}"

        DNS_SERVERS_RESOLVED=""
        for srv in ${DNS_SERVERS}; do
            if ipvalid "$srv"; then
                # NOTE that dnsmasq supports the special address 0.0.0.0 which 
                # is taken to mean "the address of the machine running dnsmasq".
                # Since dnsmasq might be listening on multiple network interfaces, each
                # with a different IP address, we need to delegate to dnsmasq the selection
                # of the right IP address to advertise through DHCP
                DNS_SERVERS_RESOLVED+="\"${srv}\","
            else
                echo "Found invalid DNS server in DHCP network config: ${srv}. Skipping."
            fi
        done

        if [[ ! -z "${DNS_SERVERS_RESOLVED}" ]]; then
            # pop last comma:
            DNS_SERVERS_RESOLVED=${DNS_SERVERS_RESOLVED::-1}
            echo "List of processed DNS servers is: ${DNS_SERVERS_RESOLVED}"

            # add post-processed DNS servers
            jq --compact-output ".dhcp_network.dns_servers_processed=[$DNS_SERVERS_RESOLVED]" \
                ${ADDON_CONFIG_RESOLVED} >${ADDON_CONFIG_RESOLVED}.tmp
            mv ${ADDON_CONFIG_RESOLVED}.tmp ${ADDON_CONFIG_RESOLVED}
        fi
    fi
}

function bump_dhcp_server_start_epoch() {
    updated_epoch="$(date +%s)"
    echo $updated_epoch > "$ADDON_DHCP_SERVER_START_EPOCH"
    bashio::log.info "Updated DHCP start epoch is: $updated_epoch"
}

function reset_dhcp_leases_database_if_just_rebooted() {
    # Get the uptime in seconds
    local uptime_seconds
    uptime_seconds=$(awk '{print int($1)}' /proc/uptime)

    if [ "$uptime_seconds" -lt "$JUST_REBOOTED_THRESHOLD_SEC" ]; then
        bashio::log.info "The HomeAssistant server has just been rebooted. Resetting DHCP lease database as requested in addon configuration."

        # Get the current timestamp
        local timestamp
        timestamp=$(date +"%Y%m%d%H%M%S")

        # the previuos database does not really get deleted, just moved in a file ignored by dnsmasq
        mv ${DNSMASQ_LEASE_DATABASE} ${DNSMASQ_LEASE_DATABASE}.${timestamp}
    else
        bashio::log.info "The HomeAssistant server is up since ${uptime_seconds}secs. Skipping DHCP lease database reset."
    fi
}


#
# MAIN
#

bashio::log.info "Starting dnsmasq configuration..."

should_reset_on_reboot=$(bashio::config 'dhcp_server.reset_dhcp_lease_database_on_reboot')
if [[ "$should_reset_on_reboot" = "null" ]]; then
    should_reset_on_reboot=false
fi
bashio::log.info The setting reset_dhcp_lease_database_on_reboot is ${should_reset_on_reboot}"..."
if $should_reset_on_reboot ; then
    reset_dhcp_leases_database_if_just_rebooted
fi

bashio::log.info "Advancing the DHCP server start epoch..."
bump_dhcp_server_start_epoch

# by default the resolved config is equal to the original config
cp ${ADDON_CONFIG} ${ADDON_CONFIG_RESOLVED}

# do some processing:
bashio::log.info "Resolving NTP hostnames eventually provided..."
resolve_ntp_servers
bashio::log.info "Processing DHCP DNS server list..."
process_dns_servers

bashio::log.info "Configuring dnsmasq..."
tempio \
    -conf ${ADDON_CONFIG_RESOLVED} \
    -template /usr/share/tempio/dnsmasq.config \
    -out "${DNSMASQ_CONFIG}"

bashio::log.info "Full dnsmasq config:"
cat -n $DNSMASQ_CONFIG

bashio::log.info "Successfully completed dnsmasq configuration."