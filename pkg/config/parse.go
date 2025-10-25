package config

import (
	"fmt"
	"net"
	"strings"
)

func ParseNet(s string) (*net.IPNet, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("empty address or CIDR")
	}

	// If it's a plain IP, return a /32 or /128 network
	if ip := net.ParseIP(s); ip != nil {
		if ip4 := ip.To4(); ip4 != nil {
			mask := net.CIDRMask(32, 32)
			return &net.IPNet{IP: ip4.Mask(mask), Mask: mask}, nil
		}
		mask := net.CIDRMask(128, 128)
		return &net.IPNet{IP: ip.Mask(mask), Mask: mask}, nil
	}

	// Try CIDR parse. net.ParseCIDR already returns the network IP masked, but re-mask to be safe.
	ip, ipnet, err := net.ParseCIDR(s)
	if err != nil {
		return nil, err
	}
	ipnet.IP = ip.Mask(ipnet.Mask)
	return ipnet, nil
}

// A very basic email validation. In production, consider using a more robust library.
func isValidEmail(email string) bool {
	if len(email) < 3 || len(email) > 254 {
		return false
	}
	// Check for the presence of an '@' symbol and a domain
	at := strings.Index(email, "@")
	if at == -1 || at == 0 || at == len(email)-1 {
		return false
	}
	return true
}

// A very basic domain validation. In production, consider using a more robust library.
func isValidDomain(domain string) bool {
	if len(domain) < 1 || len(domain) > 253 {
		return false
	}
	// Check for the presence of a top-level domain
	if !strings.Contains(domain, ".") {
		return false
	}
	return true
}