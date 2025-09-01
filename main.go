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
	rarpEnable := flag.Bool("rarp", false, "Enable built-in RARP server")
	// TFTP flags
	tftpEnable := flag.Bool("tftp", false, "Enable built-in TFTP")
	tftpFile := flag.String("tftp-file", "", "file to serve using TFTP (step 1)")
	// BOOTP/DHCP flags
	bootpEnable := flag.Bool("bootp", false, "Enable built-in BOOTP/DHCP server")
	bootpRootPath := flag.String("bootp-rootpath", "", "Root-path option (optional)")
	bootpFilename := flag.String("bootp-filename", "", "Filename/bootfile option (optional)")
	// NFS/portmap flags
	nfsEnable := flag.Bool("nfs", false, "enable minimal NFSv2 Server")
	nfsFile := flag.String("nfs-file", "", "file to server using NFSv2 (step 2)")
	// HTTP flags
	httpEnable := flag.Bool("http", false, "Enable built-in HTTP server")
	httpFile := flag.String("http-file", "", "file to serve for all HTTP requests")

	flag.Parse()

	// Start TFTP server
	if *tftpEnable {
		loggerTFTP := log.New(os.Stdout, "tftp ", log.LstdFlags)
		_, err := StartTFTPServer(":69", *tftpFile, loggerTFTP)

		if err != nil {
			log.Fatalf("start tftp failure: %v", err)
		}
	}

	// Start HTTP server if enabled
	if *httpEnable {
		if *httpFile == "" {
			log.Fatalf("http enabled but no --http-file provided")
		}
		loggerHTTP := log.New(os.Stdout, "http ", log.LstdFlags)
		_, err := StartHTTPServer(":80", *httpFile, loggerHTTP)
		if err != nil {
			log.Fatalf("start http failure: %v", err)
		}
	}

	// Start RARP allocator and discover server IP early (used by other services)
	loggerRARP := log.New(os.Stdout, "rarp ", log.LstdFlags)
	allocator, serverIP, err := StartRARPServer(iface, loggerRARP)

	// Optionally start minimal portmap and UDP proxies for mountd/nfs
	if *nfsEnable {
		loggerPM := log.New(os.Stdout, "rpc ", log.LstdFlags)
		// Start local MOUNT and NFS servers serving from TFTP root by default
		_, err := StartMountd(":20048", "/", loggerPM)
		if err != nil {
			log.Fatalf("start mountd failure: %v", err)
		}
		_, err = StartNFSD(":2049", *nfsFile, loggerPM)
		if err != nil {
			log.Fatalf("start nfsd failure: %v", err)
		}
		// Start local portmap that answers GETPORT for our services
		_, err = StartPortmapServer(":111", 20048, 2049, 0, loggerPM)
		if err != nil {
			log.Fatalf("start portmap failure: %v", err)
		}
		loggerPM.Printf("MOUNT/NFS/portmap enabled")
	}

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

	if *rarpEnable {
		// Start RARP server
		if err != nil {
			log.Fatalf("start arp failure: %v", err)
		}
		loggerRARP.Printf("RARP server enabled on %s", *iface)
	}

	// Block until termination signal to keep goroutine servers alive
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	sig := <-stop
	log.Printf("received signal %s, exiting", sig)
}
