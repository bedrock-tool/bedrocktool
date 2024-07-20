package utils

import (
	"fmt"
	"unsafe"

	"github.com/sirupsen/logrus"
	"golang.org/x/sys/windows"
)

const (
	NETISO_FLAG_FORCE_COMPUTE_BINARIES = 0x1
)

type SID_AND_ATTRIBUTES struct {
	appContainerSid *windows.SID
	attributes      uint32
}

type INET_FIREWALL_AC_CAPABILITIES struct {
	count        uint32
	capabilities *SID_AND_ATTRIBUTES
}

type INET_FIREWALL_AC_BINARIES struct {
	count    uint32
	binaries *uint16
}

type AppContainer struct {
	appContainerSid  *windows.SID
	userSid          *windows.SID
	appContainerName *uint16
	displayName      *uint16
	description      *uint16
	capabilities     INET_FIREWALL_AC_CAPABILITIES
	binaries         INET_FIREWALL_AC_BINARIES
	workingDirectory *uint16
	packageFullName  *uint16
}

var (
	modFirewallapi                        = windows.NewLazySystemDLL("Firewallapi.dll")
	NetworkIsolationEnumAppContainers     = modFirewallapi.NewProc("NetworkIsolationEnumAppContainers")
	NetworkIsolationGetAppContainerConfig = modFirewallapi.NewProc("NetworkIsolationGetAppContainerConfig")
	NetworkIsolationSetAppContainerConfig = modFirewallapi.NewProc("NetworkIsolationSetAppContainerConfig")
	NetworkIsolationFreeAppContainers     = modFirewallapi.NewProc("NetworkIsolationFreeAppContainers")
)

func Netisolation() error {
	var count uint32
	var array *AppContainer
	ret, _, err := NetworkIsolationEnumAppContainers.Call(
		uintptr(NETISO_FLAG_FORCE_COMPUTE_BINARIES),
		uintptr(unsafe.Pointer(&count)),
		uintptr(unsafe.Pointer(&array)),
	)
	if ret != 0 {
		return fmt.Errorf("failed to enumerate app containers: %w", err)
	}
	defer NetworkIsolationFreeAppContainers.Call(uintptr(unsafe.Pointer(array)))

	var countConf uint32
	var arrayConf *SID_AND_ATTRIBUTES
	ret, _, err = NetworkIsolationGetAppContainerConfig.Call(uintptr(unsafe.Pointer(&countConf)), uintptr(unsafe.Pointer(&arrayConf)))
	if ret != 0 {
		return fmt.Errorf("failed to get app container configs: %w", err)
	}
	config := unsafe.Slice(arrayConf, countConf)

	for _, ac := range unsafe.Slice(array, count) {
		moniker := windows.UTF16PtrToString(ac.appContainerName)
		displayName := windows.UTF16PtrToString(ac.displayName)

		if moniker == "Microsoft.MinecraftUWP_8wekyb3d8bbwe" {
			for _, conf := range config {
				if conf.appContainerSid.Equals(ac.appContainerSid) {
					//logrus.Info("NetIsolation Loopback was already configured")
					return nil
				}
			}
			config = append(config, SID_AND_ATTRIBUTES{
				appContainerSid: ac.appContainerSid,
				attributes:      0,
			})

			ret, _, err := NetworkIsolationSetAppContainerConfig.Call(uintptr(len(config)), uintptr(unsafe.Pointer(unsafe.SliceData(config))))
			if ret != 0 {
				return fmt.Errorf("failed to set app container configs: %w", err)
			}
			logrus.Infof("NetIsolation Loopback allowed for \"%s\"", displayName)
			return nil
		}
	}

	//logrus.Info("You dont have Minecraft Bedrock installed")
	return nil
}
