package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"log"
	"net"
	"os"
	"strings"
)

// Minimal Bootparam (RFC 951/953) server over UDP without rpcbind.
// Implements WHOAMI (proc=1) and GETFILE (proc=2) for program 100026 v1.

const (
	bootparamProg = 100026
	bootparamVers = 1

	procWhoami  = 1
	procGetfile = 2

	rpcMsgTypeCall  = 0
	rpcMsgTypeReply = 1

	// Reply status
	msgAccepted = 0
	msgDenied   = 1

	authNull = 0

	// Accept status
	accSuccess = 0
)

type BootparamConfig struct {
	Addr       string // UDP address to listen on, e.g. ":10026"
	RootPath   string // Path to serve for GETFILE("root")
	SwapPath   string // Optional swap path for GETFILE("swap")
	ServerName string // Optional override for server name in replies
	RootFS     string // If non-empty, GETFILE("root") returns this TFTP path/file; else RootPath
}

func StartBootparamUDP(cfg BootparamConfig, serverIP net.IP, logger *log.Logger) (net.PacketConn, error) {
	if cfg.Addr == "" {
		cfg.Addr = ":10026"
	}
	pc, err := net.ListenPacket("udp", cfg.Addr)
	if err != nil {
		return nil, err
	}
	if logger != nil {
		logger.Printf("bootparam UDP listening on %s (root=%q swap=%q rootfs=%q)", cfg.Addr, cfg.RootPath, cfg.SwapPath, cfg.RootFS)
	}
	go serveBootparam(pc, cfg, serverIP, logger)
	return pc, nil
}

func serveBootparam(pc net.PacketConn, cfg BootparamConfig, serverIP net.IP, logger *log.Logger) {
	bl := make([]byte, 2048)
	for {
		n, addr, err := pc.ReadFrom(bl)
		if err != nil {
			if logger != nil {
				logger.Printf("bootparam read error: %v", err)
			}
			continue
		}
		if logger != nil {
			logger.Printf("bootparam recv %dB from %s", n, addr.String())
		}
		b := make([]byte, n)
		copy(b, bl[:n])
		resp, err := handleBootparamCall(b, cfg, serverIP, addr, logger)
		if err != nil {
			if logger != nil {
				logger.Printf("bootparam handle error: %v", err)
			}
			continue
		}
		if resp != nil {
			_, _ = pc.WriteTo(resp, addr)
			if logger != nil {
				logger.Printf("bootparam sent %dB to %s", len(resp), addr.String())
			}
		}
	}
}

func handleBootparamCall(b []byte, cfg BootparamConfig, serverIP net.IP, addr net.Addr, logger *log.Logger) ([]byte, error) {
	r := bytes.NewReader(b)
	// RPC header
	xid, err := readU32(r)
	if err != nil {
		return nil, err
	}
	mtype, err := readU32(r)
	if err != nil || mtype != rpcMsgTypeCall {
		return nil, errors.New("not RPC call")
	}
	rpcvers, err := readU32(r)
	if err != nil || rpcvers != 2 {
		return nil, errors.New("bad rpcvers")
	}
	prog, _ := readU32(r)
	vers, _ := readU32(r)
	proc, _ := readU32(r)
	if logger != nil {
		logger.Printf("bootparam RPC call xid=%d prog=%d vers=%d proc=%d from %s", xid, prog, vers, proc, addr.String())
	}
	if prog != bootparamProg || vers != bootparamVers {
		if logger != nil {
			logger.Printf("bootparam program/version mismatch: got prog=%d vers=%d", prog, vers)
		}
		return buildRPCMismatchReply(xid), nil
	}
	// Credentials and verifier (skip)
	if err := skipOpaqueAuth(r); err != nil {
		return nil, err
	}
	if err := skipOpaqueAuth(r); err != nil {
		return nil, err
	}

	switch proc {
	case procWhoami:
		return buildWhoamiReply(xid, cfg, serverIP, addr, r, logger)
	case procGetfile:
		return buildGetfileReply(xid, cfg, serverIP, r, logger)
	default:
		if logger != nil {
			logger.Printf("bootparam unknown proc=%d", proc)
		}
		return buildRPCAcceptEmpty(xid), nil
	}
}

func buildWhoamiReply(xid uint32, cfg BootparamConfig, serverIP net.IP, addr net.Addr, r *bytes.Reader, logger *log.Logger) ([]byte, error) {
	// Try to read ipaddr from args if present (4 bytes). If not, use src addr.
	clientIP := extractRemoteIPv4(addr)
	if r.Len() >= 4 {
		var ipBytes [4]byte
		if _, err := r.Read(ipBytes[:]); err == nil {
			clientIP = net.IP(ipBytes[:])
		}
	}
	serverName := cfg.ServerName
	if serverName == "" {
		if h, err := os.Hostname(); err == nil {
			serverName = h
		} else {
			serverName = "server"
		}
	}
	domain := os.Getenv("DOMAINNAME")
	// Response body: client_name, domain_name, router_address
	body := &bytes.Buffer{}
	writeString(body, ipToDotted(clientIP))
	writeString(body, domain)
	body.Write(serverIP.To4())
	if logger != nil {
		logger.Printf("bootparam WHOAMI client=%s domain=%q router=%s", ipToDotted(clientIP), domain, serverIP.String())
	}
	return buildRPCAcceptReply(xid, body.Bytes()), nil
}

