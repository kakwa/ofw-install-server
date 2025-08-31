package main

import (
	"testing"
)

func TestIsHexIPv4Name(t *testing.T) {
	if !isHexIPv4Name("C0A8010A") {
		t.Fatalf("expected true for valid hex name")
	}
	if isHexIPv4Name("C0A8010") || isHexIPv4Name("C0A8010AZ") || isHexIPv4Name("..") {
		t.Fatalf("expected false for invalid names")
	}
}
