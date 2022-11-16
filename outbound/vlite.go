//go:build with_vlite

package outbound

import (
	"context"
	"net"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/dialer"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/transport/vlite"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

var _ adapter.Outbound = (*VLite)(nil)

type VLite struct {
	myOutboundAdapter
	client *vlite.Client
}

func NewVLite(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.VLiteOutboundOptions) (*VLite, error) {
	outbound := &VLite{
		myOutboundAdapter: myOutboundAdapter{
			protocol: C.TypeVLite,
			network:  []string{N.NetworkUDP},
			router:   router,
			logger:   logger,
			tag:      tag,
		},
		client: vlite.NewClient(ctx, dialer.New(router, options.DialerOptions), options),
	}
	return outbound, nil
}

func (d *VLite) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	return d.client.DialContext(ctx, network, destination)
}

func (d *VLite) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	return d.client.ListenPacket(ctx, destination)
}

func (d *VLite) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	return NewConnection(ctx, d, conn, metadata)
}

func (d *VLite) NewPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext) error {
	return NewPacketConnection(ctx, d, conn, metadata)
}
