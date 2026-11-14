package connectinfo

import (
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
)

var (
	experienceRegex = regexp.MustCompile(`^experience:(?P<Name>.+)$`)
	realmRegex      = regexp.MustCompile(`^realm:(?P<Name>.+)$`)
	pcapRegex       = regexp.MustCompile(`(?P<Filename>(?P<Name>.*)\.pcap2)(?:\?(?P<Args>.*))?`)
	xboxRegex       = regexp.MustCompile(`^xbox:(?P<ID>.+)$`)
)

func regexGetParams(r *regexp.Regexp, s string) (params map[string]string) {
	match := r.FindStringSubmatch(s)
	params = make(map[string]string)
	for i, name := range r.SubexpNames() {
		if i > 0 && i <= len(match) {
			params[name] = match[i]
		}
	}
	return params
}

// DefaultPort is the port used for a server address typed without one.
// It is the game's own default in the Add Server screen, and the port
// servers answer NetherNet joins on (play.craftersmc.net only answers
// on 19132; 443 was refused, #359).
const DefaultPort = "19132"

// NormalizeAddress converts a server address to the URL format used by NetherNet.
// If no scheme is specified, https is assumed, and a missing port is DefaultPort.
// A URL with a scheme and no port keeps the scheme's port (443 or 80).
func NormalizeAddress(address string) (string, error) {
	address = strings.TrimSpace(address)
	if address == "" {
		return "", fmt.Errorf("empty address")
	}

	if strings.Contains(address, "://") {
		u, err := url.Parse(address)
		if err != nil {
			return "", fmt.Errorf("parse address: %w", err)
		}
		switch u.Scheme {
		case "http", "https":
		default:
			return "", fmt.Errorf("unsupported scheme %q", u.Scheme)
		}
		if u.Hostname() == "" {
			return "", fmt.Errorf("address has no host: %q", address)
		}
		if u.Port() == "" {
			port := "443"
			if u.Scheme == "http" {
				port = "80"
			}
			u.Host = net.JoinHostPort(u.Hostname(), port)
		}
		return u.String(), nil
	}

	host, port, err := net.SplitHostPort(address)
	if err != nil {
		switch {
		case strings.Contains(err.Error(), "missing port in address"):
			host, port = strings.Trim(address, "[]"), DefaultPort
		case strings.Contains(err.Error(), "too many colons"):
			// A bare IPv6 address.
			host, port = strings.Trim(address, "[]"), DefaultPort
		default:
			return "", fmt.Errorf("parse address: %w", err)
		}
	}
	if port == "" {
		port = DefaultPort
	}
	return "https://" + net.JoinHostPort(host, port), nil
}

func ValidateServerInput(server string) bool {
	if pcapRegex.MatchString(server) {
		return true
	}

	if experienceRegex.MatchString(server) {
		return true
	}

	if realmRegex.MatchString(server) {
		return true
	}

	if xboxRegex.MatchString(server) {
		return true
	}

	address, err := NormalizeAddress(server)
	if err != nil {
		return false
	}
	u, err := url.Parse(address)
	if err != nil {
		return false
	}
	host := u.Hostname()

	ip := net.ParseIP(host)
	if ip != nil {
		return true
	}

	ips, _ := net.LookupIP(host)
	return len(ips) > 0
}
