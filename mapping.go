package main

import (
	"fmt"
	"net"
	"strings"
)

func parseMapping(s string) (map[[6]byte][4]byte, error) {
	m := make(map[[6]byte][4]byte)
	if s == "" {
		return m, nil
	}
	pairs := strings.Split(s, ",")
	for _, p := range pairs {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		kv := strings.SplitN(p, "=", 2)
		if len(kv) != 2 {
			return nil, fmt.Errorf("invalid mapping entry: %q (want mac=ipv4)", p)
		}
		macStr := strings.TrimSpace(kv[0])
		ipStr := strings.TrimSpace(kv[1])
		mac, err := net.ParseMAC(macStr)
		if err != nil {
			return nil, fmt.Errorf("parse MAC %q: %w", macStr, err)
		}
		ip := net.ParseIP(ipStr).To4()
		if ip == nil {
			return nil, fmt.Errorf("parse IPv4 %q: invalid", ipStr)
		}
		var mac6 [6]byte
		copy(mac6[:], mac[:6])
		var ip4 [4]byte
		copy(ip4[:], ip[:4])
		m[mac6] = ip4
	}
	return m, nil
}
