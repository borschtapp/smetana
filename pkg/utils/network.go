package utils

import (
	"net"
	"net/url"
)

// IsPublicURL checks if the URL points to a public IP address.
func IsPublicURL(urlString string) bool {
	u, err := url.Parse(urlString)
	if err != nil {
		return false
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}

	ips, err := net.LookupIP(u.Hostname())
	if err != nil {
		return false
	}

	for _, ip := range ips {
		if !ip.IsGlobalUnicast() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || isPrivateIP(ip) {
			return false
		}
	}

	return true
}

func isPrivateIP(ip net.IP) bool {
	if ip4 := ip.To4(); ip4 != nil {
		return ip4[0] == 10 ||
			(ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31) ||
			(ip4[0] == 192 && ip4[1] == 168)
	}
	return false
}
