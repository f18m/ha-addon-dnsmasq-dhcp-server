#!/bin/sh
# Requests a DHCP lease from the dnsmasq under test using busybox udhcpc.
# The interface is NOT reconfigured (the container keeps its docker-assigned IP):
# the lease obtained is just written as JSON to /results/<name>.json
# Usage: dhcp-client.sh <hostname>
NAME="$1"
MAC="$(cat /sys/class/net/eth0/address)"
RESULT="/results/${NAME}.json"

cat > /tmp/udhcpc-script.sh <<'SCRIPT'
#!/bin/sh
if [ "$1" = "bound" ] || [ "$1" = "renew" ]; then
    printf '{"leased": true, "name": "%s", "mac": "%s", "ip": "%s", "dns": "%s", "router": "%s", "domain": "%s", "ntp": "%s", "lease": %s}\n' \
        "$NAME" "$MAC" "$ip" "$dns" "$router" "$domain" "$ntpsrv" "${lease:-0}" > "$RESULT"
fi
SCRIPT
chmod +x /tmp/udhcpc-script.sh

export NAME MAC RESULT
# -B: ask the server to reply in broadcast, since the client has no IP address assigned yet
# -n: exit if no lease; -q: quit after obtaining lease; -t/-T: retries / timeout
if udhcpc -i eth0 -B -n -q -t 3 -T 2 -x "hostname:${NAME}" -s /tmp/udhcpc-script.sh; then
    echo "dhcp-client ${NAME}: lease obtained"
else
    echo "dhcp-client ${NAME}: no lease obtained"
    printf '{"leased": false, "name": "%s", "mac": "%s"}\n' "$NAME" "$MAC" > "$RESULT"
fi
exit 0
