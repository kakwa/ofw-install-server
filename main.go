package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	iface := flag.String("iface", "enp0s25", "interface to bind")
	tftpRoot := flag.String("tftproot", ".", "TFTP root directory")
	tftpDefault := flag.String("tftpdefault", "", "Default image to serve for IP-hex filenames")
	// BOOTP flags
	bootpEnable := flag.Bool("bootp", false, "Enable built-in BOOTP/DHCP server")
	bootpRootPath := flag.String("bootp-rootpath", "", "Root-path option (optional)")
	bootpFilename := flag.String("bootp-filename", "", "Filename/bootfile option (optional)")
	flag.Parse()

	// Start TFTP server
	loggerTFTP := log.New(os.Stdout, "tftp ", log.LstdFlags)
	_, err := StartTFTPServer(":69", *tftpRoot, *tftpDefault, loggerTFTP)

	if err != nil {
		log.Fatalf("start tftp failure: %v", err)
	}

	loggerRARP := log.New(os.Stdout, "rarp ", log.LstdFlags)
	allocator, serverIP, err := StartRARPServer(iface, loggerRARP)

	if err != nil {
		log.Fatalf("start arp failure: %v", err)
	}
	loggerRARP.Printf("RARP server enabled on %s", *iface)

	// Start BOOTP server if enabled
	if *bootpEnable {
		loggerBOOTP := log.New(os.Stdout, "bootp ", log.LstdFlags)
		// Defaults for router and next-server are the serverIP
		_, err = StartBOOTPServer(*iface, ":67", allocator, serverIP, *bootpRootPath, *bootpFilename, loggerBOOTP)
		if err != nil {
			log.Fatalf("start bootp failure: %v", err)
		}
		loggerBOOTP.Printf("BOOTP server enabled on %s with pool %s-%s", *iface, allocator.start, allocator.end)
	}

	// Block until termination signal to keep goroutine servers alive
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	sig := <-stop
	log.Printf("received signal %s, exiting", sig)
}
