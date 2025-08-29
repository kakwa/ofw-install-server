package main

import (
	"encoding/binary"
	"net"
	"testing"
)

func TestHtons(t *testing.T) {
	cases := []struct {
		in   uint16
		want uint16
	}{
		{0x0000, 0x0000},
		{0x1234, 0x3412},
		{0xabcd, 0xcdab},
		{0xff00, 0x00ff},
	}
	for _, tc := range cases {
		if got := htons(tc.in); got != tc.want {
			t.Fatalf("htons(0x%04x) = 0x%04x, want 0x%04x", tc.in, got, tc.want)
		}
	}
}

func TestParseIncomingRarp(t *testing.T) {
	// Craft minimal RARP request frame
	serverMAC := net.HardwareAddr{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}
	clientMAC := net.HardwareAddr{0xde, 0xad, 0xbe, 0xef, 0x00, 0x01}
	buf := make([]byte, 14+28)
	// Ethernet header
	copy(buf[0:6], serverMAC[:])
	copy(buf[6:12], clientMAC[:])
	binary.BigEndian.PutUint16(buf[12:14], htons(ETH_P_RARP))
	// RARP payload
	o := 14
	binary.BigEndian.PutUint16(buf[o:o+2], htons(1)) // HType Ethernet
	o += 2
	binary.BigEndian.PutUint16(buf[o:o+2], htons(ETH_P_IP)) // PType IPv4
	o += 2
	buf[o] = 6 // HLEN
	o++
	buf[o] = 4 // PLEN
	o++
	binary.BigEndian.PutUint16(buf[o:o+2], htons(RARP_REQUEST)) // Oper
	o += 2
	copy(buf[o:o+6], clientMAC[:]) // SHA (sender MAC)
	o += 6
	copy(buf[o:o+4], []byte{0, 0, 0, 0}) // SPA
	o += 4
	copy(buf[o:o+6], clientMAC[:]) // THA (target is self for whoami)
	o += 6
	copy(buf[o:o+4], []byte{0, 0, 0, 0}) // TPA

	eth, pkt, err := parseIncomingRarp(buf)
	if err != nil {
		t.Fatalf("parseIncomingRarp error: %v", err)
	}
	if eth.Type != htons(ETH_P_RARP) {
		t.Fatalf("unexpected ethertype: 0x%04x", eth.Type)
	}
	if pkt.Oper != htons(RARP_REQUEST) {
		t.Fatalf("unexpected oper: %d", pkt.Oper)
	}
	if pkt.HLEN != 6 || pkt.PLEN != 4 {
		t.Fatalf("unexpected lengths: HLEN=%d PLEN=%d", pkt.HLEN, pkt.PLEN)
	}
	if pkt.THA != macToArray(clientMAC) {
		t.Fatalf("unexpected THA")
	}
}

func TestBuildRarpReplyAndParse(t *testing.T) {
	serverMAC := net.HardwareAddr{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}
	serverIP := net.IPv4(192, 168, 1, 1)
	clientMAC := net.HardwareAddr{0xde, 0xad, 0xbe, 0xef, 0x00, 0x01}
	clientIP := net.IPv4(192, 168, 1, 100)

	frame, err := buildRarpReply(serverMAC, serverIP, clientMAC, clientIP)
	if err != nil {
		t.Fatalf("buildRarpReply error: %v", err)
	}
	eth, pkt, err := parseIncomingRarp(frame)
	if err != nil {
		t.Fatalf("parseIncomingRarp error: %v", err)
	}
	if eth.Type != htons(ETH_P_RARP) {
		t.Fatalf("unexpected ethertype: 0x%04x", eth.Type)
	}
	if pkt.Oper != htons(RARP_REPLY) {
		t.Fatalf("unexpected oper: %d", pkt.Oper)
	}
	if pkt.SHA != macToArray(serverMAC) || pkt.THA != macToArray(clientMAC) {
		t.Fatalf("unexpected MACs in packet")
	}
	if pkt.SPA != ipToArray(serverIP) || pkt.TPA != ipToArray(clientIP) {
		t.Fatalf("unexpected IPs in packet")
	}
}
