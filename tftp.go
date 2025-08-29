package main

import (
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	tftp "github.com/pin/tftp/v3"
)

// StartTFTPServer starts a TFTP server using github.com/pin/tftp.
// If a client requests a filename that is exactly the 8-hex-digit representation
// of an IPv4 address (case-insensitive), the server responds with the default image.
func StartTFTPServer(addr, rootDir, defaultImage string, logger *log.Logger) (*tftp.Server, error) {
	readHandler := func(filename string, rf io.ReaderFrom) error {
		base := filepath.Base(strings.TrimSpace(filename))
		// When the requested file name is an IPv4 hex (8 hex chars), serve the default image.
		if isHexIPv4Name(base) {
			if defaultImage == "" {
				return os.ErrNotExist
			}
			return serveFile(defaultImage, rf)
		}

		// Otherwise, serve from the root directory, safely.
		clean := filepath.Clean(base)
		full := filepath.Join(rootDir, clean)
		// Prevent path traversal outside the root
		if !withinRoot(rootDir, full) {
			return os.ErrPermission
		}
		return serveFile(full, rf)
	}

	// Write handler not used.
	srv := tftp.NewServer(readHandler, nil)
	srv.SetTimeout(5 * time.Second)

	go func() {
		if logger != nil {
			logger.Printf("TFTP server listening on %s, root=%q default=%q", addr, rootDir, defaultImage)
		}
		if err := srv.ListenAndServe(addr); err != nil {
			if logger != nil {
				logger.Printf("TFTP server error: %v", err)
			}
		}
	}()
	return srv, nil
}

func serveFile(path string, rf io.ReaderFrom) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = rf.ReadFrom(f)
	return err
}

func isHexIPv4Name(name string) bool {
	if len(name) != 8 {
		return false
	}
	for i := 0; i < 8; i++ {
		c := name[i]
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return false
		}
	}
	return true
}

func withinRoot(root, full string) bool {
	rootAbs, err1 := filepath.Abs(root)
	fullAbs, err2 := filepath.Abs(full)
	if err1 != nil || err2 != nil {
		return false
	}
	rootClean := filepath.Clean(rootAbs)
	fullClean := filepath.Clean(fullAbs)
	return strings.HasPrefix(fullClean+string(os.PathSeparator), rootClean+string(os.PathSeparator)) || fullClean == rootClean
}

// ipToHexString returns the 8-hex-digit uppercase representation of an IPv4 address.
func ipToHexString(ip net.IP) string {
	v4 := ip.To4()
	if v4 == nil {
		return ""
	}
	// Manual format avoids import of fmt here to keep deps minimal in hot path
	const hexdigits = "0123456789ABCDEF"
	buf := make([]byte, 8)
	for i := 0; i < 4; i++ {
		b := v4[i]
		buf[i*2] = hexdigits[b>>4]
		buf[i*2+1] = hexdigits[b&0x0F]
	}
	return string(buf)
}
