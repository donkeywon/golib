package host

import (
	"fmt"
	"net"
	"os"
	"strings"
)

var (
	MyIP, _ = GetHostIP(true)

	TryIfaces = []string{"eth0", "eth1", "eth2", "eth3", "eth4", "eth5", "bond0", "bond1", "bond2", "bond3", "bond4", "bond5"}
)

func GetHostname() (string, error) {
	return os.Hostname()
}

// below inspired by https://github.com/alibaba/ilogtail/blob/main/core/common/MachineInfoUtil.cpp

func GetHostIP(v4 bool) (string, error) {
	h, err := os.Hostname()
	if err != nil {
		return "", err
	}

	ip, err := GetHostIPByHostname(h)
	if ip != "" {
		return ip, nil
	}

	for _, iface := range TryIfaces {
		ip, err = GetHostIPByInterface(iface, v4)
		if ip != "" && !strings.HasPrefix(ip, "127.") {
			return ip, nil
		}
	}

	return GetAnyAvailableIP(v4)
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

		ip, er := GetAnyAvailableIPFromAddrs(addrs, v4)
		if er != nil {
			return "", er
		}
		if ip != "" {
			return ip, nil
		}
	}

	return "", nil
}

func GetAnyAvailableIP(v4 bool) (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}

	return GetAnyAvailableIPFromAddrs(addrs, v4)
}

func GetAllAvailableIPs(v4 bool) ([]string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}

	return GetAllAvailableIPsFromAddrs(addrs, v4)
}

func GetAnyAvailableIPFromAddrs(addrs []net.Addr, v4 bool) (string, error) {
	for _, addr := range addrs {
		switch v := addr.(type) {
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

func GetAllAvailableIPsFromAddrs(addrs []net.Addr, v4 bool) ([]string, error) {
	var ips []string
	for _, addr := range addrs {
		switch v := addr.(type) {
		case *net.IPNet:
			if !v.IP.IsLoopback() {
				if v4 && v.IP.To4() != nil || !v4 && v.IP.To16() != nil {
					ips = append(ips, v.IP.String())
				}
			}
		case *net.IPAddr:
			if !v.IP.IsLoopback() {
				if v4 && v.IP.To4() != nil || !v4 && v.IP.To16() != nil {
					ips = append(ips, v.IP.String())
				}
			}
		}
	}
	return ips, nil
}
