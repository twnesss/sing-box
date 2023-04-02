package adapter

import (
	"context"
	"crypto/tls"
	"net"

	N "github.com/sagernet/sing/common/network"
)

type MITMService interface {
	Service
	ProcessConnection(ctx context.Context, conn net.Conn, dialer N.Dialer, metadata InboundContext) (net.Conn, error)
}

type TLSOutbound interface {
	NewTLSConnection(ctx context.Context, conn net.Conn, tlsConfig *tls.Config, metadata InboundContext) error
}
