package main

import (
	"context"
	"net"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/net/ipv4"
)

func multicastSocket(ip string, port int) (*ipv4.PacketConn, error) {
	lc := &net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) (err error) {
			return c.Control(func(fd uintptr) {
				err = syscall.SetsockoptInt(int(fd),
					syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
				if err != nil {
					return
				}
				//err = syscall.SetsockoptInt(int(fd),
				//	syscall.SOL_SOCKET, syscall.SO_REUSEPORT, 1)
			})
		},
	}

	l, err := lc.ListenPacket(context.Background(), "udp4",
		":"+strconv.Itoa(ECHONET_PORT))
	if err != nil {
		return nil, err
	}

	conn := ipv4.NewPacketConn(l)

	err = conn.SetControlMessage(ipv4.FlagDst, true)
	if err != nil {
		return nil, err
	}

	udpAddr := &net.UDPAddr{
		IP:   net.ParseIP(ip),
		Port: port,
	}

	ifaces, err := getInterfaces()
	if err != nil {
		return nil, err
	}

	for _, iface := range ifaces {
		addrs, err := getAddresses(iface)
		if err != nil || len(addrs) == 0 {
			continue
		}
		err = conn.JoinGroup(iface, udpAddr)
		if err != nil {
			return nil, err
		}
	}

	return conn, err
}

func getInterfaces() ([]*net.Interface, error) {
	allifs, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	var ifs []*net.Interface
	for _, iface := range allifs {
		if iface.Flags&net.FlagUp == 0 {
			continue // interface down
		}
		if iface.Flags&net.FlagLoopback != 0 {
			continue // loopback interface
		}
		if iface.Flags&net.FlagMulticast == 0 {
			continue // no multicast
		}
		if iface.Flags&net.FlagPointToPoint != 0 {
			continue // p2p interface
		}
		if strings.HasPrefix(iface.Name, "utun") ||
			strings.HasPrefix(iface.Name, "llw") ||
			strings.HasPrefix(iface.Name, "awdl") {
			continue
		}

		addrs, err := getAddresses(&iface)
		if err != nil || len(addrs) == 0 {
			continue // no ipv4 addresses
		}

		ifs = append(ifs, &iface)
	}
	return ifs, nil
}

func getAddresses(iface *net.Interface) ([]string, error) {
	addrs, err := iface.Addrs()
	if err != nil || len(addrs) == 0 {
		return nil, err
	}
	if len(addrs) == 0 {
		return nil, nil
	}

	var retval []string
	for _, addr := range addrs {
		ss := strings.Split(addr.String(), "/")
		if len(ss) == 0 || len(ss[0]) == 0 {
			continue
		}
		if strings.Index(ss[0], ":") >= 0 {
			continue // skip ipv6
		}
		retval = append(retval, ss[0])
	}
	return retval, nil
}
