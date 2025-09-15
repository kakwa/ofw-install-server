package nfs

import (
	"encoding/binary"
	"testing"
)

func buildMinimalRPCCall(xid, prog, vers, proc uint32) []byte {
	b := make([]byte, 0, 40)
	hdr := make([]byte, 24)
	binary.BigEndian.PutUint32(hdr[0:4], xid)
	binary.BigEndian.PutUint32(hdr[4:8], 0)
	binary.BigEndian.PutUint32(hdr[8:12], 2)
	binary.BigEndian.PutUint32(hdr[12:16], prog)
	binary.BigEndian.PutUint32(hdr[16:20], vers)
	binary.BigEndian.PutUint32(hdr[20:24], proc)
	b = append(b, hdr...)
	b = append(b, make([]byte, 8)...)
	b = append(b, make([]byte, 8)...)
	return b
}

func TestParseRPCCallAndReplies(t *testing.T) {
	req := buildMinimalRPCCall(0xdeadbeef, 100000, 2, 0)
	xid, prog, vers, proc, _, err := parseRPCCall(req)
	if err != nil {
		t.Fatalf("parseRPCCall error: %v", err)
	}
	if xid != 0xdeadbeef || prog != 100000 || vers != 2 || proc != 0 {
		t.Fatalf("unexpected parsed header")
	}
	if len(rpcReplyHeaderAccepted(xid)) == 0 || len(rpcReplyDeniedAuth(xid)) == 0 {
		t.Fatalf("expected non-empty replies")
	}
}

func TestXDRReaderWriterOpaque(t *testing.T) {
	w := &xdrWriter{}
	data := []byte{1, 2, 3, 4, 5}
	w.writeOpaque(data)
	r := xdrReader{b: w.b}
	got, err := r.readOpaque()
	if err != nil {
		t.Fatalf("readOpaque error: %v", err)
	}
	if string(got) != string(data) {
		t.Fatalf("opaque mismatch")
	}
}
