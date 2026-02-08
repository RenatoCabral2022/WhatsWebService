package ingest

import (
	"fmt"
	"net"
	"net/url"
)

const maxURLLength = 2048

// ValidateURL checks that a URL is safe to fetch:
//   - max length 2048 characters
//   - scheme must be http or https
//   - no embedded credentials (user:pass@host)
//   - hostname must resolve to a public IP (no private/reserved ranges)
func ValidateURL(rawURL string) error {
	if len(rawURL) > maxURLLength {
		return fmt.Errorf("URL too long (%d chars, max %d)", len(rawURL), maxURLLength)
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("unsupported scheme %q: only http and https are allowed", u.Scheme)
	}

	if u.User != nil {
		return fmt.Errorf("URLs with embedded credentials are not allowed")
	}

	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("URL has no hostname")
	}

	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("DNS resolution failed for %q: %w", host, err)
	}
	if len(ips) == 0 {
		return fmt.Errorf("no DNS results for %q", host)
	}

	for _, ip := range ips {
		if isPrivateIP(ip) {
			return fmt.Errorf("URL resolves to private/reserved IP %s", ip)
		}
	}
	return nil
}

var privateRanges []*net.IPNet

func init() {
	cidrs := []string{
		"127.0.0.0/8",
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"169.254.0.0/16",
		"::1/128",
		"fc00::/7",
		"fe80::/10",
	}
	for _, cidr := range cidrs {
		_, network, _ := net.ParseCIDR(cidr)
		privateRanges = append(privateRanges, network)
	}
}

func isPrivateIP(ip net.IP) bool {
	for _, network := range privateRanges {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}
