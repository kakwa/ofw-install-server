package utils

import (
	"net"
	"testing"
)

func TestNewIPv4AllocatorFromCIDRAndAllocate(t *testing.T) {
	alloc, err := NewIPv4AllocatorFromCIDR("192.168.10.0/24")
	if err != nil {
		t.Fatalf("NewIPv4AllocatorFromCIDR error: %v", err)
	}
	if got, want := alloc.RangeStart().String(), "192.168.10.1"; got != want {
		t.Fatalf("start=%s want=%s", got, want)
	}
	if got, want := alloc.RangeEnd().String(), "192.168.10.254"; got != want {
		t.Fatalf("end=%s want=%s", got, want)
	}

	alloc.ReserveIP(net.ParseIP("192.168.10.1"))

	var macA [6]byte = [6]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}
	var macB [6]byte = [6]byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}

	ipA, ok := alloc.AllocateForMAC(macA)
	if !ok {
		t.Fatalf("allocation failed for macA")
	}
	if got, want := net.IP(ipA[:]).String(), "192.168.10.2"; got != want {
		t.Fatalf("macA ip=%s want=%s", got, want)
	}
	ipA2, ok := alloc.AllocateForMAC(macA)
	if !ok || ipA2 != ipA {
		t.Fatalf("expected stable lease for macA")
	}
	ipB, ok := alloc.AllocateForMAC(macB)
	if !ok {
		t.Fatalf("allocation failed for macB")
	}
	if got, want := net.IP(ipB[:]).String(), "192.168.10.3"; got != want {
		t.Fatalf("macB ip=%s want=%s", got, want)
	}
}

func TestIPv4ArithmeticHelpers(t *testing.T) {
	ip := net.ParseIP("10.0.0.255").To4()
	incrementIPv4(ip)
	if got, want := ip.String(), "10.0.1.0"; got != want {
		t.Fatalf("incrementIPv4 got=%s want=%s", got, want)
	}
	decrementIPv4(ip)
	if got, want := ip.String(), "10.0.0.255"; got != want {
		t.Fatalf("decrementIPv4 got=%s want=%s", got, want)
	}
	if !ipv4LessOrEqual(net.ParseIP("10.0.0.1").To4(), net.ParseIP("10.0.0.2").To4()) {
		t.Fatalf("ipv4LessOrEqual expected true")
	}
	if ipv4LessOrEqual(net.ParseIP("10.0.0.5").To4(), net.ParseIP("10.0.0.4").To4()) {
		t.Fatalf("ipv4LessOrEqual expected false")
	}
	dup := cloneIPv4(net.ParseIP("1.2.3.4").To4())
	if got, want := dup.String(), "1.2.3.4"; got != want {
		t.Fatalf("cloneIPv4 got=%s want=%s", got, want)
	}
}
