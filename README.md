# Home Assistant Add-on: Dnsmasq as DHCP server

A simple DHCP server, more feature-complete than the ISC DHCP server.

![Supports aarch64 Architecture][aarch64-shield] ![Supports amd64 Architecture][amd64-shield] ![Supports armhf Architecture][armhf-shield] ![Supports armv7 Architecture][armv7-shield] ![Supports i386 Architecture][i386-shield]

## About

Setup and manage a Dnsmasq instance as a DHCP server (despite the name 'dnsmasq' also provides DHCP server functionalities, not only DNS).

[aarch64-shield]: https://img.shields.io/badge/aarch64-yes-green.svg
[amd64-shield]: https://img.shields.io/badge/amd64-yes-green.svg
[armhf-shield]: https://img.shields.io/badge/armhf-yes-green.svg
[armv7-shield]: https://img.shields.io/badge/armv7-yes-green.svg
[i386-shield]: https://img.shields.io/badge/i386-yes-green.svg

## Development

See [Home Assistant addon guide](https://developers.home-assistant.io/docs/add-ons)

This addon is based on other 2 addons maintained by Home Assistant team:
* https://github.com/home-assistant/addons/tree/master/dnsmasq
* https://github.com/home-assistant/addons/tree/master/dhcp_server

To build the docker image, go to the folder where you checked out this repo and run:

```
make build
```