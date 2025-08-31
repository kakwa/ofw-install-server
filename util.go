package main

import (
	"log"
	"net"
	"strconv"
	"strings"
)

func indexOf(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}

func mustPort(addr string) int {
	// addr can be ":2049" or "0.0.0.0:2049"
	_, p, err := net.SplitHostPort(addr)
	if err != nil {
		// try when only ":NNN" present
		if strings.HasPrefix(addr, ":") {
			v, _ := strconv.Atoi(addr[1:])
			return v
		}
		log.Fatalf("invalid addr %q: %v", addr, err)
	}
	v, err := strconv.Atoi(p)
	if err != nil {
		log.Fatalf("invalid port in %q: %v", addr, err)
	}
	return v
}

