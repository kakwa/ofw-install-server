package main

import "testing"

func TestHandlePathRoundTrip(t *testing.T) {
	p := "/tmp/some/path"
	fh := handleForPath(p)
	got, ok := pathForHandle(fh)
	if !ok || got != p {
		t.Fatalf("round-trip failed: ok=%v got=%q", ok, got)
	}
}
