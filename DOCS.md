# Home Assistant Add-on: Dnsmasq-DHCP

## Installation

Follow these steps to get the add-on installed on your system:

1. Add Francesco Montorsi HA addons store by clicking here: [![Open your Home Assistant instance and show the add add-on repository dialog with a specific repository URL pre-filled.](https://my.home-assistant.io/badges/supervisor_add_addon_repository.svg)](https://my.home-assistant.io/redirect/supervisor_add_addon_repository/?repository_url=https%3A%2F%2Fgithub.com%2Ff18m%2Fha-addons-repo)

By doing so you should get to your HomeAssistant configuration page for addon digital archives and you should be asked to add `https://github.com/f18m/ha-addons-repo` to the list. Hit "Add".

2. In the list of add-ons, search for "Francesco Montorsi addons" and then the "Dnsmasq-DHCP" add-on and click on that.

3. Click on the "INSTALL" button.

## How to use

You need to make sure you don't have other DHCP servers running already in your network.
You will also need all details about the network where the DHCP server should be running:

* the netmask
* the gateway IP address (your Internet router typically)
* the DNS server IP addresses (you may use e.g. Google DNS servers)
* an IP address range free to be used to provision addresses to DHCP dynamic clients

## Configuration

The Dnsmasq-DHCP addon configuration is documented in the 'Configuration' tab of the
addon. 
Alternatively check out the comments inside the [addon configuration file](config.yaml).

## Links

- [dnsmasq manual page](https://thekelleys.org.uk/dnsmasq/docs/dnsmasq-man.html)
