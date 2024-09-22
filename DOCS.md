# Home Assistant Add-on: Dnsmasq-DHCP

## Installation

Follow these steps to get the add-on installed on your system:

1. Add Francesco Montorsi HA addons store, see info at https://github.com/f18m/ha-addons-repo
2. Find the "Dnsmasq-DHCP" add-on and click it.
3. Click on the "INSTALL" button.

## How to use

You need to make sure you don't have other DHCP servers running already in your network.
You will also need all details about the network where the DHCP server should be running:

* the netmask
* the gateway IP address (your Internet router typically)
* the DNS server IP addresses (you may use e.g. Google DNS servers)
* an IP address range free to be used to provision addresses to DHCP dynamic clients

## Configuration

The Dnsmasq-DHCP addon configuration is documented in the 'Configuration' tab of this
addon page. 
Alternatively check out the [./config.yaml](addon configuration file).

## Links

- [dnsmasq manual page](https://thekelleys.org.uk/dnsmasq/docs/dnsmasq-man.html)
