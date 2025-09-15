package tftp

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	tftp "github.com/pin/tftp/v3"
)

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

func serveFile(path string, rf io.ReaderFrom) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = rf.ReadFrom(f)
	return err
}

// TFTP server only serving the same file regardless of requested path
func StartTFTPServer(addr, defaultImage string, logger *log.Logger) (*tftp.Server, error) {
	readHandler := func(filename string, rf io.ReaderFrom) error {
		base := filepath.Base(strings.TrimSpace(filename))
		if isHexIPv4Name(base) {
			logger.Printf("HexIPv4 '%s' form detected", base)
		}
		return serveFile(defaultImage, rf)
	}

	// Write handler not used.
	srv := tftp.NewServer(readHandler, nil)
	srv.SetTimeout(5 * time.Second)

	go func() {
		logger.Printf("TFTP server listening on %s, serving=%q", addr, defaultImage)
		if err := srv.ListenAndServe(addr); err != nil {
			if logger != nil {
				logger.Printf("TFTP server error: %v", err)
			}
		}
	}()
	return srv, nil
}
