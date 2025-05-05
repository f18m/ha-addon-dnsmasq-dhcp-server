/*
Package trackerdb implements methods to query an sqlite3 Database which
is populated by the dnsmasq DHCP script.

For more info about the dnsmasq DHCP script please check
  - dnsmasq man page and --dhcp-script option
  - the /opt/bin/dnsmasq-dhcp-script.sh script part of this repository which is
    where the "tracker DB" gets populated

Q: Why do we need to have such "tracker DB" and we can't just rely on dnsmasq lease file/database?
A: The trackerDB and dnsmasq leases solve two different issues:
the dnsmasq lease file contains the _current_ list of DHCP clients.
Such file/database is persisted to disk (/data is persistent) but if a DHCP client fails to renew its lease
or does not contact dnsmasq server after a dnsmasq restart, then its entry gets deleted from dnsmasq.leases file.
The tracker DB instead is built to maintain an history of _any_ DHCP client that ever connected to the
dnsmasq server. Entries get added to the tracker DB everytime dnsmasq reports a new client and they
get deleted on a configurable time-basis.
Each entry is added with a "last_seen" timestamp and also a "start epoch" which identifies which particular instance
of dnsmasq received traffic from that DHCP client.

Q: What do we use tracker DB for?
A: To implement the "Past DHCP clients" feature of the addon web UI.
Such feature allows to list any DHCP client that was present in the past but that did not contact
the dnsmasq server since its last restart.

Q: Where is the SQL insert/update that adds entries to the tracker DB?
A: The SQL insert/update is done by the dnsmasq-dhcp-script.sh script.

Q: Where is the SQL delete that removes entries from the tracker DB?
A: The SQL delete is done in golang code, in this package.
*/
package trackerdb
