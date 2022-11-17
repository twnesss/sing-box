package vlite

import (
	"net"

	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	N "github.com/sagernet/sing/common/network"
)

var _ N.NetPacketConn = (*connWrapper)(nil)

type connWrapper struct {
	N.PacketConn
	done chan struct{}
}

func newConnWrapper(conn N.PacketConn) (N.NetPacketConn, chan struct{}) {
	done := make(chan struct{})
	return &connWrapper{
		PacketConn: conn,
		done:       done,
	}, done
}

func (c *connWrapper) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	return bufio.ReadPacket(c, buf.With(p))
}

func (c *connWrapper) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	return bufio.WritePacket(c, p, addr)
}

func (c *connWrapper) Close() error {
	select {
	case <-c.done:
	default:
		close(c.done)
	}
	return c.PacketConn.Close()
}
