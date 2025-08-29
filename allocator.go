package main

import (
	"fmt"
	"net"
)

type IPv4Allocator struct {
	netw   *net.IPNet
	start  net.IP
	end    net.IP
	used   map[string]bool
	leases map[[6]byte][4]byte
}

func NewIPv4AllocatorFromCIDR(cidr string) (*IPv4Allocator, error) {
	ip, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, err
	}
	// Compute usable host range
	network := ip.Mask(ipnet.Mask).To4()
	mask := net.IP(ipnet.Mask).To4()
	if network == nil || mask == nil {
		return nil, fmt.Errorf("allocator supports IPv4 only")
	}
	broadcast := make(net.IP, 4)
	for i := 0; i < 4; i++ {
		broadcast[i] = network[i] | ^mask[i]
	}
	// Determine first and last usable
	start := make(net.IP, 4)
	copy(start, network)
	incrementIPv4(start)
	end := make(net.IP, 4)
	copy(end, broadcast)
	decrementIPv4(end)
	if !ipv4LessOrEqual(start, end) {
		return nil, fmt.Errorf("CIDR %s has no usable host addresses", cidr)
	}
	return &IPv4Allocator{
		netw:   ipnet,
		start:  start,
		end:    end,
		used:   make(map[string]bool),
		leases: make(map[[6]byte][4]byte),
	}, nil
}

// ReserveIP marks an IP address as used and unavailable for dynamic assignment.
func (a *IPv4Allocator) ReserveIP(ip net.IP) {
	if ip == nil {
		return
	}
	if v4 := ip.To4(); v4 != nil {
		a.used[v4.String()] = true
	}
}

// AllocateForMAC returns a stable IP for the given MAC. If the MAC already has
// a lease, the same IP is returned. Otherwise the next free IP in the pool is assigned.
func (a *IPv4Allocator) AllocateForMAC(mac [6]byte) (out [4]byte, ok bool) {
	if ip, exists := a.leases[mac]; exists {
		return ip, true
	}
	// Scan from start to end for first free IP
	for ip := cloneIPv4(a.start); ipv4LessOrEqual(ip, a.end); incrementIPv4(ip) {
		if a.used[ip.String()] {
			continue
		}
		var ip4 [4]byte
		copy(ip4[:], ip[:4])
		a.leases[mac] = ip4
		a.used[ip.String()] = true
		return ip4, true
	}
	return out, false
}

func incrementIPv4(ip net.IP) {
	for i := 3; i >= 0; i-- {
		ip[i]++
		if ip[i] != 0 {
			break
		}
	}
}

func decrementIPv4(ip net.IP) {
	for i := 3; i >= 0; i-- {
		ip[i]--
		if ip[i] != 255 {
			break
		}
	}
}

func ipv4LessOrEqual(a, b net.IP) bool {
	for i := 0; i < 4; i++ {
		if a[i] < b[i] {
			return true
		}
		if a[i] > b[i] {
			return false
		}
	}
	return true
}

func cloneIPv4(ip net.IP) net.IP {
	dup := make(net.IP, 4)
	copy(dup, ip[:4])
	return dup
}
