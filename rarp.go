package main

import (
	"encoding/binary"
	"fmt"
	"net"

	"golang.org/x/sys/unix"
)

// EtherType values
const (
	ETH_P_RARP = 0x8035 // Reverse ARP
	ETH_P_IP   = 0x0800
)

// ARP / RARP opcodes
const (
	ARP_REQUEST  = 1
	ARP_REPLY    = 2
	RARP_REQUEST = 3
	RARP_REPLY   = 4
)

// Ethernet header is 14 bytes
// dst(6) | src(6) | ethertype(2)
type EthHdr struct {
	Dst  [6]byte
	Src  [6]byte
	Type uint16
}

// ARP payload per RFC 826/903 (network byte order)
// hrd(2) pro(2) hln(1) pln(1) op(2) sha(6) spa(4) tha(6) tpa(4)
// For our use: hrd=1 (Ethernet), pro=0x0800 (IPv4), hln=6, pln=4
type RarpPacket struct {
	HType uint16 // hardware type
	PType uint16 // protocol type
	HLEN  uint8  // hardware length
	PLEN  uint8  // protocol length
	Oper  uint16 // opcode
	SHA   [6]byte
	SPA   [4]byte
	THA   [6]byte
	TPA   [4]byte
}

func htons(i uint16) uint16 { return (i<<8)&0xff00 | i>>8 }

func openRawSocket(ifc *net.Interface) (int, error) {
	// AF_PACKET/SOCK_RAW for Ethernet frames on Linux
	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_RAW, int(htons(ETH_P_RARP)))
	if err != nil {
		return -1, fmt.Errorf("socket: %w", err)
	}

	// Bind to device + protocol
	ll := &unix.SockaddrLinklayer{Protocol: htons(ETH_P_RARP), Ifindex: ifc.Index}
	if err := unix.Bind(fd, ll); err != nil {
		unix.Close(fd)
		return -1, fmt.Errorf("bind: %w", err)
	}
	return fd, nil
}

func macToArray(mac net.HardwareAddr) (out [6]byte) { copy(out[:], mac[:6]); return }
func ipToArray(ip net.IP) (out [4]byte)             { copy(out[:], ip.To4()[:4]); return }

func buildRarpReply(serverMAC net.HardwareAddr, serverIP net.IP, targetMAC net.HardwareAddr, targetIP net.IP) ([]byte, error) {
	var eth EthHdr
	copy(eth.Dst[:], targetMAC[:6])
	copy(eth.Src[:], serverMAC[:6])
	eth.Type = htons(ETH_P_RARP)

	var pkt RarpPacket
	pkt.HType = htons(1)        // Ethernet
	pkt.PType = htons(ETH_P_IP) // IPv4
	pkt.HLEN = 6
	pkt.PLEN = 4
	pkt.Oper = htons(RARP_REPLY)
	pkt.SHA = macToArray(serverMAC)
	pkt.SPA = ipToArray(serverIP)
	pkt.THA = macToArray(targetMAC)
	pkt.TPA = ipToArray(targetIP)

	buf := make([]byte, 14+28)
	binary.BigEndian.PutUint16(buf[12:14], eth.Type)
	copy(buf[0:6], eth.Dst[:])
	copy(buf[6:12], eth.Src[:])

	o := 14
	binary.BigEndian.PutUint16(buf[o:o+2], pkt.HType)
	o += 2
	binary.BigEndian.PutUint16(buf[o:o+2], pkt.PType)
	o += 2
	buf[o] = pkt.HLEN
	o++
	buf[o] = pkt.PLEN
	o++
	binary.BigEndian.PutUint16(buf[o:o+2], pkt.Oper)
	o += 2
	copy(buf[o:o+6], pkt.SHA[:])
	o += 6
	copy(buf[o:o+4], pkt.SPA[:])
	o += 4
	copy(buf[o:o+6], pkt.THA[:])
	o += 6
	copy(buf[o:o+4], pkt.TPA[:])
	o += 4

	return buf, nil
}

func parseIncomingRarp(b []byte) (EthHdr, RarpPacket, error) {
	var eth EthHdr
	var pkt RarpPacket
	if len(b) < 14+28 {
		return eth, pkt, fmt.Errorf("frame too short: %d", len(b))
	}
	copy(eth.Dst[:], b[0:6])
	copy(eth.Src[:], b[6:12])
	eth.Type = binary.BigEndian.Uint16(b[12:14])
	if eth.Type != htons(ETH_P_RARP) {
		return eth, pkt, fmt.Errorf("not RARP ethertype: 0x%04x", eth.Type)
	}
	o := 14
	pkt.HType = binary.BigEndian.Uint16(b[o : o+2])
	o += 2
	pkt.PType = binary.BigEndian.Uint16(b[o : o+2])
	o += 2
	pkt.HLEN = b[o]
	o++
	pkt.PLEN = b[o]
	o++
	pkt.Oper = binary.BigEndian.Uint16(b[o : o+2])
	o += 2
	copy(pkt.SHA[:], b[o:o+6])
	o += 6
	copy(pkt.SPA[:], b[o:o+4])
	o += 4
	copy(pkt.THA[:], b[o:o+6])
	o += 6
	copy(pkt.TPA[:], b[o:o+4])
	o += 4
	return eth, pkt, nil
}
