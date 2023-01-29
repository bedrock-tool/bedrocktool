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

func serverURLToName(server string) string {
	host, _, err := net.SplitHostPort(server)
	if err != nil {
		logrus.Fatalf(locale.Loc("invalid_server", locale.Strmap{"Err": err.Error()}))
	}
	return host
}

func ServerInput(ctx context.Context, server string) (address, name string, err error) {
	if server == "" { // no arg provided, interactive input
		var cancelled bool
		server, cancelled = UserInput(ctx, locale.Loc("enter_server", nil))
		if cancelled {
			return "", "", context.Canceled
		}
	}

	if strings.HasPrefix(server, "realm:") { // for realms use api to get ip address
		realmInfo := strings.Split(server, ":")
		id := ""
		if len(realmInfo) == 3 {
			id = realmInfo[2]
		}
		name, address, err = getRealm(ctx, realmInfo[1], id)
		if err != nil {
			return "", "", err
		}
		name = CleanupName(name)
	} else if strings.HasSuffix(server, ".pcap") || strings.HasSuffix(server, ".pcap2") {
		s := strings.Split(server, ".")
		name = strings.Join(s[:len(s)-1], ".")
		address = server
	} else {
		// if an actual server address if given
		// add port if necessary
		address = server
		if len(strings.Split(address, ":")) == 1 {
			address += ":19132"
		}
		name = serverURLToName(address)
	}

	return address, name, nil
}
