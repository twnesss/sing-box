package hysteria

import (
	"net"
	"os"
	"syscall"

	"github.com/sagernet/quic-go"
	"github.com/sagernet/sing-box/common/baderror"
	"github.com/sagernet/sing/common"
)

type AbstractPacketConn interface {
	SetReadBuffer(int) error
	SetWriteBuffer(int) error
	File() (*os.File, error)
}

type PacketConnWrapper struct {
	net.PacketConn
}

func (c *PacketConnWrapper) SetReadBuffer(bytes int) error {
	conn, ok := common.Cast[AbstractPacketConn](c.PacketConn)
	if !ok {
		return os.ErrInvalid
	}
	return conn.SetReadBuffer(bytes)
}

func (c *PacketConnWrapper) SetWriteBuffer(bytes int) error {
	conn, ok := common.Cast[AbstractPacketConn](c.PacketConn)
	if !ok {
		return os.ErrInvalid
	}
	return conn.SetWriteBuffer(bytes)
}

func (c *PacketConnWrapper) SyscallConn() (syscall.RawConn, error) {
	conn, ok := common.Cast[syscall.Conn](c.PacketConn)
	if !ok {
		// fix quic-go
		return &FakeSyscallConn{}, nil
	}
	return conn.SyscallConn()
}

func (c *PacketConnWrapper) File() (f *os.File, err error) {
	r, ok := common.Cast[AbstractPacketConn](c.PacketConn)
	if !ok {
		return nil, os.ErrInvalid
	}
	return r.File()
}

func (c *PacketConnWrapper) Upstream() any {
	return c.PacketConn
}

type StreamWrapper struct {
	Conn quic.Connection
	quic.Stream
}

func (s *StreamWrapper) Read(p []byte) (n int, err error) {
	n, err = s.Stream.Read(p)
	return n, baderror.WrapQUIC(err)
}

func (s *StreamWrapper) Write(p []byte) (n int, err error) {
	n, err = s.Stream.Write(p)
	return n, baderror.WrapQUIC(err)
}

func (s *StreamWrapper) LocalAddr() net.Addr {
	return s.Conn.LocalAddr()
}

func (s *StreamWrapper) RemoteAddr() net.Addr {
	return s.Conn.RemoteAddr()
}

func (s *StreamWrapper) Upstream() any {
	return s.Stream
}

func (s *StreamWrapper) Close() error {
	s.CancelRead(0)
	s.Stream.Close()
	return nil
}
