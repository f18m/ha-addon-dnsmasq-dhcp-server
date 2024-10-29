/*
    Contains all client-side logic to format the tables,
    handle websocket events, handle tabs, etc.
*/

/* GLOBALS */
/* Note that all variables prefixed with "templated_" are globals as well, defined in the HTML template file */
var table_current = null;
var table_past = null;
var dhcp_clients_ws = new WebSocket(templated_webSocketURI);


/* FUNCTIONS */
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

    // Calculate the remaining time in hours, minutes, and seconds
    const hoursLeft = Math.floor(timeDifference / (1000 * 60 * 60));
    const minutesLeft = Math.floor((timeDifference % (1000 * 60 * 60)) / (1000 * 60));
    const secondsLeft = Math.floor((timeDifference % (1000 * 60)) / 1000);

    // Format the remaining time as a string "HH:MM:SS"
    return `${hoursLeft.toString().padStart(2, '0')}:${minutesLeft.toString().padStart(2, '0')}:${secondsLeft.toString().padStart(2, '0')}`;
}

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
            responsive: true
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
            responsive: true
        });
}

function initAll() {
    initCurrentTable()
    initPastTable()
    initTabs()
}

function processWebSocketEvent(event) {

    try {
        var data = JSON.parse(event.data);
    } catch (error) {
        console.error('Error while parsing JSON:', error);
    }

    var message = document.getElementById("message");

    if (data === null) {
        console.log("Websocket connection: received an empty JSON");

        // clear the table
        table_current.clear().draw();
        table_past.clear().draw();

        message.innerText = "No DHCP clients so far.";

    } else if (!("current_clients" in data) || 
                !("past_clients" in data)) {
        console.error("Websocket connection: expecting a JSON matching the golang WebSocketMessage type, received something else", data);

        // clear the table
        table_current.clear().draw();
        table_past.clear().draw();

        message.innerText = "Internal error. Please report upstream together with Javascript logs.";

    } else {
        // console.log("DEBUG:" + JSON.stringify(data))
        console.log("Websocket connection: received " + data.current_clients.length + " current clients from websocket");
        console.log("Websocket connection: received " + data.past_clients.length + " past clients from websocket");

        // rerender the CURRENT table
        tableData = [];
        dhcp_addresses_used = 0;
        dhcp_static_ip = 0;
        data.current_clients.forEach(function (item, index) {
            console.log(`CurrentItem ${index + 1}:`, item);

            if (item.is_inside_dhcp_pool)
                dhcp_addresses_used += 1;

            static_ip_str = "NO";
            if (item.has_static_ip) {
                static_ip_str = "YES";
                dhcp_static_ip += 1;
            }

            external_link_symbol="🡕"
            //external_link_symbol="⧉"
            if (item.evaluated_link) {
                link_str = "<a href=\"" + item.evaluated_link + "\" target=\"_blank\">" + item.evaluated_link + "</a> " + external_link_symbol
            } else {
                link_str = "N/A"
            }

            // append new row
            tableData.push([index + 1,
                item.friendly_name, item.lease.hostname, link_str,
                item.lease.ip_addr, item.lease.mac_addr, 
                formatTimeLeft(item.lease.expires), static_ip_str]);
        });
        table_current.clear().rows.add(tableData).draw(false /* do not reset page position */);

        // rerender the PAST table
        tableData = [];
        data.past_clients.forEach(function (item, index) {
            console.log(`PastItem ${index + 1}:`, item);

            static_ip_str = "NO";
            if (item.has_static_ip) {
                static_ip_str = "YES";
            }

            // append new row
            tableData.push([index + 1,
                item.friendly_name, item.past_info.hostname, 
                item.past_info.mac_addr, static_ip_str, 
                formatTimeSince(item.past_info.last_seen), item.notes]);
        });
        table_past.clear().rows.add(tableData).draw(false /* do not reset page position */);

        // compute DHCP pool usage
        var usagePerc = 0
        if (templated_dhcpPoolSize > 0) {
            usagePerc = 100 * dhcp_addresses_used / templated_dhcpPoolSize

            // truncate to only 1 digit accuracy
            usagePerc = Math.round(usagePerc * 10) / 10
        }

        // format server uptime
        uptime_str = formatTimeSince(templated_dhcpServerStartTime)

        // update the message
        message.innerHTML = "<span class='boldText'>" + data.current_clients.length + " DHCP current clients</span> hold a DHCP lease.<br/>" + 
                            dhcp_static_ip + " have a static IP address configuration.<br/>" +
                            dhcp_addresses_used + " are within the DHCP pool. DHCP pool usage is at " + usagePerc + "%.<br/>" +
                            "<span class='boldText'>" + data.past_clients.length + " DHCP past clients</span> contacted the server some while ago but failed to do so since last DHCP server restart, " + 
                            uptime_str + " hh:mm:ss ago.<br/>";
    }
}

// websocket
dhcp_clients_ws.onopen = function (event) {
    console.log("Websocket connection to " + templated_webSocketURI + " was successfully opened");
};

dhcp_clients_ws.onclose = function (event) {
    console.log("Websocket connection closed", event.code, event.reason, event.wasClean)
}

dhcp_clients_ws.onerror = function (event) {
    console.log("Websocket connection closed due to error", event.code, event.reason, event.wasClean)
}

dhcp_clients_ws.onmessage = function (event) {
    console.log("Websocket received event", event.code, event.reason, event.wasClean)
    processWebSocketEvent(event)
}


// init code
document.addEventListener('DOMContentLoaded', initAll, false);
