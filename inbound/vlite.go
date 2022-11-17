//go:build with_vlite

package inbound

import (
	"context"
	"net/netip"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/transport/vlite"
	"github.com/sagernet/sing/common/buf"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/udpnat"
)

type VLite struct {
	myInboundAdapter
	server *vlite.Server
	udpNat *udpnat.Service[netip.AddrPort]
}

func NewVLite(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.VLiteInboundOptions) (*VLite, error) {
	inbound := &VLite{
		myInboundAdapter: myInboundAdapter{
			protocol:      C.TypeVLite,
			network:       []string{N.NetworkUDP},
			ctx:           ctx,
			router:        router,
			logger:        logger,
			tag:           tag,
			listenOptions: options.ListenOptions,
		},
		server: vlite.NewServer(ctx, options),
	}
	inbound.udpNat = udpnat.New[netip.AddrPort](options.UDPTimeout, adapter.NewUpstreamContextHandler(nil, inbound.server.NewPacketConnection, inbound))
	inbound.packetHandler = inbound
	return inbound, nil
}

func (h *VLite) NewPacket(ctx context.Context, conn N.PacketConn, buffer *buf.Buffer, metadata adapter.InboundContext) error {
	h.udpNat.NewContextPacket(ctx, metadata.Source.AddrPort(), buffer, adapter.UpstreamMetadata(metadata), func(natConn N.PacketConn) (context.Context, N.PacketWriter) {
		return adapter.WithContext(log.ContextWithNewID(ctx), &metadata), &tproxyPacketWriter{ctx: ctx, source: natConn, destination: metadata.Destination}
	})
	return nil
}
