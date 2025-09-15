package utils

import (
	"errors"
	"fmt"
	"net"
)

func IfaceByName(name string) (*net.Interface, error) {
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

func FirstIPv4Addr(name string) (net.IP, error) {
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

func CIDRFromInterface(ifc *net.Interface, serverIP net.IP) (string, error) {
	addrs, err := ifc.Addrs()
	if err != nil {
		return "", err
	}
	var ipnet *net.IPNet
	for _, a := range addrs {
		if v, ok := a.(*net.IPNet); ok {
			if v.IP.To4() != nil && v.IP.Equal(serverIP) {
				ipnet = v
				break
			}
		}
	}
	if ipnet == nil {
		for _, a := range addrs {
			if v, ok := a.(*net.IPNet); ok {
				if v.IP.To4() != nil {
					ipnet = v
					break
				}
			}
		}
	}
	if ipnet == nil {
		return "", fmt.Errorf("no IPv4 network on interface %s", ifc.Name)
	}
	ones, _ := ipnet.Mask.Size()
	return fmt.Sprintf("%s/%d", serverIP.String(), ones), nil
}
