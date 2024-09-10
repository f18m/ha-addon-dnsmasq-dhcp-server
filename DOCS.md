# Home Assistant Add-on: Dnsmasq

## Installation

Follow these steps to get the add-on installed on your system:

1. Navigate in your Home Assistant frontend to **Settings** -> **Add-ons** -> **Add-on store**.
2. Find the "Dnsmasq" add-on and click it.
3. Click on the "INSTALL" button.

## How to use

The add-on has a couple of options available. For more detailed instructions
see below. The basic thing to get the add-on running would be:

1. Start the add-on.

## Configuration

The Dnsmasq add-on can be tweaked to your likings. This section
describes each of the add-on configuration options.

Example add-on configuration:

```yaml
defaults:
  - 8.8.8.8
  - 8.8.4.4
```

### Option: `defaults` (required)

The defaults are upstream DNS servers, where DNS requests that can't
be handled locally, are forwarded to. By default it is configured to have
Google's public DNS servers: `"8.8.8.8", "8.8.4.4"`.
