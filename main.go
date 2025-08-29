package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"golang.org/x/sys/unix"
)

// helpers moved to mapping.go and netutil.go

func main() {
	iface := flag.String("i", "eth0", "interface to bind (Linux only)")
	mapping := flag.String("map", "", "comma-separated MAC=IPv4 mappings (e.g. 52:54:00:12:34:56=192.168.1.10,aa:bb:cc:dd:ee:ff=192.168.1.11)")
	poolCIDR := flag.String("pool", "", "CIDR pool for dynamic IP assignment (optional)")
	tftpBind := flag.String("tftpaddr", ":69", "TFTP listen address")
	tftpRoot := flag.String("tftproot", ".", "TFTP root directory")
	tftpDefault := flag.String("tftpdefault", "", "Default image to serve for IP-hex filenames")
	nfsAddr := flag.String("nfsaddr", ":2049", "NFS listen address")
	nfsRoot := flag.String("nfsroot", "/nfsroot", "Filesystem path to export over NFS")
	bpAddr := flag.String("bootparamaddr", ":10026", "Bootparam UDP listen address")
	bpRoot := flag.String("bootparamroot", "/nfsroot", "Default NFS root path when -rootfs not set")
	bpRootFS := flag.String("rootfs", "", "Client root filesystem (TFTP path/file), e.g. ./bsd.rd")
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

	// Optional allocator
	var allocator *IPv4Allocator
	if *poolCIDR != "" {
		a, err := NewIPv4AllocatorFromCIDR(*poolCIDR)
		if err != nil {
			log.Fatalf("allocator: %v", err)
		}
		// Reserve server IP and all statically mapped IPs
		a.ReserveIP(serverIP)
		for _, ip4 := range macToIP {
			a.ReserveIP(net.IP(ip4[:]))
		}
		allocator = a
	}

	fd, err := openRawSocket(ifc)
	if err != nil {
		log.Fatalf("%v", err)
	}
	defer unix.Close(fd)

	log.Printf("RARP server on %s (MAC %s, IP %s) listening for requests...", ifc.Name, ifc.HardwareAddr, serverIP)

	// Start TFTP server
	{
		logger := log.New(os.Stdout, "tftp ", log.LstdFlags)
		_, err := StartTFTPServer(*tftpBind, *tftpRoot, *tftpDefault, logger)
		if err != nil {
			log.Fatalf("start tftp: %v", err)
		}
		if *verbose {
			log.Printf("TFTP server enabled at %s (root=%s, default=%s)", *tftpBind, *tftpRoot, *tftpDefault)
		}
		// Small delay to ensure TFTP goroutine starts before entering RARP loop
		time.Sleep(50 * time.Millisecond)
	}

	// Start NFS server
	{
		logger := log.New(os.Stdout, "nfs ", log.LstdFlags)
		_, err := StartNFSServer(NFSConfig{Addr: *nfsAddr, Root: *nfsRoot}, logger)
		if err != nil {
			log.Fatalf("start nfs: %v", err)
		}
		if *verbose {
			log.Printf("NFS exporting %s at %s", *nfsRoot, *nfsAddr)
		}
	}

	// Start Bootparam server (announces either TFTP rootfs or NFS root path)
	{
		logger := log.New(os.Stdout, "bootparam ", log.LstdFlags)
		_, err := StartBootparamUDP(BootparamConfig{
			Addr:     *bpAddr,
			RootPath: *bpRoot,
			RootFS:   *bpRootFS,
		}, serverIP, logger)
		if err != nil {
			log.Fatalf("start bootparam: %v", err)
		}
		if *verbose {
			log.Printf("Bootparam enabled at %s (rootfs=%s, nfsroot=%s)", *bpAddr, *bpRootFS, *bpRoot)
		}
	}

	// BOOTP removed per request; using Bootparams/NFS instead

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
			// Try dynamic allocation if enabled
			if allocator != nil {
				if alloc, ok2 := allocator.AllocateForMAC(targetMAC); ok2 {
					ip4 = alloc
					ok = true
					if *verbose {
						log.Printf("dynamically allocated %d.%d.%d.%d for %02x:%02x:%02x:%02x:%02x:%02x",
							ip4[0], ip4[1], ip4[2], ip4[3],
							targetMAC[0], targetMAC[1], targetMAC[2], targetMAC[3], targetMAC[4], targetMAC[5],
						)
					}
				}
			}
			if !ok {
				if *verbose {
					log.Printf("no mapping for %02x:%02x:%02x:%02x:%02x:%02x and no pool or no free IP",
						targetMAC[0], targetMAC[1], targetMAC[2], targetMAC[3], targetMAC[4], targetMAC[5])
				}
				continue
			}
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
