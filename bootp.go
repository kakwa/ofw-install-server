package main

import (
	"errors"
	"log"
	"net"
	"strings"
	"time"

	dhcp4 "github.com/krolaw/dhcp4"
	"github.com/krolaw/dhcp4/conn"
)

// StartBOOTPServer runs a minimal BOOTP/DHCP server that shares the allocator
// with the RARP server so the same MAC gets the same IP.
//
// - Listens on addr (typically ":67").
// - Uses allocator's pool; router and next-server default to serverIP if nil.
// - Optionally sets root-path and filename if provided (non-empty).
func StartBOOTPServer(ifaceName, addr string, allocator *IPv4Allocator, serverIP net.IP, rootPath string, bootFilename string, logger *log.Logger) (net.PacketConn, error) {
	if allocator == nil || serverIP == nil {
		return nil, errors.New("invalid BOOTP config: missing allocator or serverIP")
	}

	h := &dhcpHandler{
		leaseDuration: 1 * time.Hour,
		allocator:     allocator,
		serverIP:      serverIP.To4(),
		nextServerIP:  serverIP.To4(),
		routerIP:      serverIP.To4(),
		rootPath:      rootPath,
		bootFilename:  bootFilename,
		leases:        make(map[string]net.IP),
	}

	//addr = serverIP.String() + addr
	// Listen on UDP4 port 67, bound to interface.
	s, err := conn.NewUDP4BoundListener(ifaceName, addr)
	if err != nil {
		return nil, err
	}
	go func() {
		if logger != nil {
			logger.Printf("BOOTP server listening on %s, range=%s-%s router=%s next-server=%s filename=%q root-path=%q", addr, allocator.start, allocator.end, serverIP, serverIP, bootFilename, rootPath)
		}
		if serveErr := dhcp4.Serve(s, h); serveErr != nil {
			if logger != nil {
				logger.Printf("BOOTP server error: %v", serveErr)
			}
		}
	}()
	return s, nil
}

type dhcpHandler struct {
	leaseDuration time.Duration
	allocator     *IPv4Allocator
	serverIP      net.IP // used as Server Identifier option
	nextServerIP  net.IP // used as siaddr (next-server)
	routerIP      net.IP
	rootPath      string
	bootFilename  string
	leases        map[string]net.IP // by client MAC string
}

func (h *dhcpHandler) ServeDHCP(pkt dhcp4.Packet, msgType dhcp4.MessageType, options dhcp4.Options) dhcp4.Packet {
	mac := pkt.CHAddr().String()
	requestedIP := net.IP(options[dhcp4.OptionRequestedIPAddress])
	var yiaddr net.IP

	switch msgType {
	case dhcp4.Discover:
		// Offer an available IP.
		if ip := h.findOrAllocateIP(mac, requestedIP); ip != nil {
			yiaddr = ip
			return h.reply(pkt, dhcp4.Offer, yiaddr, options)
		}
		return nil
	case dhcp4.Request:
		// Confirm requested IP or assign known lease.
		if ip := h.findOrAllocateIP(mac, requestedIP); ip != nil {
			yiaddr = ip
			return h.reply(pkt, dhcp4.ACK, yiaddr, options)
		}
		// If we cannot honor the request, NAK.
		return dhcp4.ReplyPacket(pkt, dhcp4.NAK, h.nextServerIP, nil, 0, nil)
	case dhcp4.Release, dhcp4.Decline:
		delete(h.leases, mac)
	}
	return nil
}

func (h *dhcpHandler) reply(pkt dhcp4.Packet, mt dhcp4.MessageType, yiaddr net.IP, req dhcp4.Options) dhcp4.Packet {
	base := dhcp4.Options{
		dhcp4.OptionSubnetMask:       []byte(h.allocator.netw.Mask),
		//dhcp4.OptionRootPath:               []byte(h.routerIP.To4()), 
		dhcp4.OptionRouter:           []byte(h.routerIP.To4()),
		dhcp4.OptionServerIdentifier: []byte(h.serverIP.To4()),
		// Also advertise TFTP server IP as a name string (option 66)
		dhcp4.OptionTFTPServerName: []byte(h.nextServerIP.String()),
	}
	// Default root-path to "<routerIP>:" if none provided, so clients see a non-zero root addr
	if h.rootPath != "" {
		base[dhcp4.OptionRootPath] = []byte(h.rootPath)
	}
	if h.bootFilename != "" {
		base[dhcp4.OptionBootFileName] = []byte(h.bootFilename)
	}
	paramOrder := req[dhcp4.OptionParameterRequestList]
	var ordered []dhcp4.Option
	if len(paramOrder) > 0 {
		ordered = base.SelectOrderOrAll(paramOrder)
	} else {
		ordered = base.SelectOrderOrAll(nil)
	}
	// Set siaddr as next-server IP by passing it as the "server" argument.
	resp := dhcp4.ReplyPacket(pkt, mt, h.nextServerIP, yiaddr, h.leaseDuration, ordered)
	resp.SetSIAddr(h.nextServerIP)
	resp.SetSName([]byte("ofw-install-server"))
	resp.SetFile([]byte(h.bootFilename))
	return resp
}

func (h *dhcpHandler) findOrAllocateIP(mac string, requested net.IP) net.IP {
	// Allocate or retrieve the same IP using the shared allocator.
	var mac6 [6]byte
	hw, err := net.ParseMAC(mac)
	if err == nil {
		copy(mac6[:], hw[:6])
	}
	if ip4, ok := h.allocator.AllocateForMAC(mac6); ok {
		return net.IP(ip4[:]).To4()
	}
	return nil
}

func guessServerIPForSubnet(subnet *net.IPNet) net.IP {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	for _, iface := range ifaces {
		addrs, _ := iface.Addrs()
		for _, a := range addrs {
			var ip net.IP
			switch v := a.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.To4() == nil {
				continue
			}
			if subnet.Contains(ip) {
				return ip.To4()
			}
		}
	}
	return nil
}

func bytesCompare(a, b net.IP) int {
	a4 := a.To4()
	b4 := b.To4()
	for i := 0; i < 4; i++ {
		if a4[i] < b4[i] {
			return -1
		}
		if a4[i] > b4[i] {
			return 1
		}
	}
	return 0
}

func incIP(ip net.IP) net.IP {
	res := append(net.IP(nil), ip.To4()...)
	for i := 3; i >= 0; i-- {
		res[i]++
		if res[i] != 0 {
			break
		}
	}
	return res
}

// Parse helpers from simple strings like "172.24.42.0/24" and "172.24.42.1-172.24.42.32".
func parseCIDR(s string) (*net.IPNet, error) {
	_, n, err := net.ParseCIDR(strings.TrimSpace(s))
	return n, err
}

func parseIPRange(s string) (net.IP, net.IP, error) {
	parts := strings.Split(strings.TrimSpace(s), "-")
	if len(parts) != 2 {
		return nil, nil, errors.New("invalid range, expected start-end")
	}
	start := net.ParseIP(strings.TrimSpace(parts[0]))
	end := net.ParseIP(strings.TrimSpace(parts[1]))
	if start == nil || end == nil {
		return nil, nil, errors.New("invalid ip in range")
	}
	return start.To4(), end.To4(), nil
}
