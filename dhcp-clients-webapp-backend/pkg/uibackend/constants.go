package uibackend

// the dnsmasq lease file is configured in the dnsmasq config file: the value
// here has to match the server config file!
var defaultDnsmasqLeasesFile = "/data/dnsmasq.leases"

// the home assistant addon config file is fixed and cannot be changed actually:
var defaultHomeAssistantConfigFile = "/data/options.json"

// location for our small DB tracking DHCP clients:
var defaultDhcpClientTrackerDB = "/data/trackerdb.sqlite3"

// location for a basic counter that is used to tag entries in the tracker DB
// to understand if they are stale or not
var defaultStartEpoch = "/data/startepoch"

// These absolute paths must be in sync with the Dockerfile
var staticWebFilesDir = "/opt/web/static"
var templatesDir = "/opt/web/templates"

// other constants
var dnsmasqMarkerForMissingHostname = "*"
var websocketRelativeUrl = "/ws"
