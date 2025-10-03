package connectinfo

import (
	"net"
	"regexp"
	"strings"
)

var (
	realmRegex      = regexp.MustCompile("realm:(?P<Name>.*)")
	pcapRegex       = regexp.MustCompile(`(?P<Filename>(?P<Name>.*)\.pcap2)(?:\?(?P<Args>.*))?`)
	gatheringRegex  = regexp.MustCompile("gathering:(?P<Title>.*)+")
	experienceRegex = regexp.MustCompile(`experience:(?P<ID>.+)`)
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

func ValidateServerInput(server string) bool {
	if pcapRegex.MatchString(server) {
		return true
	}

	if realmRegex.MatchString(server) {
		return true // todo
	}

	if gatheringRegex.MatchString(server) {
		return true
	}

	if experienceRegex.MatchString(server) {
		return true
	}

	host, _, err := net.SplitHostPort(server)
	if err != nil {
		if strings.Contains(err.Error(), "missing port in address") {
			host = server
		}
	}

	ip := net.ParseIP(host)
	if ip != nil {
		return true
	}

	ips, _ := net.LookupIP(host)
	return len(ips) > 0
}
