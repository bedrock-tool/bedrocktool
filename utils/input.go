package utils

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"regexp"
	"strings"

	"github.com/sirupsen/logrus"
)

func server_url_to_name(server string) string {
	host, _, err := net.SplitHostPort(server)
	if err != nil {
		logrus.Fatalf("Invalid server: %s", err)
	}
	return host
}

func ServerInput(server string) (address, name string, err error) {
	if server == "" { // no arg provided, interactive input
		fmt.Printf("Enter Server: ")
		reader := bufio.NewReader(os.Stdin)
		server, _ = reader.ReadString('\n')
		r, _ := regexp.Compile(`[\n\r]`)
		server = string(r.ReplaceAll([]byte(server), []byte("")))
	}

	if strings.HasPrefix(server, "realm:") { // for realms use api to get ip address
		realm_info := strings.Split(server, ":")
		id := ""
		if len(realm_info) == 3 {
			id = realm_info[2]
		}
		name, address, err = get_realm(context.Background(), GetRealmsApi(), realm_info[1], id)
		if err != nil {
			return "", "", err
		}
		name = CleanupName(name)
	} else if strings.HasSuffix(server, ".pcap") {
		s := strings.Split(server, ".")
		name = strings.Join(s[:len(s)-1], ".")
		address = server
		/*} else if strings.HasPrefix(server, "gathering:") {
		gathering_info := strings.Split(server, ":")
		if len(gathering_info) < 2 {
			return "", "", fmt.Errorf("use: gathering:<uuid>")
		}
		gathering_id := gathering_info[1]
		g := gatherings.NewGathering(GetTokenSource(), gathering_id)
		address, err = g.Address()
		if err != nil {
			return "", "", err
		}
		return address, "gathering_"+gathering_id, nil
		*/
	} else {
		// if an actual server address if given
		// add port if necessary
		address = server
		if len(strings.Split(address, ":")) == 1 {
			address += ":19132"
		}
		name = server_url_to_name(address)
	}

	return address, name, nil
}
