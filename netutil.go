package main

import (
	"errors"
	"fmt"
	"net"
)

func ifaceByName(name string) (*net.Interface, error) {
	ifc, err := net.InterfaceByName(name)
	if err != nil {
		return nil, err
	}
	if (ifc.Flags & net.FlagUp) == 0 {
		return nil, fmt.Errorf("interface %s is down", name)
	}
	if ifc.HardwareAddr == nil || len(ifc.HardwareAddr) != 6 {
		return nil, fmt.Errorf("interface %s has no 6-byte MAC", name)
	}
	return ifc, nil
}

func firstIPv4Addr(name string) (net.IP, error) {
	ifc, err := net.InterfaceByName(name)
	if err != nil {
		return nil, err
	}
	addrs, err := ifc.Addrs()
	if err != nil {
		return nil, err
	}
	for _, a := range addrs {
		switch v := a.(type) {
		case *net.IPNet:
			ip := v.IP.To4()
			if ip != nil {
				return ip, nil
			}
		}
	}
	return nil, errors.New("no IPv4 on interface")
}
