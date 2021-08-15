package host

import (
	"fmt"
	"net"
	"os"
	"strings"
)

var (
	_ip, _ = GetHostIP()
)

func MyIP() string {
	return _ip
}

func GetHostname() (string, error) {
	return os.Hostname()
}

// below inspired by https://github.com/alibaba/ilogtail/blob/main/core/common/MachineInfoUtil.cpp

func GetHostIP() (string, error) {
	h, err := os.Hostname()
	if err != nil {
		return "", err
	}

	ip, err := GetHostIPByHostname(h)
	if ip != "" {
		return ip, nil
	}

	if strings.HasPrefix(ip, "127.") || ip == "" {
		ip, err = GetHostIPByInterface("eth0", true)
		if strings.HasPrefix(ip, "127.") || ip == "" {
			ip, err = GetHostIPByInterface("bond0", true)
		}
	}

	if ip != "" {
		return ip, err
	}

	return GetAnyAvailableIP()
}

func GetHostIPByHostname(h string) (string, error) {
	addrs, err := net.LookupHost(h)
	if err != nil {
		return "", err
	}

	return addrs[0], nil
}

func GetHostIPByInterface(iface string, v4 bool) (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}

	for _, i := range ifaces {
		if i.Name != iface {
			continue
		}

		addrs, er := i.Addrs()
		if er != nil {
			return "", er
		}
		if len(addrs) == 0 {
			return "", fmt.Errorf("%s has no addrs", iface)
		}

		switch v := addrs[0].(type) {
		case *net.IPNet:
			if !v.IP.IsLoopback() {
				if v4 && v.IP.To4() != nil || !v4 && v.IP.To16() != nil {
					return v.IP.String(), nil
				}
			}
		case *net.IPAddr:
			if !v.IP.IsLoopback() {
				if v4 && v.IP.To4() != nil || !v4 && v.IP.To16() != nil {
					return v.IP.String(), nil
				}
			}
		}
	}

	return "", nil
}

func GetAnyAvailableIP() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}

	for _, addr := range addrs {
		switch v := addr.(type) {
		case *net.IPNet:
			if !v.IP.IsLoopback() && v.IP.To4() != nil {
				return v.IP.String(), nil
			}
		case *net.IPAddr:
			if !v.IP.IsLoopback() && v.IP.To4() != nil {
				return v.IP.String(), nil
			}
		}
	}

	return "", nil
}
