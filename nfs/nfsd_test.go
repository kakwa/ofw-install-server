package nfs

import (
	"os"
	"testing"
)

func TestWriteNFSv2Fattr(t *testing.T) {
	f, err := os.CreateTemp("", "nfsattr-*.tmp")
	if err != nil {
		t.Fatalf("temp file: %v", err)
	}
	defer os.Remove(f.Name())
	defer f.Close()

	w := &xdrWriter{}
	writeNFSV2Fattr(w, f.Name())
	if len(w.b) == 0 {
		t.Fatalf("expected some bytes written")
	}
}
