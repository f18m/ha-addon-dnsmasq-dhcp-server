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

# Function to add or update a DHCP client in the SQLite3 database
add_dhcp_client() {
    local db_path=$1
    local mac_addr=$2
    local hostname=$3
    local has_static_ip=$4
    local friendly_name=$5
    local last_seen=$6

    # Check if all required arguments are provided
    if [[ -z "$mac_addr" || -z "$hostname" || -z "$has_static_ip" || -z "$friendly_name" || -z "$last_seen" ]]; then
        echo "Error: Missing required arguments."
        echo "Usage: add_dhcp_client <db_path> <mac_addr> <hostname> <has_static_ip> <friendly_name> <last_seen>"
        return 1
    fi

    # Convert boolean value (0 or 1) for has_static_ip to integer
    if [[ "$has_static_ip" == "true" ]]; then
        has_static_ip=1
    elif [[ "$has_static_ip" == "false" ]]; then
        has_static_ip=0
    else
        echo "Error: has_static_ip must be 'true' or 'false'."
        return 1
    fi

    # Create the table if it doesn't exist
    sqlite3 "$db_path" <<EOF
CREATE TABLE IF NOT EXISTS dhcp_clients (
    mac_addr TEXT PRIMARY KEY,
    hostname TEXT,
    has_static_ip INTEGER,
    friendly_name TEXT,
    last_seen TEXT
);
EOF

    # Insert or update the DHCP client data
    sqlite3 "$db_path" <<EOF
INSERT INTO dhcp_clients (mac_addr, hostname, has_static_ip, friendly_name, last_seen)
VALUES ('$mac_addr', '$hostname', $has_static_ip, '$friendly_name', '$last_seen')
ON CONFLICT(mac_addr) DO UPDATE SET
    hostname=excluded.hostname,
    has_static_ip=excluded.has_static_ip,
    friendly_name=excluded.friendly_name,
    last_seen=excluded.last_seen;
EOF

    if [[ $? -eq 0 ]]; then
        echo "Client with MAC address $mac_addr has been added/updated successfully."
    else
        echo "Error: Failed to add/update client."
    fi
}

# TODO
has_static_ip="true"
friendly_name="NA"

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
if [[ "$MODE" = "add" ]]; then
    last_seen=$(date -u +"%Y-%m-%dT%H:%M:%SZ")  # ISO 8601 UTC format
    add_dhcp_client "$DB_PATH" "$MAC_ADDRESS" "$HOSTNAME" "$has_static_ip" "$friendly_name" "$last_seen"
fi
