# Home Assistant Add-on: Dnsmasq-DHCP

## Installation

Follow these steps to get the add-on installed on your system:

1. Add the HA addon store for this addon by clicking here: [![Open your Home Assistant instance and show the add add-on repository dialog with a specific repository URL pre-filled.](https://my.home-assistant.io/badges/supervisor_add_addon_repository.svg)](https://my.home-assistant.io/redirect/supervisor_add_addon_repository/?repository_url=https%3A%2F%2Fgithub.com%2Ff18m%2Fha-addons-repo)

By doing so you should get to your HomeAssistant configuration page for addon digital archives and you should be asked to add `https://github.com/f18m/ha-addons-repo` to the list. Click "Add".

2. In the list of add-ons, search for "Francesco Montorsi addons" and then the `Dnsmasq-DHCP` add-on and click on that. There is also a "BETA" version available, skip it unless you want to try the latest bugfixes and developments.

3. Click on the "INSTALL" button.


## Requirements

You need to make sure you don't have other DHCP servers running already in your network.
You will also need all details about the network(s) where the DHCP server will be running (in other words,
the details of the networks attached to your HomeAssistant server):

* the network interface(s);
* the netmask;
* the gateway IP address (your Internet router typically; in most countries this is provided by the ISP);
* an IP address range free to be used to provision addresses to DHCP dynamic clients
* optionally: the upstream DNS server IP addresses (e.g. you may use Google DNS servers or Cloudflare quad9 servers), if you want to enable the DNS functionality;
* optionally: the upstream NTP servers;

An example for all elements above could be:

* the network interface(s): `eth0`
* the netmask: `255.255.255.0`
* the gateway IP address: `192.168.1.1`
* an IP address range free to be used: `192.168.1.200` - `192.168.1.250`
* upstream DNS server IP addresses: `8.8.8.8`
* upstream NTP servers: `0.north-america.pool.ntp.org` or `0.europe.pool.ntp.org`

Another important requirement is that the server which is running HomeAssistant must
be configured to use a **STATIC IP address** in your network.
In other words, you cannot use DHCP to configure the device where the DHCP server will be running (!!).
You can setup a static IP address from your HomeAssistant through the `Settings->Network` menu:

<img src="docs/ha-network-settings.png" alt="HA screenshot"/>

Once you have collected all these info and verified that you're using a static IP address, you can start editing the `Dnsmasq-DHCP` addon configuration. 
Before getting there, the [Concepts section](#concepts) below provides the meaning for some terms that 
will appear in the [Configuration section](#configuration).


## Concepts

### DHCP Pool

The DHCP server needs to be configured with a start and end IP address that define the 
pool of IP addresses automatically managed by the DHCP server and provided dynamically to the clients
that request them.
See also [Wikipedia DHCP page](https://en.wikipedia.org/wiki/Dynamic_Host_Configuration_Protocol)
for more information.
Please note that the configured network should use private IP addresses both for its start/end IPs.
A private IP is defined in RFC 1918 (IPv4 addresses) and RFC 4193 (IPv6 addresses).
Check [wikipedia page for private networks](https://en.wikipedia.org/wiki/Private_network) for more information.

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


## Configuration

The following YAML file shows all possible `Dnsmasq-DHCP` addon configurations and their default values:

```yaml
# The interfaces on which the DHCP/DNS server will listen
# DHCP requests are listened on port 67
# DNS requests are listened on port 53 (DNS port is configurable via dns_server.port key)
interfaces:
  - enp1s0

# DHCP server configs that apply to all networks later defined in "dhcp_pools"
dhcp_server:
  # Lease time for all DHCP clients except those having an IP address reservation.
  default_lease: 1h

  # Lease time for all DHCP clients having an IP address reservation.
  # The address reservation lease might also be 'infinite' but this is discouraged since
  # it means that the DHCP clients will never come back to this server to refresh their lease
  # and this makes the whole DHCP server less useful... it's better to force the clients
  # to some frequent check-in, since that becomes a basic heartbeat / client health check.
  address_reservation_lease: 1h

  # The addon can detect whether the server which is running the addon has just rebooted;
  # if that's the case and the following flag is set to "true", then the DHCP lease database
  # is reset before starting the DHCP server; this is useful in case a loss of power of the
  # HomeAssistant server means also a loss of power of several/all DHCP clients. In such a case
  # the old DHCP lease database is not useful and actually misleading.
  reset_dhcp_lease_database_on_reboot: false

  # The addon keeps track of all "past DHCP clients", i.e. clients that connected in the past
  # but missed to renew their DHCP lease. These are shown in the "Past DHCP Clients" tab in the web UI.
  # This setting allows to cleanup that view after a certain amount of time. 
  # The default value of 30 days means that you will see DHCP clients that connected up to 30days back,
  # no more. If you set this option to ZERO, then the cleanup of old DHCP clients is disabled and the UI
  # will show any client that has ever connected to the server.
  forget_past_clients_after: 30d

  # Shall every DHCP request be logged?
  log_requests: true

  # DNS domain to advertise in DHCP answers
  dns_domain: lan

  # DNS servers to advertise in DHCP answers.
  # Note that even if online someone is naming these as "primary" and "secondary" DNS servers,
  # the DHCP clients are free to use them in any order they like, or even to use only one of them.
  # There is no "primary" or "secondary".
  # In particular if you use "dns_server.enable=true", then you want to provide in this list ONLY "0.0.0.0"
  # (special IP address which is translated to the IP of the machine running dnsmasq) as DNS server. 
  # If you use "dns_server.enable=false", you want to provide some public DNS server (and you can provide
  # more than one if you like), e.g. 8.8.8.8, 1.1.1.1, etc.
  dns_servers:
    - 0.0.0.0
  
  # NTP servers to provide in DHCP answers.
  # Note that not all clients will honor this setting; some devices will require you to manually configure/change
  # the NTP settings through their custom configuration pages.
  ntp_servers:
    # Where can you find reliable NTP servers? https://www.ntppool.org is your answer.
    # example1: online NTP servers: check https://www.ntppool.org/zone/@ for details about continental zones:
    - 0.europe.pool.ntp.org
    - 1.europe.pool.ntp.org
    - 2.europe.pool.ntp.org
    # example2: another way to go is to use Google NTP:
    #- time1.google.com
    #- time2.google.com
    #- time3.google.com
    # example3: the entry 0.0.0.0 means "the address of the machine running dnsmasq"
    #- 0.0.0.0

# dhcp_pools is the core config for the DHCP server.
# Each entry in the list represents a network segment. 
# You can have multiple entries for the same "interface" (with same "gateway" and "netmask") 
# to provide disjoint IP address ranges within the same network (e.g. if you want to provide .100-120 and .200-220
# IP addresses of the same network).
# You can provide IP ranges of different networks in case e.g. the DHCP server
# is attached to multiple network interfaces (i.e. attached to different networks).
#
# In any case remember that the "gateway" IP address must always be an IP address within the
# network specified by the "start", "end" and "netmask" properties.
dhcp_pools:

    # each DHCP pool starts with the "interface" on which a specific IP range / IP network will be served
  - interface: enp1s0
    start: 192.168.1.50
    # the "end" IP address must always be numerically larger than the "start" IP
    end: 192.168.1.150
    # the "gateway" IP address to advertise in DHCP answers
    gateway: 192.168.1.254
    # the "netmask" to advertise in DHCP answers
    netmask: 255.255.255.0

  # another entry, just for the sake of the example:
  - interface: enp1s0
    start: 192.168.1.220
    end: 192.168.1.230
    gateway: 192.168.1.254
    netmask: 255.255.255.0

# DHCP IP address reservations for special/important devices (identified by MAC address)
dhcp_ip_address_reservations:
  - mac: aa:bb:cc:dd:ee:ff
    name: "An-important-host-with-reserved-IP"
    ip: 192.168.1.15
    # the 'link' property accepts a basic golang template. Available variables are 'mac', 'name' and 'ip'
    # e.g. "http://{{ ip }}/landing/page". It is used to render a link into the "current DHCP clients" tab of the UI.
    link: "http://{{ .ip }}/landing-page/for/this/host"

# DHCP friendly names 
# Sometimes DHCP client devices will report an incomprehensible hostname to the DHCP server.
# This option can be used to remap the hostnames to human-friendly names, via the DHCP protocol.
dhcp_clients_friendly_names:
  - mac: dd:ee:aa:dd:bb:ee
    name: "This is a friendly name to label this host, even if it gets a dynamic IP"
    # the 'link' property accepts a basic golang template. Available variables are 'mac', 'name' and 'ip'
    # e.g. "http://{{ ip }}/landing/page/for/this/dynamic/host"
    link: "http://{{ .ip }}/landing-page/for/this/host"

# DNS server configuration
dns_server:
  # Should this addon provide also a DNS server?
  enable: true
  # On which port the dnsmasq DNS server must listen to?
  port: 53
  # How many entries should be cached on the DNS server to reduce traffic to upstream DNS servers?
  # the max size for this cache is 10k entries according to dnsmasq docs
  cache_size: 10000
  # log_requests will enable logging all DNS requests... which results in a very verbose log!!
  log_requests: false
  # DNS domain to resolve locally
  dns_domain: lan
  # Upstream servers to which queries are forwarded when the answer is not cached locally
  upstream_servers:
    - 8.8.8.8
    - 8.8.4.4

# All settings related to the web UI
web_ui:
  log_activity: false
  # this addon uses "host_network: true" so the internal HTTP server will bind on the interface
  # provided as network.interface and will occupy a port there; the following parameter makes
  # that port configurable to avoid conflicts with other services
  port: 8976
  # defines how frequently the tables in the web UI will refresh;
  # if set to zero, table refresh is disabled
  refresh_interval_sec: 10
```

In case you want to enable the DNS server, you probably want to configure in the `dhcp_server`
section of the [config.yaml](config.yaml) file a single DNS server with IP `0.0.0.0`.
Such special IP address configures the DHCP server to advertise as DNS server itself.
This has the advantage that you will be able to resolve any DHCP host via an FQDN composed by the
DHCP client hostname plus the DNS domain set using `dns_server.dns_domain` in [config.yaml](config.yaml).
For example if you have a device that is advertising itself as `shelly1-abcd` on DHCP, and you have
configured `home` as your DNS domain, then you can use `shelly1-abcd.home` to refer to that device,
instead of its actual IP address.


## Using the Beta version

The _beta_ version of `Dnsmasq-DHCP` is where most bugfixes are first deployed and tested.
Only if they are working fine, they will be merged in the _stable_ version.

Since the beta version of `Dnsmasq-DHCP` does not use a real version scheme, to make sure you're running
the latest build of the beta, please run:

```sh
docker pull ghcr.io/f18m/amd64-addon-dnsmasq-dhcp:beta
```

on your HomeAssistant server. 

To switch from the _stable_ version to the _beta_ version, without loosing the list of enrolled
DHCP clients, their lease times and the list of the old DHCP clients, just use:

```sh
docker pull ghcr.io/f18m/amd64-addon-dnsmasq-dhcp:beta
cd /usr/share/hassio/addons/data/79957c2e_dnsmasq-dhcp && cp -av * ../79957c2e_dnsmasq-dhcp-beta/
```

Then stop the _stable_version of the addon from HomeAssistant UI and start the _beta_ variant.


## Development

To test changes to `Dnsmasq-DHCP` locally, before deployment of the new addon, you can use:

```sh
make test-docker-image
```

To verify that the whole "setup chain" works correctly; you can check
* webui-backend ability to read the configuration file `test-options.json`
* dnsmasq-init script
* dnsmasq resulting configuration file

If you're working on the web UI backend you can use

```sh
make test-docker-image-live
```

and then launch a browser on http://localhost:8976 to verify the look&feel of the UI.
In such mode, there are no real DHCP clients but you can simulate a past DHCP client with

```sh
make test-database-add-entry
```


## Links

- [dnsmasq manual page](https://thekelleys.org.uk/dnsmasq/docs/dnsmasq-man.html)
