package utils

import "testing"

func TestIndexOf(t *testing.T) {
	if got := IndexOf("hello", 'e'); got != 1 {
		t.Fatalf("IndexOf got=%d want=1", got)
	}
	if got := IndexOf("hello", 'x'); got != -1 {
		t.Fatalf("IndexOf not found got=%d want=-1", got)
	}
}

func TestMustPort(t *testing.T) {
	if got := MustPort(":2049"); got != 2049 {
		t.Fatalf("MustPort got=%d want=2049", got)
	}
	if got := MustPort("0.0.0.0:111"); got != 111 {
		t.Fatalf("MustPort got=%d want=111", got)
	}
}
