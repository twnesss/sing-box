package vlite

import (
	"context"
	"math/rand"
	"net"
	"os"
	"sync"

	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/bufio"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"github.com/mustafaturan/bus"
	"github.com/xiaokangwang/VLite/ass/udpconn2tun"
	"github.com/xiaokangwang/VLite/interfaces"
	"github.com/xiaokangwang/VLite/interfaces/ibus"
	vlite_transport "github.com/xiaokangwang/VLite/transport"
	sctp_server "github.com/xiaokangwang/VLite/transport/packetsctp/sctprelay"
	"github.com/xiaokangwang/VLite/transport/packetuni/puniClient"
	"github.com/xiaokangwang/VLite/transport/udp/udpuni/udpunic"
	"github.com/xiaokangwang/VLite/transport/uni/uniclient"
	worker_client "github.com/xiaokangwang/VLite/workers/client"
)

var _ N.Dialer = (*Client)(nil)

// TODO: how to close?!

type Client struct {
	ctx             context.Context
	msgbus          *bus.Bus
	udpdialer       vlite_transport.UnderlayTransportDialer
	puni            *puniClient.PacketUniClient
	udprelay        *sctp_server.PacketSCTPRelay
	udpserver       *worker_client.UDPClientContext
	TunnelTxToTun   chan interfaces.UDPPacket
	TunnelRxFromTun chan interfaces.UDPPacket
	connAdp         *udpconn2tun.UDPConn2Tun
	access          sync.Mutex
	options         option.VLiteOutboundOptions
}

func NewClient(ctx context.Context, dialer N.Dialer, options option.VLiteOutboundOptions) *Client { //nolint:unparam
	client := &Client{
		options: options,
		msgbus:  ibus.NewMessageBus(),
	}
	ctx = context.WithValue(ctx, interfaces.ExtraOptionsDisableAutoQuitForClient, true) //nolint:revive,staticcheck
	ctx = context.WithValue(ctx, interfaces.ExtraOptionsUDPMask, options.Password)      //nolint:revive,staticcheck
	if options.EnableFEC {
		ctx = context.WithValue(ctx, interfaces.ExtraOptionsUDPFECEnabled, true) //nolint:revive,staticcheck
	}
	if options.ScramblePacket {
		ctx = context.WithValue(ctx, interfaces.ExtraOptionsUDPShouldMask, true) //nolint:revive,staticcheck
	}
	if options.HandshakeMaskingPaddingSize != 0 {
		ctxv := &interfaces.ExtraOptionsUsePacketArmorValue{PacketArmorPaddingTo: options.HandshakeMaskingPaddingSize, UsePacketArmor: true}
		ctx = context.WithValue(ctx, interfaces.ExtraOptionsUsePacketArmor, ctxv) //nolint:revive,staticcheck
	}
	client.udpdialer = NewDialer(ctx, dialer, options.ServerOptions.Build())
	if options.EnableStabilization {
		client.udpdialer = udpunic.NewUdpUniClient(options.Password, ctx, client.udpdialer)
		client.udpdialer = uniclient.NewUnifiedConnectionClient(client.udpdialer, ctx)
	}
	client.ctx = ctx
	return client
}

func (c *Client) Start() error {
	conn, err, connctx := c.udpdialer.Connect(c.ctx)
	if err != nil {
		return err
	}

	C_C2STraffic := make(chan worker_client.UDPClientTxToServerTraffic, 8)         //nolint:revive,stylecheck
	C_C2SDataTraffic := make(chan worker_client.UDPClientTxToServerDataTraffic, 8) //nolint:revive,stylecheck
	C_S2CTraffic := make(chan worker_client.UDPClientRxFromServerTraffic, 8)       //nolint:revive,stylecheck

	C_C2STraffic2 := make(chan interfaces.TrafficWithChannelTag, 8)     //nolint:revive,stylecheck
	C_C2SDataTraffic2 := make(chan interfaces.TrafficWithChannelTag, 8) //nolint:revive,stylecheck
	C_S2CTraffic2 := make(chan interfaces.TrafficWithChannelTag, 8)     //nolint:revive,stylecheck

	go func(ctx context.Context) {
		for {
			select {
			case data := <-C_C2STraffic:
				C_C2STraffic2 <- interfaces.TrafficWithChannelTag(data)
			case <-ctx.Done():
				return
			}
		}
	}(connctx)

	go func(ctx context.Context) {
		for {
			select {
			case data := <-C_C2SDataTraffic:
				C_C2SDataTraffic2 <- interfaces.TrafficWithChannelTag(data)
			case <-ctx.Done():
				return
			}
		}
	}(connctx)

	go func(ctx context.Context) {
		for {
			select {
			case data := <-C_S2CTraffic2:
				C_S2CTraffic <- worker_client.UDPClientRxFromServerTraffic(data)
			case <-ctx.Done():
				return
			}
		}
	}(connctx)

	TunnelTxToTun := make(chan interfaces.UDPPacket)
	TunnelRxFromTun := make(chan interfaces.UDPPacket)

	c.TunnelTxToTun = TunnelTxToTun
	c.TunnelRxFromTun = TunnelRxFromTun

	if c.options.EnableStabilization && c.options.EnableRenegotiation {
		c.puni = puniClient.NewPacketUniClient(C_C2STraffic2, C_C2SDataTraffic2, C_S2CTraffic2, []byte(c.options.Password), connctx)
		c.puni.OnAutoCarrier(conn, connctx)
		c.udpserver = worker_client.UDPClient(connctx, C_C2STraffic, C_C2SDataTraffic, C_S2CTraffic, TunnelTxToTun, TunnelRxFromTun, c.puni)
	} else {
		c.udprelay = sctp_server.NewPacketRelayClient(conn, C_C2STraffic2, C_C2SDataTraffic2, C_S2CTraffic2, []byte(c.options.Password), connctx)
		c.udpserver = worker_client.UDPClient(connctx, C_C2STraffic, C_C2SDataTraffic, C_S2CTraffic, TunnelTxToTun, TunnelRxFromTun, c.udprelay)
	}
	c.ctx = connctx
	c.connAdp = udpconn2tun.NewUDPConn2Tun(TunnelTxToTun, TunnelRxFromTun)
	return nil
}

func (c *Client) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	if network != N.NetworkUDP {
		return nil, os.ErrInvalid
	}
	id, loaded := log.IDFromContext(ctx)
	if !loaded {
		id = rand.Uint32()
	}
	return &bufio.BindPacketConn{PacketConn: c.connAdp.DialUDP(net.UDPAddr{Port: int(id % 65535)}), Addr: destination}, nil
}

func (c *Client) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	id, loaded := log.IDFromContext(ctx)
	if !loaded {
		id = rand.Uint32()
	}
	return c.connAdp.DialUDP(net.UDPAddr{Port: int(id % 65535)}), nil
}
