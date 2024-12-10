package uibackend

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/miekg/dns"
)

// chaosTXTQuery performs a DNS CHAOS TXT query against a specified DNS server.
func chaosTXTQuery(server, query string, timeout time.Duration) ([]string, error) {
	// Create a new DNS client.
	c := new(dns.Client)
	c.Timeout = timeout

	// Create a new DNS message.
	m := new(dns.Msg)
	m.Id = dns.Id()
	//m.RecursionDesired = true

	// Add the CHAOS TXT query to the message.
	m.Question = append(m.Question, dns.Question{
		Name:   query + ".",
		Qtype:  dns.TypeTXT,
		Qclass: dns.ClassCHAOS})

	// Send the DNS query.
	r, _, err := c.ExchangeContext(context.Background(), m, server)
	if err != nil {
		return nil, err
	}

	// Check for errors in the response.
	if r.Rcode != dns.RcodeSuccess {
		return nil, fmt.Errorf("invalid answer name %s after querying for %s: %s", query, server, dns.RcodeToString[r.Rcode])
	}

	// Extract the TXT record from the response.
	var txt []string
	for _, ans := range r.Answer {
		if t, ok := ans.(*dns.TXT); ok {
			txt = append(txt, t.Txt...)
			break
		}
	}

	return txt, nil
}

func chaosTXTQueryInteger(server, query string, timeout time.Duration) (int, error) {
	// Invoke chaosTXTQuery to get the string value.
	strVal, err := chaosTXTQuery(server, query, timeout)
	if err != nil {
		return 0, err
	}
	if len(strVal) != 1 {
		return 0, err
	}

	// Convert the string value to an integer.
	var intVal int
	_, err = fmt.Sscan(strVal[0], &intVal)
	if err != nil {
		return 0, fmt.Errorf("failed to convert TXT record '%s' to integer: %w", strVal, err)
	}

	return intVal, nil
}

func getDnsStats() (DnsServerStats, error) {

	// this code is meant to be executed on the same machine/container where dnsmasq is running, so:
	dnsServer := "localhost"

	// since the server is local, the max query duration is expected to be small
	dnsTimeout := time.Duration(500 * time.Millisecond)

	ret := DnsServerStats{}

	// From dnsmasq manpage:
	// "The domain names are cachesize.bind, insertions.bind, evictions.bind, misses.bind,
	// hits.bind, auth.bind and servers.bind unless disabled at compile-time."

	// Start querying all cache-related stats
	var intStat int
	var err, lastErr error
	intStat, err = chaosTXTQueryInteger(dnsServer, "cachesize.bind", dnsTimeout)
	if err == nil {
		ret.CacheSize = intStat
	} else {
		lastErr = err
	}
	intStat, err = chaosTXTQueryInteger(dnsServer, "insertions.bind", dnsTimeout)
	if err == nil {
		ret.CacheInsertions = intStat
	} else {
		lastErr = err
	}
	intStat, err = chaosTXTQueryInteger(dnsServer, "evictions.bind", dnsTimeout)
	if err == nil {
		ret.CacheEvictions = intStat
	} else {
		lastErr = err
	}
	intStat, err = chaosTXTQueryInteger(dnsServer, "misses.bind", dnsTimeout)
	if err == nil {
		ret.CacheMisses = intStat
	} else {
		lastErr = err
	}
	intStat, err = chaosTXTQueryInteger(dnsServer, "hits.bind", dnsTimeout)
	if err == nil {
		ret.CacheHits = intStat
	} else {
		lastErr = err
	}

	// Interpret the servers.bind output
	var serversEncodedStr []string
	serversEncodedStr, err = chaosTXTQuery(dnsServer, "servers.bind", dnsTimeout)
	if err != nil {
		lastErr = err
	}
	for _, svrStat := range serversEncodedStr {
		// srvStat would look like "8.8.8.8#53 30048 0"
		fields := strings.Fields(svrStat)
		if len(fields) == 3 {
			svr := fields[0]
			queries, err := strconv.Atoi(fields[1])
			if err != nil {
				return ret, fmt.Errorf("failed to convert queries to integer: %w", err)
			}
			failures, err := strconv.Atoi(fields[2])
			if err != nil {
				return ret, fmt.Errorf("failed to convert failures to integer: %w", err)
			}
			ret.UpstreamServers = append(ret.UpstreamServers, DnsUpstreamStats{
				ServerURL:     svr,
				QueriesSent:   queries,
				QueriesFailed: failures,
			})
		}
	}

	return ret, lastErr
}
