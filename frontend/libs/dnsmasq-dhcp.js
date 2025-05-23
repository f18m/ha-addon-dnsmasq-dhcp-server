/*
    Dnsmasq-DHCP Javascript code

    Contains all client-side logic to format the tables,
    handle websocket events, handle tabs, etc.
*/

/* GLOBALS */
var config = { // this global variable is initialized via setConfig()
    "webSocketURI": null,
    "dhcpServerStartTime": null,
    "dhcpPoolSize": null,
}
// TODO create a "status" dictionary holding all these globals below
var table_current = null;
var table_past = null;
var table_dns_upstreams = null;
var backend_ws = null;
var num_updates = 0;


/* FORMATTING FUNCTIONS */
function formatTimeLeft(unixFutureTimestamp) {
    if (unixFutureTimestamp == 0) {
        return "Never expires";
    }

    // Calculate the difference in milliseconds between the timestamp and the current time
    const now = new Date();
    const timestampInMillis = unixFutureTimestamp * 1000;
    const timeDifference = timestampInMillis - now.getTime();

    // If the time has already passed, return 0
    if (timeDifference <= 0) {
        return "Already expired";
    }

    // Calculate the remaining time in hours, minutes, and seconds
    const hoursLeft = Math.floor(timeDifference / (1000 * 60 * 60));
    const minutesLeft = Math.floor((timeDifference % (1000 * 60 * 60)) / (1000 * 60));
    const secondsLeft = Math.floor((timeDifference % (1000 * 60)) / 1000);

    // Format the remaining time as a string "HH:MM:SS"
    return `${hoursLeft.toString().padStart(2, '0')}:${minutesLeft.toString().padStart(2, '0')}:${secondsLeft.toString().padStart(2, '0')}`;
}

function formatTimeSince(unixPastTimestamp) {
    if (unixPastTimestamp == 0) {
        return "Invalid timestamp";
    }

    // Calculate the difference in milliseconds between the timestamp and the current time
    const now = new Date();
    const timestampInMillis = unixPastTimestamp * 1000;
    const timeDifference = now.getTime() - timestampInMillis;

    // If the time has already passed, return 0
    if (timeDifference <= 0) {
        return "Timestamp in future?";
    }

    // Calculate the time difference in days, hours, minutes, and seconds
    const msecsInDay = 1000 * 60 * 60 * 24;
    const msecsInHour = 1000 * 60 * 60;
    const msecsInMinute = 1000 * 60;

    const days = Math.floor(timeDifference / msecsInDay);
    const hours = Math.floor((timeDifference % msecsInDay) / msecsInHour);
    const minutes = Math.floor((timeDifference % msecsInHour) / msecsInMinute);
    const seconds = Math.floor((timeDifference % msecsInMinute) / 1000);

    // Format the time as a string
    const dayPart = days > 0 ? `${days}d, ` : '';
    const timePart = `${hours.toString().padStart(2, '0')}:${minutes.toString().padStart(2, '0')}:${seconds.toString().padStart(2, '0')}`;
    
    return dayPart + timePart;
}


/* INIT FUNCTIONS */

function initTabs() {
    const tabButtons = document.querySelectorAll('.tabs__pills .btn');
    const tabContents = document.querySelectorAll('.tabs__panels > div');

    if (tabButtons && tabContents) {
        tabButtons.forEach((tabBtn) => {
            tabBtn.addEventListener('click', () => {
                // console.log("click intercepted")
                const tabId = tabBtn.getAttribute('data-id');

                tabButtons.forEach((btn) => btn.classList.remove('active'));
                tabBtn.classList.add('active');

                tabContents.forEach((content) => {
                    content.classList.remove('active');

                    if (content.id === tabId) {
                    content.classList.add('active');
                    }
                });
            });
        });
    }
}

