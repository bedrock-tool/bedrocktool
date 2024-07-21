package utils

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"regexp"
	"strings"
	"sync/atomic"

	"github.com/bedrock-tool/bedrocktool/utils/gatherings"
	"github.com/sandertv/gophertunnel/minecraft/realms"
	"golang.org/x/term"
)

func UserInput(ctx context.Context, q string, validator func(string) bool) (string, bool) {
	c := make(chan string)
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		panic(err)
	}
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

var (
	realmRegex     = regexp.MustCompile("realm:(?P<Name>.*)(?::(?P<ID>.*))+")
	pcapRegex      = regexp.MustCompile(`(?P<Filename>(?P<Name>.*)\.pcap2)(?:\?(?P<Args>.*))?`)
	gatheringRegex = regexp.MustCompile("gathering:(?::(?P<Title>.*))+")
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

func ParseServer(ctx context.Context, server string) (*ConnectInfo, error) {
	// gathering
	if gatheringRegex.MatchString(server) {
		p := regexGetParams(gatheringRegex, server)

		gatheringsClient, err := Auth.Gatherings(ctx)
		if err != nil {
			return nil, err
		}
		gatheringsList, err := gatheringsClient.Gatherings(ctx)
		if err != nil {
			return nil, err
		}
		var gathering *gatherings.Gathering
		for _, gg := range gatheringsList {
			if gg.Title == p["Title"] {
				gathering = gg
				break
			}
		}
		if gathering == nil {
			return nil, errors.New("gathering not foun")
		}

		return &ConnectInfo{
			Gathering: gathering,
		}, nil
	}

	// realm
	if realmRegex.MatchString(server) {
		p := regexGetParams(realmRegex, server)

		realmsList, err := Auth.Realms.Realms(ctx)
		if err != nil {
			return nil, err
		}

		var realm *realms.Realm
		for _, rr := range realmsList {
			if strings.HasPrefix(rr.Name, p["Name"]) {
				realm = &rr
				break
			}
		}

		if realm == nil {
			return nil, errors.New("realm not found")
		}

		return &ConnectInfo{
			Realm: realm,
		}, nil
	}

	// pcap replay
	if pcapRegex.MatchString(server) {
		p := regexGetParams(pcapRegex, server)
		return &ConnectInfo{
			Replay: p["Filename"],
		}, nil
	}

	// normal server dns or ip
	if len(strings.Split(server, ":")) == 1 {
		server += ":19132"
	}
	return &ConnectInfo{
		ServerAddress: server,
	}, nil
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
