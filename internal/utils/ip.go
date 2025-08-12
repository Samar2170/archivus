package utils

import (
	"net"
)

func GetPrivateIp() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", HandleError("GetPrivateIp", "Failed to get network interface addresses", err)
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String(), nil
			}
		}
	}
	return "", nil
}
