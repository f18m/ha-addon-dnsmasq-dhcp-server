# Home Assistant Add-on: Dnsmasq

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

The Dnsmasq add-on can be tweaked to your likings. This section
describes each of the add-on configuration options.

Example add-on configuration:

```yaml
dns:
  - 8.8.8.8
  - 8.8.4.4
```

### Option: `dns` (required)

The defaults are upstream DNS servers, where DNS requests that can't
be handled locally, are forwarded to. By default it is configured to have
Google's public DNS servers: `"8.8.8.8", "8.8.4.4"`.

TO BE WRITTEN

## Links

- [dnsmasq manual page](https://thekelleys.org.uk/dnsmasq/docs/dnsmasq-man.html)
