# Home Assistant Add-on: Dnsmasq as DHCP server

A DHCP server based on the [dnsmasq](https://thekelleys.org.uk/dnsmasq/doc.html) utility rather than the [ISC dhcpd](https://www.isc.org/dhcp/) utility.
Dnsmasq is on many aspects more feature-complete than the ISC DHCP server. Moreover ISC DHCP is discontinued.
This addon also implements a UI webpage to view the list of DHCP clients with all relevant information that can be obtained through DHCP.

![Supports aarch64 Architecture][aarch64-shield] ![Supports amd64 Architecture][amd64-shield] ![Supports armv7 Architecture][armv7-shield] ![Supports i386 Architecture][i386-shield]

## About

This addon setups and manages a Dnsmasq instance configured to run as a DHCP server (despite the name 'dnsmasq' also provides DHCP server functionalities, not only DNS).

[aarch64-shield]: https://img.shields.io/badge/aarch64-yes-green.svg
[amd64-shield]: https://img.shields.io/badge/amd64-yes-green.svg
[armv7-shield]: https://img.shields.io/badge/armv7-yes-green.svg
[i386-shield]: https://img.shields.io/badge/i386-yes-green.svg

## Features

* **Web-based UI** integrated in Home Assistant to view the list of all DHCP clients; the web UI is responsive and has nice rendering also from mobile phones.
* **UI Instant update**: no need to refresh the UI, whenever a new DHCP client connects to or leaves the network
  the UI gets instantly updated.
* **IP address reservation** using the MAC address: you can associate a specific IP address (even outside
  the DHCP address pool) to particular hosts.
* **Friendly name configuration**: you can provide your own friendly-name to any host (using its MAC address
  as identifier); this is particularly useful to identify the DHCP clients that provide unhelpful hostnames
  in their DHCP requests.
* **NTP and DNS server options**: you can advertise in DHCP OFFER packets whatever NTP and DNS server you want.
* **Past DHCP clients**: the addon keeps track of _any_ DHCP client ever connected to your network, and allows you to check if some important device in your network was connected in the past but somehow has failed to renew its DHCP lease (e.g. it is shut down).

## Web UI

These are screenshots from the addon UI v1.4.0:

<img src="docs/screenshot2.png" alt="WebUI screenshot"/>
<img src="docs/screenshot3.png" alt="WebUI screenshot"/>

The two screenshots show the Current and the Past DHCP clients tabs.
The tables of DHCP clients are updated in real-time (no manual refresh needed) and can be sorted on any column.
Plus they're responsive and thus adapt nicely to small screens (cellphones).

## Development

See [Home Assistant addon guide](https://developers.home-assistant.io/docs/add-ons)

This addon is based on other 2 addons maintained by Home Assistant team:
* https://github.com/home-assistant/addons/tree/master/dnsmasq
* https://github.com/home-assistant/addons/tree/master/dhcp_server

The UI nginx reverse-proxy configuration has been adapted from:
* https://github.com/alexbelgium/hassio-addons/tree/master/photoprism/

## How to Install

Check out the [addon docs](DOCS.md). Open an issue if you hit any problem.
