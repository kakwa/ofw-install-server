package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

// RPCUDPProxy forwards ONC/RPC UDP packets between local clients and an upstream server.
// It tracks XID->clientAddr so replies are routed back.
type RPCUDPProxy struct {
	localConn    net.PacketConn
	upstreamConn *net.UDPConn
	logger       *log.Logger

	mu sync.Mutex
	// map xid -> client address and last-seen time
	xidToClient map[uint32]clientEntry
}

type clientEntry struct {
	addr     net.Addr
	lastSeen time.Time
}

// StartRPCUDPProxy listens on localAddr (e.g., ":20048") and forwards to upstreamHost:port.
// If upstreamPort is 0, it will query rpcbind on upstreamHost:111 for the given program/version (UDP protocol).
func StartRPCUDPProxy(localAddr string, upstreamHost string, upstreamPort uint32, program uint32, version uint32, logger *log.Logger) (*RPCUDPProxy, error) {
	if localAddr == "" {
		return nil, fmt.Errorf("localAddr required")
	}
	if upstreamHost == "" {
		return nil, fmt.Errorf("upstreamHost required")
	}
	// Resolve upstream port if needed
	if upstreamPort == 0 {
		p, err := rpcbindGetPortUDP(upstreamHost, program, version)
		if err != nil {
			return nil, fmt.Errorf("rpcbind getport: %w", err)
		}
		upstreamPort = p
	}
	lconn, err := net.ListenPacket("udp4", localAddr)
	if err != nil {
		return nil, err
	}
	raddr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%d", upstreamHost, upstreamPort))
	if err != nil {
		lconn.Close()
		return nil, err
	}
	uconn, err := net.DialUDP("udp4", nil, raddr)
	if err != nil {
		lconn.Close()
		return nil, err
	}
	p := &RPCUDPProxy{
		localConn:    lconn,
		upstreamConn: uconn,
		logger:       logger,
		xidToClient:  make(map[uint32]clientEntry),
	}
	go p.pumpLocalToUpstream()
	go p.pumpUpstreamToLocal()
	go p.gcLoop()
	if logger != nil {
		logger.Printf("rpc proxy listening on %s -> %s", localAddr, raddr.String())
	}
	return p, nil
}

func (p *RPCUDPProxy) pumpLocalToUpstream() {
	buf := make([]byte, 65535)
	for {
		n, addr, err := p.localConn.ReadFrom(buf)
		if err != nil {
			if p.logger != nil {
				p.logger.Printf("proxy local read error: %v", err)
			}
			return
		}
		if n >= 4 {
			xid := binary.BigEndian.Uint32(buf[0:4])
			p.mu.Lock()
			p.xidToClient[xid] = clientEntry{addr: addr, lastSeen: time.Now()}
			p.mu.Unlock()
		}
		_, err = p.upstreamConn.Write(buf[:n])
		if err != nil {
			if p.logger != nil {
				p.logger.Printf("proxy write upstream error: %v", err)
			}
			continue
		}
	}
}

func (p *RPCUDPProxy) pumpUpstreamToLocal() {
	buf := make([]byte, 65535)
	for {
		n, err := p.upstreamConn.Read(buf)
		if err != nil {
			if p.logger != nil {
				p.logger.Printf("proxy upstream read error: %v", err)
			}
			return
		}
		if n < 4 {
			continue
		}
		xid := binary.BigEndian.Uint32(buf[0:4])
		p.mu.Lock()
		ce, ok := p.xidToClient[xid]
		p.mu.Unlock()
		if !ok {
			continue
		}
		_, err = p.localConn.WriteTo(buf[:n], ce.addr)
		if err != nil {
			if p.logger != nil {
				p.logger.Printf("proxy write back error: %v", err)
			}
			continue
		}
	}
}

func (p *RPCUDPProxy) gcLoop() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		cutoff := time.Now().Add(-5 * time.Minute)
		p.mu.Lock()
		for xid, ce := range p.xidToClient {
			if ce.lastSeen.Before(cutoff) {
				delete(p.xidToClient, xid)
			}
		}
		p.mu.Unlock()
	}
}

// rpcbindGetPortUDP sends a PMAP GETPORT to upstreamHost and returns the port.
func rpcbindGetPortUDP(upstreamHost string, program uint32, version uint32) (uint32, error) {
	// Build minimal CALL to portmap v2 GETPORT over UDP
	// xid random-ish: use current time low bits
	xid := uint32(time.Now().UnixNano())
	// header 24 + cred 8 + verf 8 + args 16
	req := make([]byte, 24+8+8+16)
	off := 0
	binary.BigEndian.PutUint32(req[off:], xid)
	off += 4
	binary.BigEndian.PutUint32(req[off:], 0)
	off += 4 // CALL
	binary.BigEndian.PutUint32(req[off:], 2)
	off += 4 // rpcvers 2
	binary.BigEndian.PutUint32(req[off:], 100000)
	off += 4 // program portmap
	binary.BigEndian.PutUint32(req[off:], 2)
	off += 4 // version 2
	binary.BigEndian.PutUint32(req[off:], 3)
	off += 4 // proc GETPORT
	// cred: AUTH_NONE, length 0
	binary.BigEndian.PutUint32(req[off:], 0)
	off += 4
	binary.BigEndian.PutUint32(req[off:], 0)
	off += 4
	// verf: AUTH_NONE, length 0
	binary.BigEndian.PutUint32(req[off:], 0)
	off += 4
	binary.BigEndian.PutUint32(req[off:], 0)
	off += 4
	// args: program, version, protocol(udp=17), port(0)
	binary.BigEndian.PutUint32(req[off:], program)
	off += 4
	binary.BigEndian.PutUint32(req[off:], version)
	off += 4
	binary.BigEndian.PutUint32(req[off:], 17)
	off += 4
	binary.BigEndian.PutUint32(req[off:], 0)
	off += 4

	raddr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:111", upstreamHost))
	if err != nil {
		return 0, err
	}
	conn, err := net.DialUDP("udp4", nil, raddr)
	if err != nil {
		return 0, err
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Write(req); err != nil {
		return 0, err
	}
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		return 0, err
	}
	if n < 28 { // minimal reply header + uint32
		return 0, fmt.Errorf("short reply")
	}
	// last 4 bytes should be port per our encoder; be more robust by reading after verifier and accept_stat
	// parse fixed reply: xid, mtype, MSG_ACCEPTED, verf(auth, len=0), accept_stat=SUCCESS, port
	rxid := binary.BigEndian.Uint32(buf[0:4])
	if rxid != xid {
		// best-effort: ignore xid mismatch
	}
	// Extract port from last 4 bytes
	port := binary.BigEndian.Uint32(buf[n-4 : n])
	return port, nil
}
