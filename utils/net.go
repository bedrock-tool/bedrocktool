package utils

import (
	"fmt"
	"net"
	"strings"

	_ "gioui.org/app/permission/networkstate"
)

func getLocalIp2() (string, error) {
	nc, err := net.Dial("tcp4", "1.1.1.1:80")
	if err != nil {
		return "", err
	}
	defer nc.Close()
	return nc.LocalAddr().(*net.TCPAddr).IP.String(), nil
}

func GetLocalIP() (string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return getLocalIp2()
	}

	for _, iface := range interfaces {
		fmt.Printf("%v\n", iface)
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		if iface.Flags&net.FlagMulticast == 0 {
			continue
		}
		nameLower := strings.ToLower(iface.Name)
		if strings.Contains(nameLower, "tun") ||
			strings.Contains(nameLower, "tap") ||
			strings.Contains(nameLower, "vpn") ||
			strings.Contains(nameLower, "wg") ||
			strings.Contains(nameLower, "tailscale") ||
			strings.Contains(nameLower, "vethernet") {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			if ipNet.IP.To4() != nil && !ipNet.IP.IsLoopback() {
				return ipNet.IP.String(), nil
			}
		}
	}

	return "", fmt.Errorf("no suitable local IP address found")
}
