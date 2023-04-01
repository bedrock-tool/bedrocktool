package utils

import (
	"net"

	"github.com/repeale/fp-go"
)

var privateIPNetworks = []net.IPNet{
	{
		IP:   net.ParseIP("10.0.0.0"),
		Mask: net.CIDRMask(8, 32),
	},
	{
		IP:   net.ParseIP("172.16.0.0"),
		Mask: net.CIDRMask(12, 32),
	},
	{
		IP:   net.ParseIP("192.168.0.0"),
		Mask: net.CIDRMask(16, 32),
	},
}

// IPPrivate checks if ip is private
func IPPrivate(ip net.IP) bool {
	return fp.Some(func(ipNet net.IPNet) bool {
		return ipNet.Contains(ip)
	})(privateIPNetworks)
}

// GetLocalIP returns the non loopback local IP of the host
func GetLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, address := range addrs {
		// check the address type and if it is not a loopback the display it
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return ""
}
