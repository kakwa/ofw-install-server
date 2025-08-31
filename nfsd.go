package main

import (
	"log"
	"net"
	"os"
	"path/filepath"
	"time"
)

// NFS program and version (v2 minimal)
const (
	nfsProgram = 100003
	nfsV2      = 2
)

// NFS v2 procedures (subset)
const (
	nfsProcNull    = 0
	nfsProcGetAttr = 1
	nfsProcLookup  = 4
	nfsProcRead    = 6
)

// StartNFSD runs a tiny NFS v2 UDP server rooted at baseDir.
func StartNFSD(addr string, baseDir string, logger *log.Logger) (net.PacketConn, error) {
	if addr == "" {
		addr = ":2049"
	}
	pc, err := net.ListenPacket("udp4", addr)
	if err != nil {
		return nil, err
	}
	go func() {
		if logger != nil {
			logger.Printf("nfsd v2 listening on %s base=%q", addr, baseDir)
		}
		buf := make([]byte, 8192)
		for {
			n, raddr, err := pc.ReadFrom(buf)
			if err != nil {
				if logger != nil {
					logger.Printf("nfsd read error: %v", err)
				}
				return
			}
			resp := handleNFSD(buf[:n], baseDir, logger)
			if resp != nil {
				_, _ = pc.WriteTo(resp, raddr)
			}
		}
	}()
	return pc, nil
}

func handleNFSD(pkt []byte, baseDir string, logger *log.Logger) []byte {
	xid, prog, vers, proc, rr, err := parseRPCCall(pkt)
	if err != nil {
		return nil
	}
	if prog != nfsProgram || vers != nfsV2 {
		return rpcReplyDeniedAuth(xid)
	}
	switch proc {
	case nfsProcNull:
		logger.Printf("nfsProcNull")
		return rpcReplyHeaderAccepted(xid)
	case nfsProcGetAttr:
		logger.Printf("nfsProcGetAttr")
		// args: fhandle (opaque up to 32)
		fh, err := rr.readOpaque()
		if err != nil {
			return rpcReplyDeniedAuth(xid)
		}
		_ = fh // we do not map fhandles; always return root attributes
		if logger != nil {
			logger.Printf("nfsd GETATTR for root -> %q", baseDir)
		}
		return nfsReplyAttrOK(xid, baseDir)
	case nfsProcLookup:
		logger.Printf("nfsProcLookup")
		// args: diropargs: dir(fh), name(string)
		_, err := rr.readOpaque() // dir fh
		if err != nil {
			return rpcReplyDeniedAuth(xid)
		}
		//name, err := rr.readOpaque()
		//if err != nil {
		//	logger.Printf("nfsd LOOKUP failed to read name")
		//	return rpcReplyDeniedAuth(xid)
		//}
		name := "bsd"
		logger.Printf("nfsd LOOKUP attempt: %q", string(name))

		target := filepath.Join(baseDir, filepath.Clean(string(name)))
		logger.Printf("nfsd LOOKUP name=%q -> %q", string(name), target)
		if !withinRoot(baseDir, target) {
			if logger != nil {
				logger.Printf("nfsd LOOKUP denied (outside root): %q", target)
			}
			return nfsReplyErrNoEnt(xid)
		}
		if _, err := os.Stat(target); err != nil {
			if logger != nil {
				logger.Printf("nfsd LOOKUP noent: %q", target)
			}
			return nfsReplyErrNoEnt(xid)
		}
		// Return a fake filehandle and attributes
		w := &xdrWriter{}
		w.b = append(w.b, rpcReplyHeaderAccepted(xid)...)
		w.writeUint32(0)                // status OK
		w.writeOpaque(make([]byte, 32)) // object fh
		// post_op_attr: attributes_follow = TRUE(1)
		w.writeUint32(1)
		writeNFSV2Fattr(w, target)
		if logger != nil {
			logger.Printf("nfsd LOOKUP ok: %q", target)
		}
		return w.b
	case nfsProcRead:
		// args: fh(opaque), offset(uint32), count(uint32), totalcount(uint32)
		fh, err := rr.readOpaque()
		if err != nil {
			return rpcReplyDeniedAuth(xid)
		}
		offset, err := rr.readUint32()
		if err != nil {
			return rpcReplyDeniedAuth(xid)
		}
		count, err := rr.readUint32()
		if err != nil {
			return rpcReplyDeniedAuth(xid)
		}
		// totalcount ignored
		_, _ = rr.readUint32()
		p, ok := pathForHandle(fh)
		if !ok {
			return nfsReplyErrNoEnt(xid)
		}
		if !withinRoot(baseDir, p) {
			return nfsReplyErrNoEnt(xid)
		}
		f, err := os.Open(p)
		if err != nil {
			return nfsReplyErrNoEnt(xid)
		}
		defer f.Close()
		buf := make([]byte, count)
		n, _ := f.ReadAt(buf, int64(offset))
		buf = buf[:n]
		if logger != nil {
			logger.Printf("nfsd READ %q off=%d count=%d -> %d bytes", p, offset, count, n)
		}
		w := &xdrWriter{}
		w.b = append(w.b, rpcReplyHeaderAccepted(xid)...)
		w.writeUint32(0) // status OK
		writeNFSV2Fattr(w, p)
		w.writeOpaque(buf)
		return w.b
	default:
		logger.Printf("proc not supported: %d", proc)
		return rpcReplyDeniedAuth(xid)
	}
}

func nfsReplyAttrOK(xid uint32, path string) []byte {
	w := &xdrWriter{}
	w.b = append(w.b, rpcReplyHeaderAccepted(xid)...)
	w.writeUint32(0) // status OK
	writeNFSV2Fattr(w, path)
	return w.b
}

func nfsReplyErrNoEnt(xid uint32) []byte {
	w := &xdrWriter{}
	w.b = append(w.b, rpcReplyHeaderAccepted(xid)...)
	w.writeUint32(2) // NFSERR_NOENT
	return w.b
}

// NFSv2 fattr (RFC 1094):
// ftype(4) mode(4) nlink(4) uid(4) gid(4) size(4) blocksize(4) rdev(4)
// blocks(4) fsid(4) fileid(4) atime(3*4) mtime(3*4) ctime(3*4)
func writeNFSV2Fattr(w *xdrWriter, path string) {
	fi, err := os.Stat(path)
	var mode uint32 = 040755
	var ftype uint32 = 2 // directory
	var nlink uint32 = 1
	var size uint32 = 0
	if err == nil {
		if fi.IsDir() {
			ftype = 2
			mode = 040755
		} else {
			ftype = 1
			mode = 0100644
			if fi.Size() > 0 {
				if fi.Size() > 0xFFFFFFFF {
					size = 0xFFFFFFFF
				} else {
					size = uint32(fi.Size())
				}
			}
		}
		nlink = uint32(1)
	}
	// naive values for the rest
	w.writeUint32(ftype)
	w.writeUint32(mode)
	w.writeUint32(nlink)
	w.writeUint32(0) // uid
	w.writeUint32(0) // gid
	w.writeUint32(size)
	w.writeUint32(4096) // blocksize
	w.writeUint32(0)    // rdev
	w.writeUint32(0)    // blocks
	w.writeUint32(1)    // fsid
	w.writeUint32(1)    // fileid
	now := time.Now()
	writeNFSv2Time(w, now)
	writeNFSv2Time(w, now)
	writeNFSv2Time(w, now)
}

func writeNFSv2Time(w *xdrWriter, t time.Time) {
	w.writeUint32(uint32(t.Unix()))
	w.writeUint32(0) // usec
}
