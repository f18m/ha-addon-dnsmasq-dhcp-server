# Home Assistant Add-on: Dnsmasq-DHCP

## Installation

Follow these steps to get the add-on installed on your system:

1. Add my HA addons store by clicking here: [![Open your Home Assistant instance and show the add add-on repository dialog with a specific repository URL pre-filled.](https://my.home-assistant.io/badges/supervisor_add_addon_repository.svg)](https://my.home-assistant.io/redirect/supervisor_add_addon_repository/?repository_url=https%3A%2F%2Fgithub.com%2Ff18m%2Fha-addons-repo)

By doing so you should get to your HomeAssistant configuration page for addon digital archives and you should be asked to add `https://github.com/f18m/ha-addons-repo` to the list. Hit "Add".

2. In the list of add-ons, search for "Francesco Montorsi addons" and then the `Dnsmasq-DHCP` add-on and click on that.

3. Click on the "INSTALL" button.

## How to use

You need to make sure you don't have other DHCP servers running already in your network.
You will also need all details about the network where the DHCP server should be running:

* the netmask
* the gateway IP address (your Internet router typically)
* the upstream DNS server IP addresses (e.g. you may use Google DNS servers or Cloudflare quad9 servers)
* the upstream NTP servers
* an IP address range free to be used to provision addresses to DHCP dynamic clients

## Configuration

The `Dnsmasq-DHCP` addon configuration is documented in the 'Configuration' tab of the
addon. 
Alternatively check out the comments inside the [addon configuration file](config.yaml).

In case you want to enable the DNS server, you probably want to configure in the `dhcp_network`
section of the [config.yaml](config.yaml) file a single DNS server with IP `0.0.0.0`.
Such special IP address configures the DHCP server to advertise as DNS server itself.
This has the advantage that you will be able to resolve any DHCP host via an FQDN composed by the
DHCP client hostname plus the DNS domain set using `dns_server.dns_domain` in [config.yaml](config.yaml).
For example if you have a device that is advertising itself as `shelly1-abcd` on DHCP, and you have
configured `home` as your DNS domain, then you can use `shelly1-abcd.home` to refer to that device,
instead of its actual IP address.

## Concepts

### DHCP Pool

The DHCP server needs to be configured with a start and end IP address that define the 
pool of IP addresses automatically managed by the DHCP server and provided dynamically to the clients
that request them.
See also [Wikipedia DHCP page](https://en.wikipedia.org/wiki/Dynamic_Host_Configuration_Protocol)
for more information.

### DHCP Static IP addresses

The DHCP server may be configured to provide a specific IP address
to a specific client (using its [MAC address](https://en.wikipedia.org/wiki/MAC_address) as identifier).
These are _IP address reservations_.
Note that static IP addresses do not need to be inside the DHCP range; indeed quite often the
static IP address reserved lies outside the DHCP range.

### DHCP Friendly Names

Sometimes the hostname provided by the DHCP client to the DHCP server is really awkward and
non-informative, so `Dnsmasq-DHCP` allow users to override that by specifying a human-friendly
name for a particular DHCP client (using its MAC address as identifier).


### Upstream DNS servers

If the DNS server of `Dnsmasq-DHCP` is enabled (by setting `dns_server.enable` to `true`),
then `Dnsmasq-DHCP` maintains a local cache of DNS resolutions but needs to know which
external or _upstream_ DNS servers should be contacted when something in the LAN network 
is asking for a DNS resolution that is not cached.
The upstream servers typically used are:

* Google DNS servers: `8.8.8.8` and `8.8.4.4`
* Cloudflare DNS servers: `1.1.1.1`

but you can actually point `Dnsmasq-DHCP` DNS server to another locally-hosted DNS server
like e.g. the [AdGuard Home](https://github.com/hassio-addons/addon-adguard-home) DNS server
to block ADs in your LAN.


### HomeAssistant mDNS

HomeAssistant runs an [mDNS](https://en.wikipedia.org/wiki/Multicast_DNS) server on port 5353.
This is not impacted in any way by the DNS server functionality offered by this addon.


## Using the Beta version

The _beta_ version of `Dnsmasq-DHCP` is where most bugfixes are first deployed and tested.
Only if they are working fine, they will be merged in the _stable_ version.

Since the beta version does not employ a real version scheme, to make sure you're running
the latest build of the beta, please run:

```
docker pull ghcr.io/f18m/amd64-addon-dnsmasq-dhcp:beta
```

on your HomeAssistant server. 

To switch from the _stable_ version to the _beta_ version, just use:

```
docker pull ghcr.io/f18m/amd64-addon-dnsmasq-dhcp:beta
cd /usr/share/hassio/addons/data/79957c2e_dnsmasq-dhcp && cp -av * ../79957c2e_dnsmasq-dhcp-beta/
```

Then stop the _stable_version of the addon from HomeAssistant UI and start the _beta_ variant.


## Links

- [dnsmasq manual page](https://thekelleys.org.uk/dnsmasq/docs/dnsmasq-man.html)
