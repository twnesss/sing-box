//go:build !linux || !with_tcpraw

package tcpraw

import (
	"errors"
	"net"
)

type TCPConn struct{ *net.UDPConn }

// Dial connects to the remote TCP port,
// and returns a single packet-oriented connection
func Dial(network, address string) (*TCPConn, error) {
	return nil, errors.New("tcpraw is not supported on current os")
}

func Listen(network, address string) (*TCPConn, error) {
	return nil, errors.New("os not supported")
}
