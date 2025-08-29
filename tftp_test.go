package main

import (
	"net"
	"testing"
)

func TestIPToHexString(t *testing.T) {
	cases := []struct {
		ip   net.IP
		want string
	}{
		{net.IPv4(0, 0, 0, 0), "00000000"},
		{net.IPv4(192, 168, 1, 10), "C0A8010A"},
		{net.IPv4(10, 0, 0, 2), "0A000002"},
	}
	for _, tc := range cases {
		if got := ipToHexString(tc.ip); got != tc.want {
			t.Fatalf("ipToHexString(%v) = %q, want %q", tc.ip, got, tc.want)
		}
	}
}

func TestIsHexIPv4Name(t *testing.T) {
	if !isHexIPv4Name("C0A8010A") {
		t.Fatalf("expected true for valid hex name")
	}
	if isHexIPv4Name("C0A8010") || isHexIPv4Name("C0A8010AZ") || isHexIPv4Name("..") {
		t.Fatalf("expected false for invalid names")
	}
}
