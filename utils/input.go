package utils

import (
	"context"
	"io"
	"net"
	"regexp"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/chzyer/readline"
)

func UserInput(ctx context.Context, q string, validator func(string) bool) (string, bool) {
	inst, err := readline.New(q)
	if err != nil {
		panic(err)
	}
	line, err := inst.Readline()
	switch {
	case err == io.EOF:
		return "", true
	case err == readline.ErrInterrupt:
		return "", true
	case err != nil:
		logrus.Error(err)
		return "", true
	default:
		return line, false
	}
}

var (
	realmRegex     = regexp.MustCompile("realm:(?P<Name>.*)")
	pcapRegex      = regexp.MustCompile(`(?P<Filename>(?P<Name>.*)\.pcap2)(?:\?(?P<Args>.*))?`)
	gatheringRegex = regexp.MustCompile("gathering:(?P<Title>.*)+")
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
