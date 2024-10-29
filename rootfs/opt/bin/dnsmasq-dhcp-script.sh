#!/bin/sh

# input args
# see https://thekelleys.org.uk/dnsmasq/docs/dnsmasq-man.html
# MODE can be "add", "del", "old"
MODE="$1"
MAC_ADDRESS="$2"
IP_ADDRESS="$3"
HOSTNAME="${4:-}"

# constants
DB_PATH=/data/trackerdb.sqlite3
ADDON_DHCP_SERVER_START_EPOCH="/data/startepoch"
START_TIME_THRESHOLD_SEC=3

# About logging
# Unfortunately logging from this script to stdout does not produce any output
# (dnsmasq which launches this script seems to ignore its output).
# Logging to a file implies also handling its rotation/cleanup...
# so instead we log to a Unix socket and we have a basic 'socat' server 
# that acts as log proxy; see the /etc/services.d/log-helper for more details.
#LOGFILE="/data/dnsmasq-dhcp-script.log"
LOG_SOCKET=/tmp/dnsmasq-script-log-socket

log_info() {
    MSG="dnsmasq-script[$$]: $(date -Iseconds): INFO: $*"
    echo "$MSG" | socat - UNIX-CONNECT:${LOG_SOCKET}
}

log_error() {
    MSG="dnsmasq-script[$$]: $(date -Iseconds): ERR: $*"
    echo "$MSG" | socat - UNIX-CONNECT:${LOG_SOCKET}
}

# Reads the current start epoch into global var START_EPOCH
read_start_epoch() {
    if [[ -f "$ADDON_DHCP_SERVER_START_EPOCH" ]]; then
        START_EPOCH=$(cat "$ADDON_DHCP_SERVER_START_EPOCH")
    else
        # this is a sign something's wrong...
        log_error "Failed to read the $ADDON_DHCP_SERVER_START_EPOCH"
        START_EPOCH=1
    fi
}

# Returns 1 if the dnsmasq just started or 0 if not
dnsmasq_just_started() {  
    # Perform the subtraction
    CURRENT_EPOCH=$(date +%s)
    RESULT=$((CURRENT_EPOCH - START_EPOCH))

    # Compare the result with 3
    #log_info "Comparing DHCP server start epoch (${START_EPOCH}) with the current epoch (${CURRENT_EPOCH})"
    if [[ "$RESULT" -lt $START_TIME_THRESHOLD_SEC ]]; then
        return 1
    else
        return 0
    fi
}

# Function to add or update a DHCP client in the SQLite3 database
add_or_update_dhcp_client() {
    local db_path=$1
    local mac_addr=$2
    local hostname=$3
    local last_seen=$4
    local dhcp_server_start_counter=$5

    # Create the table if it doesn't exist
	# NOTE: the 'dhcp_server_start_counter' column actually contains Epochs and is named like that for backward compat
    sqlite3 "$db_path" <<EOF
CREATE TABLE IF NOT EXISTS dhcp_clients (
    mac_addr TEXT PRIMARY KEY,
    hostname TEXT,
    last_seen TEXT,
    dhcp_server_start_counter INT
);
EOF

    # Insert or update the DHCP client data
    sqlite3 "$db_path" <<EOF
INSERT INTO dhcp_clients (mac_addr, hostname, last_seen, dhcp_server_start_counter)
VALUES ('$mac_addr', '$hostname','$last_seen', $dhcp_server_start_counter)
ON CONFLICT(mac_addr) DO UPDATE SET
    hostname=excluded.hostname,
    last_seen=excluded.last_seen,
    dhcp_server_start_counter=excluded.dhcp_server_start_counter;
EOF

    if [[ $? -eq 0 ]]; then
        log_info "Stored in trackerDB updated information for client mac=$mac_addr, hostname=$hostname: last_seen=$last_seen, dhcp_server_start_epoch=$dhcp_server_start_counter"
    else
        log_error "Failed to add/update client. Expect inconsistencies."
    fi
}

#
# IMPORTANT:
# We do something only when MODE==add, which means a new DHCP lease was given, which means the
# DHCP client was talking with the DHCP server.
# We purposely exclude events produced at the start of dnsmasq, since they do not indicate
# the DHCP client is really alive.
# According to docs:
#  """
#   At dnsmasq startup, the script will be invoked for all existing leases as they are read from 
#   the lease file. Expired leases will be called with "del" and others with "old". 
#   When dnsmasq receives a HUP signal, the script will be invoked for existing leases with an "old" event.
#  """
#
read_start_epoch
if [[ "$MODE" = "add" ]]; then
    log_info "*** Triggered with mode=${MODE}, mac=${MAC_ADDRESS}, hostname=${HOSTNAME} ***"
    last_seen=$(date -u +"%Y-%m-%dT%H:%M:%SZ")  # ISO 8601 UTC format
    add_or_update_dhcp_client "$DB_PATH" "$MAC_ADDRESS" "$HOSTNAME" "$last_seen" "$START_EPOCH"

elif [[ "$MODE" = "old" ]]; then
    # at dnsmasq startup we get a bunch of these 'old' updates -- we need to filter them out
    dnsmasq_just_started
    if [[ $? -eq 0 ]]; then 
        log_info "*** Triggered with mode=${MODE}, mac=${MAC_ADDRESS}, hostname=${HOSTNAME} ***"
        last_seen=$(date -u +"%Y-%m-%dT%H:%M:%SZ")  # ISO 8601 UTC format
        add_or_update_dhcp_client "$DB_PATH" "$MAC_ADDRESS" "$HOSTNAME" "$last_seen" "$START_EPOCH"

    # reduce logging at startup:
    #else
        #log_info "Detected startup LEASE processing and ignoring it"
    fi
else
    log_info "*** Triggered with mode=${MODE}, mac=${MAC_ADDRESS}, hostname=${HOSTNAME} ***"
    log_info "Ignoring this trigger"
fi
