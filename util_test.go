package main

import "testing"

func TestIndexOf(t *testing.T) {
	if got := indexOf("hello", 'e'); got != 1 {
		t.Fatalf("indexOf got=%d want=1", got)
	}
	if got := indexOf("hello", 'x'); got != -1 {
		t.Fatalf("indexOf not found got=%d want=-1", got)
	}
}

func TestMustPort(t *testing.T) {
	if got := mustPort(":2049"); got != 2049 {
		t.Fatalf("mustPort got=%d want=2049", got)
	}
	if got := mustPort("0.0.0.0:111"); got != 111 {
		t.Fatalf("mustPort got=%d want=111", got)
	}
}
