package utils

import (
	"context"
	"fmt"
	"net"
	"os"
	"regexp"
	"strings"
	"sync/atomic"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/sirupsen/logrus"
	"golang.org/x/term"
)

func UserInput(ctx context.Context, q string, validator func(string) bool) (string, bool) {
	c := make(chan string)
	oldState, _ := term.MakeRaw(int(os.Stdin.Fd()))
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	go func() {
		fmt.Print(q)

		var answerb []byte
		var b [1]byte

		var done = false
		var validatorRunning atomic.Bool
		var validatorQueued atomic.Bool

		for {
			_, _ = os.Stdin.Read(b[:])

			done = false
			switch b[0] {
			case 0x3:
				c <- ""
				return
			case 0xA:
				fallthrough
			case 0xD:
				done = true
			case 0x8:
				fallthrough
			case 0x7F:
				if len(answerb) > 0 {
					answerb = answerb[:len(answerb)-1]
				}
			default:
				if b[0] >= 0x20 {
					answerb = append(answerb, b[0])
				}
			}

			if done {
				break
			}

			fmt.Printf("\r%s%s\033[K", q, string(answerb))

			if validator != nil {
				validatorQueued.Store(true)
				if validatorRunning.CompareAndSwap(false, true) {
					go func() {
						for validatorQueued.Load() {
							validatorQueued.Store(false)
							valid := validator(string(answerb))
							validatorRunning.Store(false)
							if done {
								return
							}
							var st = "❌"
							if valid {
								st = "✅"
							}
							fmt.Printf("\r%s%s  %s\033[K\033[%dD", q, string(answerb), st, 4)
						}
					}()
				}
			}
		}

		print("\r\n")
		answer := string(answerb)
		c <- answer
		validatorQueued.Store(false)
		done = true
	}()

	select {
	case <-ctx.Done():
		return "", true
	case a := <-c:
		if a == "" {
			return a, true
		}
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
	realmRegex = regexp.MustCompile("realm:(?P<Name>.*)(?::(?P<ID>.*))+")
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

func ParseServer(ctx context.Context, server string) (address, name string, err error) {
	// realm
	if realmRegex.MatchString(server) {
		p := regexGetParams(realmRegex, server)
		name, address, err := getRealm(ctx, p["Name"], p["ID"])
		if err != nil {
			return "", "", err
		}
		return address, CleanupName(name), nil
	}

	// pcap replay
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

func ServerInput(ctx context.Context, server string) (string, string, error) {
	// no arg provided, interactive input
	if server == "" {
		var cancelled bool
		server, cancelled = UserInput(ctx, locale.Loc("enter_server", nil), validateServerInput)
		if cancelled {
			return "", "", context.Canceled
		}
	}
	return ParseServer(ctx, server)
}

func validateServerInput(server string) bool {
	if pcapRegex.MatchString(server) {
		return true
	}

	if realmRegex.MatchString(server) {
		return true // todo
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