function initCurrentTable() {
    console.log("Initializing table for current DHCP clients");

    // custom sorting for content formatted as HH:MM:SS
    $.fn.dataTable.ext.order['custom-time-order'] = function (settings, colIndex) {
        return this.api().column(colIndex, { order: 'index' }).nodes().map(function (td, i) {
            var time = $(td).text().split(':');
            // convert to seconds (HH * 3600 + MM * 60 + SS)
            return (parseInt(time[0], 10) * 3600) + (parseInt(time[1], 10) * 60) + parseInt(time[2], 10);
        });
    };
    table_current = new DataTable('#current_table', {
            columns: [
                { title: '#', type: 'num' },
                { title: 'Friendly Name', type: 'string' },
                { title: 'Hostname', type: 'string' },
                { title: 'Link', type: 'string' },
                { title: 'IP Address', type: 'ip-address' },
                { title: 'MAC Address', type: 'string' },
                { title: 'Expires in', 'orderDataType': 'custom-date-order' },
                { title: 'Static IP?', type: 'string' }
            ],
            data: [],
            pageLength: 20,
            responsive: true,
            className: 'data-table',
            layout: {
                topStart: {
                    buttons: [
                        'copy', 'excel'
                    ]
                },
                topEnd: 'search',
                bottomStart: 'pageLength'
            }
        });
}

function initPastTable() {
    console.log("Initializing table for past DHCP clients");

    // custom sorting for content formatted as HH:MM:SS
    $.fn.dataTable.ext.order['custom-time-order'] = function (settings, colIndex) {
        return this.api().column(colIndex, { order: 'index' }).nodes().map(function (td, i) {
            var time = $(td).text().split(':');
            // convert to seconds (HH * 3600 + MM * 60 + SS)
            return (parseInt(time[0], 10) * 3600) + (parseInt(time[1], 10) * 60) + parseInt(time[2], 10);
        });
    };
    table_past = new DataTable('#past_table', {
            columns: [
                { title: '#', type: 'num' },
                { title: 'Friendly Name', type: 'string' },
                { title: 'Hostname', type: 'string' },
                { title: 'MAC Address', type: 'string' },
                { title: 'Static IP?', type: 'string' },
                { title: 'Last Seen hh:mm:ss ago', 'orderDataType': 'custom-date-order' },
                { title: 'Notes', type: 'string' }
            ],
            data: [],
            pageLength: 20,
            responsive: true,
            className: 'data-table',
            layout: {
                topStart: {
                    buttons: [
                        'copy', 'excel'
                    ]
                },
                topEnd: 'search',
                bottomStart: 'pageLength'
            }
        });
}

function initDnsUpstreamServersTable() {
    console.log("Initializing table for DNS upstream servers");

    table_dns_upstreams = new DataTable('#dns_upstream_servers', {
            columns: [
                { title: '#', type: 'num' },
                { title: 'Upstream DNS server', type: 'string' },
                { title: 'Queries sent', type: 'num' },
                { title: 'Queries failed', type: 'num' },
            ],
            data: [],
            responsive: true,
            className: 'data-table',
            layout: {
                topStart: null,
                topEnd: null
            }
        });
}

function initTableDarkOrLightTheme() {
    let prefers = window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
    let html = document.querySelector('html');
    
    // see https://datatables.net/manual/styling/dark-mode#Auto-detection
    html.classList.add(prefers);
    html.setAttribute('data-bs-theme', prefers);

    console.log("Adapting the web UI to the auto-detected color-scheme: " + prefers);
}

function initAll() {
    initCurrentTable()
    initPastTable()
    initDnsUpstreamServersTable()
    initTabs()
    initTableDarkOrLightTheme()
}

function setConfig(webSocketURI, dhcpServerStartTime, dhcpPoolSize) {
    // update the global config variable
    config = {
        "webSocketURI": webSocketURI,
        "dhcpServerStartTime": dhcpServerStartTime,
        "dhcpPoolSize": dhcpPoolSize,
    }

    // now that we have the URI of the websocket server, we can open the connection
    backend_ws = new WebSocket(webSocketURI)

    backend_ws.onopen = function (event) {
        console.log("Websocket connection to " + config["webSocketURI"] + " was successfully opened");
    };

    backend_ws.onclose = function (event) {
        console.log("Websocket connection closed", event.code, event.reason, event.wasClean)
        updateLiveIndicator(false)
    }

    backend_ws.onerror = function (event) {
        console.log("Websocket connection closed due to error", event.code, event.reason, event.wasClean)
        updateLiveIndicator(false)
    }

    backend_ws.onmessage = function (event) {
        console.log("Websocket received event", event.code, event.reason, event.wasClean)
        processWebSocketEvent(event)
    }
}


/* DYNAMIC UPDATES PROCESSING FUNCTIONS */