func buildGetfileReply(xid uint32, cfg BootparamConfig, serverIP net.IP, r *bytes.Reader, logger *log.Logger) ([]byte, error) {
	clientName, err := readString(r)
	if err != nil {
		clientName = ""
	}
	_ = clientName // reserved for future policy
	fileID, err := readString(r)
	if err != nil {
		fileID = "root"
	}
	fileID = strings.ToLower(fileID)
	serverName := cfg.ServerName
	if serverName == "" {
		if h, err := os.Hostname(); err == nil {
			serverName = h
		} else {
			serverName = "server"
		}
	}
	var path string
	switch fileID {
	case "root":
		// For root, prefer explicit RootFS (TFTP-served file), else NFS RootPath
		if cfg.RootFS != "" {
			path = cfg.RootFS
		} else {
			path = cfg.RootPath
		}
	case "swap":
		path = cfg.SwapPath
	default:
		path = cfg.RootPath
	}
	body := &bytes.Buffer{}
	writeString(body, serverName)
	body.Write(serverIP.To4())
	writeString(body, path)
	if logger != nil {
		logger.Printf("bootparam GETFILE client=%q id=%q -> server=%q ip=%s path=%q", clientName, fileID, serverName, serverIP.String(), path)
	}
	return buildRPCAcceptReply(xid, body.Bytes()), nil
}

func buildRPCMismatchReply(xid uint32) []byte {
	// Reply with MSG_ACCEPTED + SUCCESS and empty body to be benign.
	return buildRPCAcceptReply(xid, nil)
}

func buildRPCAcceptEmpty(xid uint32) []byte { return buildRPCAcceptReply(xid, nil) }

func buildRPCAcceptReply(xid uint32, payload []byte) []byte {
	buf := &bytes.Buffer{}
	writeU32(buf, xid)
	writeU32(buf, rpcMsgTypeReply)
	writeU32(buf, msgAccepted)
	// Verifier: AUTH_NULL
	writeU32(buf, authNull)
	writeU32(buf, 0)
	// accept stat
	writeU32(buf, accSuccess)
	if len(payload) > 0 {
		buf.Write(payload)
	}
	return buf.Bytes()
}

func skipOpaqueAuth(r *bytes.Reader) error {
	// flavor, length, body (rounded up to 4)
	if _, err := readU32(r); err != nil {
		return err
	}
	ln, err := readU32(r)
	if err != nil {
		return err
	}
	if int(ln) > r.Len() {
		return errors.New("short opaque auth")
	}
	// Skip body + padding
	pad := (4 - (ln % 4)) % 4
	if _, err := r.Seek(int64(ln+pad), 1); err != nil {
		return err
	}
	return nil
}

func readU32(r *bytes.Reader) (uint32, error) {
	var v uint32
	if err := binary.Read(r, binary.BigEndian, &v); err != nil {
		return 0, err
	}
	return v, nil
}

func readString(r *bytes.Reader) (string, error) {
	ln, err := readU32(r)
	if err != nil {
		return "", err
	}
	if int(ln) > r.Len() {
		return "", errors.New("short string")
	}
	b := make([]byte, ln)
	if _, err := r.Read(b); err != nil {
		return "", err
	}
	// padding
	pad := (4 - (ln % 4)) % 4
	if pad > 0 {
		if _, err := r.Seek(int64(pad), 1); err != nil {
			return "", err
		}
	}
	return string(b), nil
}

func writeU32(w *bytes.Buffer, v uint32) { _ = binary.Write(w, binary.BigEndian, v) }

func writeString(w *bytes.Buffer, s string) {
	writeU32(w, uint32(len(s)))
	w.WriteString(s)
	// padding
	pad := (4 - (len(s) % 4)) % 4
	if pad > 0 {
		w.Write(make([]byte, pad))
	}
}

func extractRemoteIPv4(addr net.Addr) net.IP {
	udp, ok := addr.(*net.UDPAddr)
	if ok && udp.IP != nil {
		return udp.IP.To4()
	}
	return net.IPv4(0, 0, 0, 0)
}

func ipToDotted(ip net.IP) string {
	v4 := ip.To4()
	if v4 == nil {
		return "0.0.0.0"
	}
	return net.IP(v4).String()
}
