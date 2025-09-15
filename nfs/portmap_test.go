package nfs

import (
	"encoding/binary"
	"testing"
)

func TestHandlePortmapUDP_GetPort(t *testing.T) {
	xid := uint32(0x12345678)
	b := buildMinimalRPCCall(xid, programPortmap, portmapVersion2, procPMAPPROC_GETPORT)
	args := make([]byte, 16)
	binary.BigEndian.PutUint32(args[0:4], programNFS)
	binary.BigEndian.PutUint32(args[4:8], 2)
	binary.BigEndian.PutUint32(args[8:12], 17)
	binary.BigEndian.PutUint32(args[12:16], 0)
	b = append(b, args...)
	resp, prog, vers, proc, err := handlePortmapUDP(b, 20048, 2049, 0)
	if err != nil {
		t.Fatalf("handlePortmapUDP error: %v", err)
	}
	if prog != programPortmap || vers != portmapVersion2 || proc != procPMAPPROC_GETPORT {
		t.Fatalf("unexpected parse summary")
	}
	if len(resp) < 28 {
		t.Fatalf("short reply")
	}
	got := binary.BigEndian.Uint32(resp[len(resp)-4:])
	if got != 2049 {
		t.Fatalf("expected port 2049, got %d", got)
	}
}
