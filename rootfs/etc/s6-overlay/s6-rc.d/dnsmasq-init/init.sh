#!/usr/bin/with-contenv bashio

# ==============================================================================
# DNSMASQ constants
# ==============================================================================

ADDON_DHCP_SERVER_START_EPOCH="/data/startepoch"
ADDON_CONFIG="/data/options.json"
ADDON_CONFIG_RESOLVED="/data/options.resolved.json"
DNSMASQ_CONFIG="/etc/dnsmasq.conf"
DNSMASQ_LEASE_DATABASE="/data/dnsmasq.leases"

# 5min is a reasonable threshold
JUST_REBOOTED_THRESHOLD_SEC=300

# ==============================================================================
# FUNCTIONS
# ==============================================================================

function log_info() {
    bashio::log.info "dnsmasq-init.sh: $@"
}

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
        log_info "NTP servers are ${NTP_SERVERS//$'\n'/,}"

        NTP_SERVERS_RESOLVED=""
        for srv in ${NTP_SERVERS}; do
            if ipvalid "$srv"; then
                # no need to carry out any DNS resolution
                log_info "Using NTP IP $srv without any DNS resolution"
                NTP_SERVERS_RESOLVED+="\"${srv}\","
            else
                # run the DNS resolution and pick the first IP address
                ip_addr="$(dig +short $srv | head -1)"
                if [[ ! -z "${ip_addr}" ]]; then
                    log_info "DNS resolved $srv -> $ip_addr"
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
        log_info "DNS servers are ${DNS_SERVERS//$'\n'/,}"

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
                log_info "Found invalid DNS server in DHCP network config: ${srv}. Skipping."
            fi
        done

        if [[ ! -z "${DNS_SERVERS_RESOLVED}" ]]; then
            # pop last comma:
            DNS_SERVERS_RESOLVED=${DNS_SERVERS_RESOLVED::-1}
            log_info "List of processed DNS servers is: ${DNS_SERVERS_RESOLVED}"

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
    log_info "Updated DHCP start epoch is: $updated_epoch"
}

function reset_dhcp_leases_database_if_just_rebooted() {
    # Get the uptime in seconds
    local uptime_seconds
    uptime_seconds=$(awk '{print int($1)}' /proc/uptime)

    if [ "$uptime_seconds" -lt "$JUST_REBOOTED_THRESHOLD_SEC" ]; then
        log_info "The HomeAssistant server has just been rebooted. Resetting DHCP lease database as requested in addon configuration."

        # Get the current timestamp
        local timestamp
        timestamp=$(date +"%Y%m%d%H%M%S")

        # the previuos database does not really get deleted, just moved in a file ignored by dnsmasq
        mv ${DNSMASQ_LEASE_DATABASE} ${DNSMASQ_LEASE_DATABASE}.${timestamp}
    else
        log_info "The HomeAssistant server is up since ${uptime_seconds}secs. Skipping DHCP lease database reset."
    fi
}


#
# MAIN
#

log_info "Starting dnsmasq configuration..."

should_reset_on_reboot=$(bashio::config 'dhcp_server.reset_dhcp_lease_database_on_reboot')
if [[ "$should_reset_on_reboot" = "null" ]]; then
    should_reset_on_reboot=false
fi
log_info The setting reset_dhcp_lease_database_on_reboot is ${should_reset_on_reboot}"..."
if $should_reset_on_reboot ; then
    reset_dhcp_leases_database_if_just_rebooted
fi

log_info "Advancing the DHCP server start epoch..."
bump_dhcp_server_start_epoch

# by default the resolved config is equal to the original config
cp ${ADDON_CONFIG} ${ADDON_CONFIG_RESOLVED}

# do some processing:
log_info "Resolving NTP hostnames eventually provided..."
resolve_ntp_servers
log_info "Processing DHCP DNS server list..."
process_dns_servers

log_info "Configuring dnsmasq..."
tempio \
    -conf ${ADDON_CONFIG_RESOLVED} \
    -template /usr/share/tempio/dnsmasq.config \
    -out "${DNSMASQ_CONFIG}"

log_info "Full dnsmasq config:"
cat -n $DNSMASQ_CONFIG

log_info "Successfully completed dnsmasq configuration."