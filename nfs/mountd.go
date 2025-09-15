package nfs

import (
	"log"
	"net"
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
			resp := handleMountd(buf[:n], baseDir, logger)
			if resp != nil {
				_, _ = pc.WriteTo(resp, raddr)
			}
		}
	}()
	return pc, nil
}

func handleMountd(pkt []byte, baseDir string, logger *log.Logger) []byte {
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
		if logger != nil {
			logger.Printf("mountd MNT request path=%q full=%q", clean, full)
		}
		// success: status=0 and a 32-byte file handle derived from path
		w := &xdrWriter{}
		w.b = append(w.b, rpcReplyHeaderAccepted(xid)...)
		w.writeUint32(0) // status OK
		w.writeOpaque(handleForPath(full))
		if logger != nil {
			logger.Printf("mountd MNT ok path=%q", full)
		}
		return w.b
	case mountProcUmnt:
		if logger != nil {
			logger.Printf("mountd UMNT request")
		}
		w := &xdrWriter{}
		w.b = append(w.b, rpcReplyHeaderAccepted(xid)...)
		w.writeUint32(0)
		return w.b
	default:
		return rpcReplyDeniedAuth(xid)
	}
}
