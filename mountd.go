package main

import (
	"log"
	"net"
	"os"
	"path/filepath"
)

// Program and version numbers
const (
	mountProgram = 100005
	mountV1      = 1
)

// MOUNT v1 procedures
const (
	mountProcNull = 0
	mountProcMnt  = 1
	mountProcUmnt = 3
)

// StartMountd runs a tiny MOUNT v1 UDP server that accepts any export under baseDir.
func StartMountd(addr string, baseDir string, logger *log.Logger) (net.PacketConn, error) {
	if addr == "" {
		addr = ":20048"
	}
	pc, err := net.ListenPacket("udp4", addr)
	if err != nil {
		return nil, err
	}
	go func() {
		if logger != nil {
			logger.Printf("mountd v1 listening on %s base=%q", addr, baseDir)
		}
		buf := make([]byte, 8192)
		for {
			n, raddr, err := pc.ReadFrom(buf)
			if err != nil {
				if logger != nil {
					logger.Printf("mountd read error: %v", err)
				}
				return
			}
			resp := handleMountd(buf[:n], baseDir)
			if resp != nil {
				_, _ = pc.WriteTo(resp, raddr)
			}
		}
	}()
	return pc, nil
}

func handleMountd(pkt []byte, baseDir string) []byte {
	xid, prog, vers, proc, rr, err := parseRPCCall(pkt)
	if err != nil {
		return nil
	}
	if prog != mountProgram || vers != mountV1 {
		return rpcReplyDeniedAuth(xid)
	}
	switch proc {
	case mountProcNull:
		return rpcReplyHeaderAccepted(xid)
	case mountProcMnt:
		// args: dirpath (string)
		path, err := rr.readOpaque()
		if err != nil {
			return rpcReplyDeniedAuth(xid)
		}
		// Clean path and ensure inside baseDir
		clean := filepath.Clean(string(path))
		full := filepath.Join(baseDir, clean)
		if !withinRoot(baseDir, full) {
			// return status error (NFSERR_PERM = 1). For v1, fhandle length 32 opaque on success.
			w := &xdrWriter{}
			w.b = append(w.b, rpcReplyHeaderAccepted(xid)...)
			w.writeUint32(1) // status error
			return w.b
		}
		// ensure exists
		if s, err := os.Stat(full); err != nil || !s.IsDir() {
			w := &xdrWriter{}
			w.b = append(w.b, rpcReplyHeaderAccepted(xid)...)
			w.writeUint32(2) // NFSERR_NOENT simplistic
			return w.b
		}
		// success: status=0 and a 32-byte fake file handle (all zeros)
		w := &xdrWriter{}
		w.b = append(w.b, rpcReplyHeaderAccepted(xid)...)
		w.writeUint32(0) // status OK
		w.writeOpaque(make([]byte, 32))
		return w.b
	case mountProcUmnt:
		w := &xdrWriter{}
		w.b = append(w.b, rpcReplyHeaderAccepted(xid)...)
		w.writeUint32(0)
		return w.b
	default:
		return rpcReplyDeniedAuth(xid)
	}
}
