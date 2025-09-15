package httpx

import (
	"io"
	"log"
	"net"
	"net/http"
	"os"
)

func StartHTTPServer(addr string, filePath string, logger *log.Logger) (net.Listener, error) {
	if addr == "" {
		addr = ":80"
	}
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(f)
	f.Close()
	if err != nil {
		return nil, err
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