function compareArraysIgnoringColumns(a, b, columnsToIgnore) {
    //console.log("ARRAY A:", a.toString());
    //console.log("ARRAY B:", b.toString());
    //return a.toString() === b.toString();

    if (a.length !== b.length) {
        return false;
    } else {
      // This is a 2D array: first go through rows
      for (var i = 0; i < a.length; i++) {

        // then go through columns
        for (var j = 0; j < a[i].length; j++) {

            if (columnsToIgnore.includes(j)) {
                continue; // Skip the columns to ignore
            }
            if (a[i][j] !== b[i][j]) {
                console.log("DIFFERENT AT ROW" + i + " COLUMN" + j + ": A=" + a[i][j] + " B=" + b[i][j]);
                return false;
            }
        }
      }
      
      return true;
    }
}
  
function processWebSocketDHCPCurrentClients(data) {
    console.log("Websocket connection: received " + data.current_clients.length + " current DHCP clients from websocket");

    // rerender the CURRENT table
    newData = [];
    newTimeLeftColumn = [];
    dhcp_addresses_used = 0;
    dhcp_static_ip = 0;
    data.current_clients.forEach(function (item, index) {
        // console.log(`CurrentItem ${index + 1}:`, item);

        if (item.is_inside_dhcp_pool)
            dhcp_addresses_used += 1;

        static_ip_str = "NO";
        if (item.has_static_ip) {
            static_ip_str = "YES";
            dhcp_static_ip += 1;
        }

        // Apparently not all browsers use fonts supporting the U+1F855 symbol... 
        // E.g. my Android phone does not render it
        //external_link_symbol="🡕" // https://www.compart.com/en/unicode/U+1F855

        // hopefully the U+29C9 symbol is more commonly supported:
        external_link_symbol="⧉" // https://www.compart.com/en/unicode/U+29C9
        if (item.evaluated_link) {
            link_str = "<a href=\"" + item.evaluated_link + "\" target=\"_blank\">" + item.evaluated_link + " " + external_link_symbol + "</a>"
        } else {
            link_str = "N/A"
        }

        // append new row
        time_left_str = formatTimeLeft(item.lease.expires)
        newData.push([index + 1,
            item.friendly_name, item.lease.hostname, link_str,
            item.lease.ip_addr, item.lease.mac_addr, 
            time_left_str, static_ip_str]);
        newTimeLeftColumn.push(time_left_str);
    });

    var index_of_time_left_column = 6;
    var currentData = table_current.data().toArray();
    if (compareArraysIgnoringColumns(currentData, newData, [index_of_time_left_column])) {
        console.log("No change in current DHCP clients, updating only the time-left column");

        // selective update to avoid unwanted resets of the current position (this is specially annoying
        // when using the responsive plugin and the user has expanded a collapsed row!!)
        for (var i = 0; i < currentData.length; i++) {
            table_current.cell(i, index_of_time_left_column).data(newTimeLeftColumn[i]);
        }
        table_current.draw(false);
    } else {
        console.log("There are changes for the current DHCP clients, refreshing the table");
        table_current.clear().rows.add(newData).draw(false /* do not reset page position */);
    }

    return [dhcp_static_ip, dhcp_addresses_used]
}

function processWebSocketDHCPPastClients(data) {
    console.log("Websocket connection: received " + data.past_clients.length + " past DHCP clients from websocket");

    // rerender the PAST table
    newData = [];
    newLastSeenColumn = [];
    data.past_clients.forEach(function (item, index) {
        // console.log(`PastItem ${index + 1}:`, item);

        static_ip_str = "NO";
        if (item.has_static_ip) {
            static_ip_str = "YES";
        }

        // append new row
        last_seen_str = formatTimeSince(item.past_info.last_seen)
        newData.push([index + 1,
            item.friendly_name, item.past_info.hostname, 
            item.past_info.mac_addr, static_ip_str, 
            last_seen_str, item.notes]);
        newLastSeenColumn.push(last_seen_str);
    });

    var index_of_time_last_seen_column = 5;
    var currentData = table_past.data().toArray();
    if (compareArraysIgnoringColumns(currentData, newData, [index_of_time_last_seen_column])) {
        console.log("No change in past DHCP clients, updating only the last-seen column");

        // selective update to avoid unwanted resets of the current position (this is specially annoying
        // when using the responsive plugin and the user has expanded a collapsed row!!)
        for (var i = 0; i < currentData.length; i++) {
            table_past.cell(i, index_of_time_last_seen_column).data(newLastSeenColumn[i]);
        }
        table_past.draw(false);

    } else {
        console.log("There are changes for the past DHCP clients, refreshing the table");
        table_past.clear().rows.add(newData).draw(false /* do not reset page position */);
    }
}

