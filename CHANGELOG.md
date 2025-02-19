# Changelog

For the changelog please check https://github.com/f18m/ha-addon-dnsmasq-dhcp-server/releases


## Migrating from version 2.0 to 2.1

If you have a valid configuration for version 2.0.x, you need to adjust the YAML configuration when migrating
to version 2.1.

1. The top-level "interface" key has been renamed to "interfaces" (plural) and now expects a YAML list of network interface names


## Migrating from version 1.x to 2.x

If you used the addon version 1.x, you need to adjust the YAML configuration to match the 2.x YAML config schema:

1. The "default_lease" and "address_reservation_lease" were moved under the "dhcp_server" key.
1. The "network" key was renamed to "dhcp_network".
1. The "network.interface" key has been moved as top-level key (it's now just "interface").
1. The "ntp" key was renamed to "ntp_servers".
1. The "ip_address_reservations" key was renamed to "dhcp_ip_address_reservations".
1. The "log_dhcp" key was renamed as "dhcp_server.log_requests".
1. The "log_web_ui" key was renamed as "web_ui.log_requests".
1. The "web_ui_port" key was renamed as "web_ui.port".
1. The "reset_dhcp_lease_database_on_reboot" key was renamed as "dhcp_server.reset_dhcp_lease_database_on_reboot".
1. The "dhcp_range.start_ip" and "dhcp_range.end_ip" are now "dhcp_server.start_ip" and "dhcp_server.end_ip".
