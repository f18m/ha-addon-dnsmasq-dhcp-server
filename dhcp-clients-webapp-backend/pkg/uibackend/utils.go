package uibackend

import (
	"dhcp-clients-webapp-backend/pkg/ippool"
	"fmt"
	"time"
)

func IpPoolToHtmlTemplateRanges(pool ippool.Pool) []HtmlTemplateIpRange {
	var ranges []HtmlTemplateIpRange
	for _, r := range pool.Ranges {
		ranges = append(ranges, HtmlTemplateIpRange{
			Start: r.Start.String(),
			End:   r.End.String(),
		})
	}
	return ranges
}

func LeaseTimeToString(t time.Time) string {

	if t.IsZero() {
		return "Never expires"
	}

	now := time.Now()
	duration := t.Sub(now)
	if duration < 0 {
		return "Expired"
	}

	// compute hours, min, secs
	days := int(duration.Hours()) / 24
	hours := int(duration.Hours()) % 24
	minutes := int(duration.Minutes()) % 60
	seconds := int(duration.Seconds()) % 60

	if days > 0 {
		return fmt.Sprintf("%02dd, %02dh, %02dm, %02ds", days, hours, minutes, seconds)
	} else if hours > 0 {
		return fmt.Sprintf("%02dh, %02dm, %02ds", hours, minutes, seconds)
	} else {
		return fmt.Sprintf("%02dm, %02ds", minutes, seconds)
	}
}
