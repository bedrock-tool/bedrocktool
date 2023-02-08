package utils

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"regexp"
	"strings"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/sirupsen/logrus"
)

func UserInput(ctx context.Context, q string) (string, bool) {
	c := make(chan string)
	go func() {
		fmt.Print(q)
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		r, _ := regexp.Compile(`[\n\r]`)
		answer = string(r.ReplaceAll([]byte(answer), []byte("")))
		c <- answer
	}()

	select {
	case <-ctx.Done():
		return "", true
	case a := <-c:
		return a, false
	}
}

func serverGetHostname(server string) string {
	host, _, err := net.SplitHostPort(server)
	if err != nil {
		logrus.Fatalf(locale.Loc("invalid_server", locale.Strmap{"Err": err.Error()}))
	}
	return host
}

var (
	realmRegex = regexp.MustCompile("realm:(?P<Name>.*)(?::(?P<ID>.*))?")
	pcapRegex  = regexp.MustCompile(`(?P<Filename>(?P<Name>.*)\.pcap2)(?:\?(?P<Args>.*))?`)
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

func ServerInput(ctx context.Context, server string) (string, string, error) {
	// no arg provided, interactive input
	if server == "" {
		var cancelled bool
		server, cancelled = UserInput(ctx, locale.Loc("enter_server", nil))
		if cancelled {
			return "", "", context.Canceled
		}
	}

	// realm
	if realmRegex.MatchString(server) {
		p := regexGetParams(realmRegex, server)
		name, address, err := getRealm(ctx, p["Name"], p["ID"])
		if err != nil {
			return "", "", err
		}
		return address, CleanupName(name), nil
	}

	// old pcap format
	if match, _ := regexp.MatchString(`.*\.pcap$`, server); match {
		return "", "", fmt.Errorf(locale.Loc("not_supported_anymore", nil))
	}

	// new pcap format

	if pcapRegex.MatchString(server) {
		p := regexGetParams(pcapRegex, server)
		return "PCAP!" + p["Filename"], p["Name"], nil
	}

	// normal server dns or ip
	if len(strings.Split(server, ":")) == 1 {
		server += ":19132"
	}
	return server, serverGetHostname(server), nil
}
