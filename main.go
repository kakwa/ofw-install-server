package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	iface := flag.String("i", "enp0s25", "interface to bind (Linux only)")
	tftpRoot := flag.String("tftproot", ".", "TFTP root directory")
	tftpDefault := flag.String("tftpdefault", "", "Default image to serve for IP-hex filenames")
	flag.Parse()

	// Start TFTP server
	loggerTFTP := log.New(os.Stdout, "tftp ", log.LstdFlags)
	_, err := StartTFTPServer(":69", *tftpRoot, *tftpDefault, loggerTFTP)

	if err != nil {
		log.Fatalf("start tftp failure: %v", err)
	}

	loggerRARP := log.New(os.Stdout, "rarp ", log.LstdFlags)
	_, err = StartRARPServer(iface, loggerRARP)

	if err != nil {
		log.Fatalf("start arp failure: %v", err)
	}
	loggerRARP.Printf("RARP server enabled on %s", *iface)

	// Block until termination signal to keep goroutine servers alive
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	sig := <-stop
	log.Printf("received signal %s, exiting", sig)
}
