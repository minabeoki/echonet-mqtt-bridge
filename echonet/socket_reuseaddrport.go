//go:build darwin || bsd || freebsd

package echonet

import "syscall"

func listenControl(network, address string, c syscall.RawConn) error {
	var operr error
	err := c.Control(func(fd uintptr) {
		operr = syscall.SetsockoptInt(int(fd),
			syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
		if operr != nil {
			return
		}
		operr = syscall.SetsockoptInt(int(fd),
			syscall.SOL_SOCKET, syscall.SO_REUSEPORT, 1)
	})
	if err != nil {
		return err
	}
	return operr
}
