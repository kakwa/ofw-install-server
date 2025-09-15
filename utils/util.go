package utils

import (
	"log"
	"net"
	"strconv"
	"strings"
)

func IndexOf(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}

func MustPort(addr string) int {
	_, p, err := net.SplitHostPort(addr)
	if err != nil {
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
