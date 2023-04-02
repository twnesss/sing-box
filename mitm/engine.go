package mitm

import (
	"context"
	"crypto/tls"
	"net"

	"github.com/sagernet/sing-box/adapter"
)

type Engine interface {
	ProcessConnection(ctx context.Context, clientConn net.Conn, serverConn *tls.Conn, metadata adapter.InboundContext) (net.Conn, error)
}
