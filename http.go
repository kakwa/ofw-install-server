package main

import (
	"io"
	"log"
	"net"
	"net/http"
	"os"
)

// StartHTTPServer starts an HTTP server on :80 that serves a single file for any request path.
// It returns the listener so the caller can manage lifecycle if needed.
func StartHTTPServer(addr string, filePath string, logger *log.Logger) (net.Listener, error) {
	if addr == "" {
		addr = ":80"
	}
	// Pre-open file for efficiency and to fail fast
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	// We will read the file content into memory once; file sizes expected small for boot files
	data, err := io.ReadAll(f)
	f.Close()
	if err != nil {
		return nil, err
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Best-effort content type based on extension, otherwise octet-stream
		// Leave default; clients will likely accept raw bytes
		_, _ = w.Write(data)
	})

	srv := &http.Server{Handler: handler}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	go func() {
		if logger != nil {
			logger.Printf("http server listening on %s serving %q", addr, filePath)
		}
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			if logger != nil {
				logger.Printf("http serve error: %v", err)
			}
		}
	}()
	return ln, nil
}
