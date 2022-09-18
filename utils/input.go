package utils

import (
	"net"

	"github.com/sirupsen/logrus"
)

func server_url_to_name(server string) string {
	host, _, err := net.SplitHostPort(server)
	if err != nil {
		logrus.Fatalf("Invalid server: %s", err)
	}
	return host
}
