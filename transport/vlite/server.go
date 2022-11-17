package vlite

import (
	"context"
	"io"
	"math/rand"
	"net"
	"strconv"
	"sync"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	N "github.com/sagernet/sing/common/network"

	"github.com/mustafaturan/bus"
	"github.com/xiaokangwang/VLite/interfaces"
	"github.com/xiaokangwang/VLite/interfaces/ibus"
	"github.com/xiaokangwang/VLite/transport"
	sctp_server "github.com/xiaokangwang/VLite/transport/packetsctp/sctprelay"
	"github.com/xiaokangwang/VLite/transport/packetuni/puniServer"
	"github.com/xiaokangwang/VLite/transport/udp/udpServer"
	"github.com/xiaokangwang/VLite/transport/udp/udpuni/udpunis"
	"github.com/xiaokangwang/VLite/transport/uni/uniserver"
	"github.com/xiaokangwang/VLite/workers/server"
)

var (
	_ transport.UnderlayTransportListener = (*Server)(nil)
	_ adapter.PacketConnectionHandler     = (*Server)(nil)
)

type Server struct {
	ctx       context.Context
	msgbus    *bus.Bus
	transport transport.UnderlayTransportListener
	access    sync.Mutex
	options   option.VLiteInboundOptions
}

func NewServer(ctx context.Context, options option.VLiteInboundOptions) *Server {
	server := &Server{
		ctx:     ctx,
		options: options,
		msgbus:  ibus.NewMessageBus(),
	}

	server.ctx = context.WithValue(server.ctx, interfaces.ExtraOptionsMessageBus, server.msgbus) //nolint:revive,staticcheck

	if options.ScramblePacket {
		server.ctx = context.WithValue(server.ctx, interfaces.ExtraOptionsUDPShouldMask, true) //nolint:revive,staticcheck
	}

	if options.EnableFEC {
		server.ctx = context.WithValue(server.ctx, interfaces.ExtraOptionsUDPFECEnabled, true) //nolint:revive,staticcheck
	}

	server.ctx = context.WithValue(server.ctx, interfaces.ExtraOptionsUDPMask, options.Password) //nolint:revive,staticcheck

	if options.HandshakeMaskingPaddingSize != 0 {
		ctxv := &interfaces.ExtraOptionsUsePacketArmorValue{PacketArmorPaddingTo: options.HandshakeMaskingPaddingSize, UsePacketArmor: true}
		server.ctx = context.WithValue(server.ctx, interfaces.ExtraOptionsUsePacketArmor, ctxv) //nolint:revive,staticcheck
	}

	server.transport = server
	if options.EnableStabilization {
		server.transport = uniserver.NewUnifiedConnectionTransportHub(server.transport, server.ctx)
	}
	if options.EnableStabilization {
		server.transport = udpunis.NewUdpUniServer(string(server.options.Password), server.ctx, server.transport)
	}

	return server
}

func (s *Server) Connection(conn net.Conn, ctx context.Context) context.Context {
	S_S2CTraffic := make(chan server.UDPServerTxToClientTraffic, 8)         //nolint:revive,stylecheck
	S_S2CDataTraffic := make(chan server.UDPServerTxToClientDataTraffic, 8) //nolint:revive,stylecheck
	S_C2STraffic := make(chan server.UDPServerRxFromClientTraffic, 8)       //nolint:revive,stylecheck

	S_S2CTraffic2 := make(chan interfaces.TrafficWithChannelTag, 8)     //nolint:revive,stylecheck
	S_S2CDataTraffic2 := make(chan interfaces.TrafficWithChannelTag, 8) //nolint:revive,stylecheck
	S_C2STraffic2 := make(chan interfaces.TrafficWithChannelTag, 8)     //nolint:revive,stylecheck

	go func(ctx context.Context) {
		for {
			select {
			case data := <-S_S2CTraffic:
				S_S2CTraffic2 <- interfaces.TrafficWithChannelTag(data)
			case <-ctx.Done():
				return
			}
		}
	}(ctx)

	go func(ctx context.Context) {
		for {
			select {
			case data := <-S_S2CDataTraffic:
				S_S2CDataTraffic2 <- interfaces.TrafficWithChannelTag(data)
			case <-ctx.Done():
				return
			}
		}
	}(ctx)

	go func(ctx context.Context) {
		for {
			select {
			case data := <-S_C2STraffic2:
				S_C2STraffic <- server.UDPServerRxFromClientTraffic(data)
			case <-ctx.Done():
				return
			}
		}
	}(ctx)

	if !s.options.EnableStabilization || !s.options.EnableRenegotiation {
		relay := sctp_server.NewPacketRelayServer(conn, S_S2CTraffic2, S_S2CDataTraffic2, S_C2STraffic2, s, []byte(s.options.Password), ctx)
		udpserver := server.UDPServer(ctx, S_S2CTraffic, S_S2CDataTraffic, S_C2STraffic, relay)
		_ = udpserver
	} else {
		relay := puniServer.NewPacketUniServer(S_S2CTraffic2, S_S2CDataTraffic2, S_C2STraffic2, s, []byte(s.options.Password), ctx)
		relay.OnAutoCarrier(conn, ctx)
		udpserver := server.UDPServer(ctx, S_S2CTraffic, S_S2CDataTraffic, S_C2STraffic, relay)
		_ = udpserver
	}
	return ctx
}

func (s *Server) RelayStream(conn io.ReadWriteCloser, ctx context.Context) {
}

func (s *Server) NewPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext) error {
	wrapper, done := newConnWrapper(conn)
	pc := &bufio.BindPacketConn{
		PacketConn: wrapper,
		Addr:       metadata.Destination,
	}
	var initialData [1600]byte
	c, err := pc.Read(initialData[:])
	if err != nil {
		return E.Cause(err, "unable to read initial data")
	}
	id, loaded := log.IDFromContext(ctx)
	if !loaded {
		id = rand.Uint32()
	}
	vconn, connctx := udpServer.PrepareIncomingUDPConnection(pc, s.ctx, initialData[:c], strconv.FormatInt(int64(id), 10))
	connctx = s.transport.Connection(vconn, connctx)
	if connctx == nil {
		return E.New("invalid connection discarded")
	}
	select {
	case <-done:
	case <-ctx.Done():
	}
	return nil
}
