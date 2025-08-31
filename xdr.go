package main

import (
	"encoding/binary"
	"errors"
	"io"
)

type xdrReader struct {
	b []byte
	o int
}

func (r *xdrReader) readUint32() (uint32, error) {
	if len(r.b) < r.o+4 {
		return 0, io.ErrUnexpectedEOF
	}
	v := binary.BigEndian.Uint32(r.b[r.o : r.o+4])
	r.o += 4
	return v, nil
}

func (r *xdrReader) readOpaque() ([]byte, error) {
	ln, err := r.readUint32()
	if err != nil {
		return nil, err
	}
	if ln > uint32(len(r.b)-r.o) {
		return nil, io.ErrUnexpectedEOF
	}
	data := r.b[r.o : r.o+int(ln)]
	r.o += int(ln)
	pad := (4 - (int(ln) & 3)) & 3
	if len(r.b) < r.o+pad {
		return nil, io.ErrUnexpectedEOF
	}
	r.o += pad
	return data, nil
}

func (r *xdrReader) skipOpaqueAuth() error {
	// flavor(4), length(4), bytes...
	if len(r.b) < r.o+8 {
		return io.ErrUnexpectedEOF
	}
	// skip flavor
	r.o += 4
	ln := int(binary.BigEndian.Uint32(r.b[r.o : r.o+4]))
	r.o += 4
	pad := (4 - (ln & 3)) & 3
	if len(r.b) < r.o+ln+pad {
		return io.ErrUnexpectedEOF
	}
	r.o += ln + pad
	return nil
}

type xdrWriter struct {
	b []byte
}

func (w *xdrWriter) writeUint32(v uint32) {
	tmp := make([]byte, 4)
	binary.BigEndian.PutUint32(tmp, v)
	w.b = append(w.b, tmp...)
}

func (w *xdrWriter) writeOpaque(p []byte) {
	w.writeUint32(uint32(len(p)))
	w.b = append(w.b, p...)
	pad := (4 - (len(p) & 3)) & 3
	if pad > 0 {
		w.b = append(w.b, make([]byte, pad)...)
	}
}

func parseRPCCall(b []byte) (xid, prog, vers, proc uint32, rr xdrReader, err error) {
	if len(b) < 24 {
		return 0, 0, 0, 0, xdrReader{}, errors.New("short rpc")
	}
	xid = binary.BigEndian.Uint32(b[0:4])
	mtype := binary.BigEndian.Uint32(b[4:8])
	if mtype != 0 {
		return 0, 0, 0, 0, xdrReader{}, errors.New("not call")
	}
	rpcvers := binary.BigEndian.Uint32(b[8:12])
	if rpcvers != 2 {
		return 0, 0, 0, 0, xdrReader{}, errors.New("rpc vers")
	}
	prog = binary.BigEndian.Uint32(b[12:16])
	vers = binary.BigEndian.Uint32(b[16:20])
	proc = binary.BigEndian.Uint32(b[20:24])
	rr = xdrReader{b: b, o: 24}
	// skip cred and verf
	if err = rr.skipOpaqueAuth(); err != nil {
		return
	}
	if err = rr.skipOpaqueAuth(); err != nil {
		return
	}
	return
}

func rpcReplyHeaderAccepted(xid uint32) []byte {
	w := &xdrWriter{}
	// xid
	w.writeUint32(xid)
	// REPLY
	w.writeUint32(1)
	// MSG_ACCEPTED
	w.writeUint32(0)
	// verf: AUTH_NONE, 0
	w.writeUint32(0)
	w.writeUint32(0)
	// accept_stat SUCCESS
	w.writeUint32(0)
	return w.b
}

func rpcReplyDeniedAuth(xid uint32) []byte {
	w := &xdrWriter{}
	w.writeUint32(xid)
	// REPLY
	w.writeUint32(1)
	// MSG_DENIED
	w.writeUint32(1)
	// AUTH_ERROR
	w.writeUint32(1)
	// auth_stat = AUTH_BADCRED (1)
	w.writeUint32(1)
	return w.b
}
