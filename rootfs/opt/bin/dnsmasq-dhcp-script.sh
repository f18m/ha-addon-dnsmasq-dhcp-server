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
ADDON_DHCP_SERVER_START_COUNTER="/data/startcounter"
ADDON_DHCP_SERVER_START_EPOCH="/data/startepoch"
LOGFILE="/data/dnsmasq-dhcp-script.log"
START_TIME_THRESHOLD_SEC=3

log_info() {
    echo "$(date): INFO: $*" >>$LOGFILE
}

log_error() {
    echo "$(date): ERR: $*" >>$LOGFILE
}

# Reads the current start counter into global var DHCP_SERVER_START_COUNTER
read_start_counter() {
    if [[ -f "$ADDON_DHCP_SERVER_START_COUNTER" ]]; then
        DHCP_SERVER_START_COUNTER=$(cat "$ADDON_DHCP_SERVER_START_COUNTER")
    else
        DHCP_SERVER_START_COUNTER=0
    fi

    log_info "The DHCP server start counter is ${DHCP_SERVER_START_COUNTER}"
}

# Returns 1 if the dnsmasq just started or 0 if not
dnsmasq_just_started() {  
    if [[ -f "$ADDON_DHCP_SERVER_START_EPOCH" ]]; then
        START_EPOCH=$(cat "$ADDON_DHCP_SERVER_START_EPOCH")
    else
        return 1
    fi
    
    # Perform the subtraction
    CURRENT_EPOCH=$(date +%s)
    RESULT=$((CURRENT_EPOCH - START_EPOCH))

    # Compare the result with 3
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
        log_info "Client with MAC address $mac_addr has been added/updated successfully."
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
log_info "Triggered with mode=${MODE}, mac=${MAC_ADDRESS}, hostname=${HOSTNAME}"
if [[ "$MODE" = "add" ]]; then
    read_start_counter
    last_seen=$(date -u +"%Y-%m-%dT%H:%M:%SZ")  # ISO 8601 UTC format

    add_or_update_dhcp_client "$DB_PATH" "$MAC_ADDRESS" "$HOSTNAME" "$last_seen" "$DHCP_SERVER_START_COUNTER"

elif [[ "$MODE" = "old" ]]; then

    # at dnsmasq startup we get a bunch of these 'old' updates -- we need to filter them out
    dnsmasq_just_started
    if [[ $? -eq 1 ]]; then 
        log_info "Detected startup LEASE processing and ignoring it"
    else
        log_info "Updating lease"
        read_start_counter
        last_seen=$(date -u +"%Y-%m-%dT%H:%M:%SZ")  # ISO 8601 UTC format
        add_or_update_dhcp_client "$DB_PATH" "$MAC_ADDRESS" "$HOSTNAME" "$last_seen" "$DHCP_SERVER_START_COUNTER"
    fi
fi
