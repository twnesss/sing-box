package adapter

import (
	"context"
	"net"

	N "github.com/sagernet/sing/common/network"
)

type MITMService interface {
	Service
	ProcessConnection(ctx context.Context, conn net.Conn, dialer N.Dialer, metadata InboundContext) (net.Conn, error)
}
