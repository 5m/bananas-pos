package app

import (
	"net"
	"net/netip"
)

func hostIPAddress() string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return ""
	}

	addrs := make([]netip.Addr, 0)
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		ifaceAddrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range ifaceAddrs {
			prefix, err := netip.ParsePrefix(addr.String())
			if err != nil {
				continue
			}
			ip := prefix.Addr()
			if !ip.IsValid() || ip.IsLoopback() || !ip.IsGlobalUnicast() {
				continue
			}
			addrs = append(addrs, ip)
		}
	}

	return preferredHostIP(addrs)
}

func preferredHostIP(addrs []netip.Addr) string {
	var fallbackIPv4 string
	var fallbackIPv6 string

	for _, addr := range addrs {
		if !addr.IsValid() || addr.IsLoopback() || !addr.IsGlobalUnicast() {
			continue
		}

		if addr.Is4() {
			if addr.IsPrivate() {
				return addr.String()
			}
			if fallbackIPv4 == "" {
				fallbackIPv4 = addr.String()
			}
			continue
		}

		if fallbackIPv6 == "" {
			fallbackIPv6 = addr.String()
		}
	}

	if fallbackIPv4 != "" {
		return fallbackIPv4
	}
	return fallbackIPv6
}
