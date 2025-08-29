package main

import (
	"log"
	"net"

	"github.com/go-git/go-billy/v5/osfs"
	nfs "github.com/willscott/go-nfs"
	nfshelper "github.com/willscott/go-nfs/helpers"
)

type NFSConfig struct {
	Addr string // e.g. ":2049"
	Root string // filesystem path to export
}

func StartNFSServer(cfg NFSConfig, logger *log.Logger) (net.Listener, error) {
	if cfg.Addr == "" {
		cfg.Addr = ":2049"
	}
	fs := osfs.New(cfg.Root)
	h := nfshelper.NewNullAuthHandler(fs)
	ch := nfshelper.NewCachingHandler(h, 1)
	l, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return nil, err
	}
	go func() {
		if logger != nil {
			logger.Printf("NFS exporting %s on %s", cfg.Root, cfg.Addr)
		}
		_ = nfs.Serve(l, ch)
	}()
	return l, nil
}
