package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net"
	"os"

	"golang.org/x/sys/unix"
)

// helpers moved to mapping.go and netutil.go

func main() {
	iface := flag.String("i", "eth0", "interface to bind (Linux only)")
	mapping := flag.String("map", "", "comma-separated MAC=IPv4 mappings (e.g. 52:54:00:12:34:56=192.168.1.10,aa:bb:cc:dd:ee:ff=192.168.1.11)")
	verbose := flag.Bool("v", false, "verbose logging")
	flag.Parse()

	ifc, err := ifaceByName(*iface)
	if err != nil {
		log.Fatalf("%v", err)
	}

	serverIP, err := firstIPv4Addr(*iface)
	if err != nil {
		log.Fatalf("%v", err)
	}

	macToIP, err := parseMapping(*mapping)
	if err != nil {
		log.Fatalf("%v", err)
	}

	fd, err := openRawSocket(ifc)
	if err != nil {
		log.Fatalf("%v", err)
	}
	defer unix.Close(fd)

	log.Printf("RARP server on %s (MAC %s, IP %s) listening for requests...", ifc.Name, ifc.HardwareAddr, serverIP)

	reader := bufio.NewReader(os.NewFile(uintptr(fd), fmt.Sprintf("fd%d", fd)))
	for {
		// Read a single Ethernet frame (up to MTU; 1518 is safe default)
		buf := make([]byte, 2048)
		n, err := reader.Read(buf)
		if err != nil {
			log.Fatalf("read: %v", err)
		}
		frame := buf[:n]
		_, pkt, err := parseIncomingRarp(frame)
		if err != nil {
			// not RARP or malformed; skip silently unless verbose
			if *verbose {
				log.Printf("skip frame: %v", err)
			}
			continue
		}

		// Only handle RARP requests
		if pkt.Oper != htons(RARP_REQUEST) {
			if *verbose {
				log.Printf("ignore opcode %d", pkt.Oper)
			}
			continue
		}

		// Target MAC is who is asking for its IP
		var targetMAC [6]byte = pkt.THA
		ip4, ok := macToIP[targetMAC]
		if !ok {
			if *verbose {
				log.Printf("no mapping for %02x:%02x:%02x:%02x:%02x:%02x", targetMAC[0], targetMAC[1], targetMAC[2], targetMAC[3], targetMAC[4], targetMAC[5])
			}
			continue
		}

		reply, err := buildRarpReply(ifc.HardwareAddr, serverIP, net.HardwareAddr(pkt.THA[:]), net.IP(ip4[:]))
		if err != nil {
			log.Printf("build reply: %v", err)
			continue
		}

		// Send using sendto() with SockaddrLinklayer (dst MAC is in frame)
		ll := &unix.SockaddrLinklayer{Ifindex: ifc.Index}
		if err := unix.Sendto(fd, reply, 0, ll); err != nil {
			log.Printf("sendto: %v", err)
			continue
		}

		if *verbose {
			log.Printf("answered RARP for %02x:%02x:%02x:%02x:%02x:%02x -> %d.%d.%d.%d",
				pkt.THA[0], pkt.THA[1], pkt.THA[2], pkt.THA[3], pkt.THA[4], pkt.THA[5],
				ip4[0], ip4[1], ip4[2], ip4[3],
			)
		}
	}
}