function updateDHCPStatus(data, dhcp_static_ip, dhcp_addresses_used, messageElem) {
    // compute DHCP pool usage
    var usagePerc = 0
    if (config["dhcpPoolSize"] > 0) {
        usagePerc = 100 * dhcp_addresses_used / config["dhcpPoolSize"]

        // truncate to only 1 digit accuracy
        usagePerc = Math.round(usagePerc * 10) / 10
    }

    // format server uptime
    uptime_str = formatTimeSince(config["dhcpServerStartTime"])

    // update the message
    messageElem.innerHTML = "<span class='boldText'>" + data.current_clients.length + " clients</span> currently hold a DHCP lease.<br/>" + 
                        dhcp_static_ip + " clients have a static IP address configuration.<br/>" +
                        dhcp_addresses_used + " clients are within the DHCP pool. DHCP pool contains " + config["dhcpPoolSize"] + " IP addresses and its usage is at " + usagePerc + "%.<br/>" +
                        "<span class='boldText'>" + data.past_clients.length + " past clients</span> contacted the server some time ago but failed to do so since last DHCP server restart, " + 
                        uptime_str + " hh:mm:ss ago.<br/>";
}

function updateDNSStatus(data, messageElem) {
    console.log(`DnsStats:`, data.dns_stats);

    // rerender the UPSTREAM SERVERS table
    tableData = [];
    if (data.dns_stats.upstream_servers_stats != null) {
        data.dns_stats.upstream_servers_stats.forEach(function (item, index) {
            console.log(`Upstream ${index + 1}:`, item);

            // append new row
            tableData.push([index + 1,
                item.server_url, 
                item.queries_sent, 
                item.queries_failed]);
        });
        table_dns_upstreams.clear().rows.add(tableData).draw(false /* do not reset page position */);
    }

    // update the message
    messageElem.innerHTML = 
        "Cache size: <span class='boldText'>" + data.dns_stats.cache_size + "</span><br/>" +
        "Cache insertions: <span class='boldText'>" + data.dns_stats.cache_insertions + "</span><br/>" +
        "Cache evictions: <span class='boldText'>" + data.dns_stats.cache_evictions + "</span><br/>" +
        "Cache misses: <span class='boldText'>" + data.dns_stats.cache_misses + "</span><br/>" +
        "Cache hits: <span class='boldText'>" + data.dns_stats.cache_hits + "</span><br/>"
        ;
}

function updateLiveIndicator(isLive) {
    var liveElem = document.getElementById("websocket_conn_status");

    // change the source image for the live indicator
    liveElem.src = isLive ? "static/ok.png" : "static/ko.png";
    console.log("Updated live indicator to " + liveElem.src);
}

function processWebSocketEvent(event) {

    try {
        var data = JSON.parse(event.data);
    } catch (error) {
        console.error('Error while parsing JSON:', error);
    }

    var dhcpMsgElem = document.getElementById("dhcp_stats_message");
    var dnsMsgElem = document.getElementById("dns_stats_message");

    if (data === null) {
        console.log("Websocket connection: received an empty JSON");

        // clear the table
        table_current.clear().draw();
        table_past.clear().draw();

        dhcpMsgElem.innerText = "No DHCP clients so far.";
        dnsMsgElem.innerText = "No DNS stats so far.";

    } else if (!("current_clients" in data) || 
                !("past_clients" in data) ||
                !("dns_stats" in data)) {
        console.error("Websocket connection: expecting a JSON matching the golang WebSocketMessage type, received something else", data);

        // clear the table
        table_current.clear().draw();
        table_past.clear().draw();

        dhcpMsgElem.innerText = "Internal error. Please report upstream together with Javascript logs.";
        dnsMsgElem.innerText = "Internal error. Please report upstream together with Javascript logs.";

    } else {
        // console.log("DEBUG:" + JSON.stringify(data))
        num_updates += 1
        console.log("****** Update " + num_updates + " ******");

        // process DHCP 
        [dhcp_static_ip, dhcp_addresses_used] = processWebSocketDHCPCurrentClients(data)
        processWebSocketDHCPPastClients(data)
        updateDHCPStatus(data, dhcp_static_ip, dhcp_addresses_used, dhcpMsgElem)

        // process DNS
        updateDNSStatus(data, dnsMsgElem)

        // update live update indicator
        updateLiveIndicator(true)
    }
}


// init code
document.addEventListener('DOMContentLoaded', initAll, false);
