package main

import (
	"encoding/binary"
	"errors"
	"log"
	"net"
)

// Minimal rpcbind/portmap v2 over UDP implementing only GETPORT (proc 3)
// RFC 1833 / RFC 1057 XDR subset for our needs.

const (
	rpcVersion2    = 2
	programPortmap = 100000
	programMountd  = 100005
	programNFS     = 100003
	programNLMP    = 100021 // nlockmgr

	portmapVersion2      = 2
	procPMAPPROC_NULL    = 0
	procPMAPPROC_GETPORT = 3

	// RPC message types
	rpcCall  = 0
	rpcReply = 1

	// Auth flavors
	authNone = 0
)

// StartPortmapServer starts a very small rpcbind v2 server answering GETPORT for
// MOUNT and NFS programs using provided static mappings.
// It listens on UDP :111 by default, unless addr specifies another port.
func StartPortmapServer(addr string, mountdPort, nfsPort, nlockmgrPort uint32, logger *log.Logger) (net.PacketConn, error) {
	if addr == "" {
		addr = ":111"
	}
	pc, err := net.ListenPacket("udp4", addr)
	if err != nil {
		return nil, err
	}
	go func() {
		if logger != nil {
			logger.Printf("portmap listening on %s (mountd=%d nfs=%d nlockmgr=%d)", addr, mountdPort, nfsPort, nlockmgrPort)
		}
		buf := make([]byte, 2048)
		for {
			n, raddr, err := pc.ReadFrom(buf)
			if err != nil {
				if logger != nil {
					logger.Printf("portmap read error: %v", err)
				}
				return
			}
			resp, prog, vers, proc, err := handlePortmapUDP(buf[:n], mountdPort, nfsPort, nlockmgrPort)
			if logger != nil && err == nil {
				logger.Printf("portmap call from %s prog=%d vers=%d proc=%d", raddr.String(), prog, vers, proc)
			}
			if err != nil {
				// ignore malformed
				continue
			}
			_, _ = pc.WriteTo(resp, raddr)
		}
	}()
	return pc, nil
}

func handlePortmapUDP(req []byte, mountdPort, nfsPort, nlockmgrPort uint32) ([]byte, uint32, uint32, uint32, error) {
	// Minimal RPC header parse
	// struct rpc_msg {
	//   unsigned int xid;
	//   enum msg_type { CALL=0, REPLY=1 } mtype;
	//   union {
	//     call_body cbody; (version, program, version, proc, cred, verf)
	//     reply_body rbody;
	//   } body;
	// }
	if len(req) < 24 { // xid(4) mtype(4) rpcvers(4) prog(4) vers(4) proc(4)
		return nil, 0, 0, 0, errors.New("short rpc")
	}
	xid := binary.BigEndian.Uint32(req[0:4])
	mtype := binary.BigEndian.Uint32(req[4:8])
	if mtype != rpcCall {
		return nil, 0, 0, 0, errors.New("not call")
	}
	rpcvers := binary.BigEndian.Uint32(req[8:12])
	if rpcvers != rpcVersion2 {
		return nil, 0, 0, 0, errors.New("rpc version")
	}
	prog := binary.BigEndian.Uint32(req[12:16])
	vers := binary.BigEndian.Uint32(req[16:20])
	proc := binary.BigEndian.Uint32(req[20:24])

	// Skip creds/verifier (opaque_auth): flavor(4) length(4) bytes... twice
	off := 24
	// cred
	if len(req) < off+8 {
		return nil, 0, 0, 0, errors.New("short cred")
	}
	// flavor := binary.BigEndian.Uint32(req[off:off+4])
	llen := int(binary.BigEndian.Uint32(req[off+4 : off+8]))
	off += 8
	// payload padded to 4
	off += ((llen + 3) &^ 3)
	if len(req) < off+8 {
		return nil, 0, 0, 0, errors.New("short verf")
	}
	vlen := int(binary.BigEndian.Uint32(req[off+4 : off+8]))
	off += 8
	off += ((vlen + 3) &^ 3)

	if prog != programPortmap || vers != portmapVersion2 {
		return rpcErrorReply(xid), prog, vers, proc, nil
	}

	switch proc {
	case procPMAPPROC_NULL:
		return rpcNullReply(xid), prog, vers, proc, nil
	case procPMAPPROC_GETPORT:
		// GETPORT args: program(4) version(4) protocol(4) port(4)
		if len(req) < off+16 {
			return nil, 0, 0, 0, errors.New("short getport args")
		}
		pprog := binary.BigEndian.Uint32(req[off : off+4])
		pvers := binary.BigEndian.Uint32(req[off+4 : off+8])
		pproto := binary.BigEndian.Uint32(req[off+8 : off+12])
		_ = pvers
		_ = pproto // 17=udp 6=tcp
		var port uint32
		switch pprog {
		case programMountd:
			port = mountdPort
		case programNFS:
			port = nfsPort
		case programNLMP:
			port = nlockmgrPort
		default:
			port = 0
		}
		return rpcUintReply(xid, port), prog, vers, proc, nil
	default:
		return rpcErrorReply(xid), prog, vers, proc, nil
	}
}

func rpcHeaderReply(xid uint32) []byte {
	// xid, mtype=REPLY, reply_stat=MSG_ACCEPTED(0), verf (AUTH_NONE,0), accept_stat=SUCCESS(0)
	resp := make([]byte, 0, 32)
	hdr := make([]byte, 24)
	binary.BigEndian.PutUint32(hdr[0:4], xid)
	binary.BigEndian.PutUint32(hdr[4:8], rpcReply)
	// reply_stat = MSG_ACCEPTED(0)
	binary.BigEndian.PutUint32(hdr[8:12], 0)
	// verifier: AUTH_NONE(0), length 0
	binary.BigEndian.PutUint32(hdr[12:16], authNone)
	binary.BigEndian.PutUint32(hdr[16:20], 0)
	// accept_stat = SUCCESS(0)
	binary.BigEndian.PutUint32(hdr[20:24], 0)
	resp = append(resp, hdr...)
	return resp
}

func rpcNullReply(xid uint32) []byte {
	return rpcHeaderReply(xid)
}

func rpcUintReply(xid, val uint32) []byte {
	resp := rpcHeaderReply(xid)
	tmp := make([]byte, 4)
	binary.BigEndian.PutUint32(tmp, val)
	resp = append(resp, tmp...)
	return resp
}

func rpcErrorReply(xid uint32) []byte {
	// xid, REPLY, MSG_DENIED(1), AUTH_ERROR(1) as a simple generic error
	buf := make([]byte, 20)
	binary.BigEndian.PutUint32(buf[0:4], xid)
	binary.BigEndian.PutUint32(buf[4:8], rpcReply)
	binary.BigEndian.PutUint32(buf[8:12], 1)  // MSG_DENIED
	binary.BigEndian.PutUint32(buf[12:16], 1) // AUTH_ERROR
	binary.BigEndian.PutUint32(buf[16:20], 0)
	return buf
}
