package vlite

import (
	"context"
	"net"

	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"github.com/xiaokangwang/VLite/interfaces"
	"github.com/xiaokangwang/VLite/interfaces/ibus"
	vlite_transport "github.com/xiaokangwang/VLite/transport"
	"github.com/xiaokangwang/VLite/transport/udp/packetMasker/masker2conn"
	"github.com/xiaokangwang/VLite/transport/udp/packetMasker/presets/prependandxor"
)

var _ vlite_transport.UnderlayTransportDialer = (*DialerWrapper)(nil)

type DialerWrapper struct {
	ctx         context.Context
	dialer      N.Dialer
	masking     string
	designation M.Socksaddr
}

func NewDialer(ctx context.Context, dialer N.Dialer, destination M.Socksaddr) *DialerWrapper {
	masking := ""
	if v := ctx.Value(interfaces.ExtraOptionsUDPMask); v != nil {
		masking = v.(string)
	}
	return &DialerWrapper{
		ctx:         ctx,
		dialer:      dialer,
		masking:     masking,
		designation: destination,
	}
}

func (d *DialerWrapper) Connect(ctx context.Context) (net.Conn, error, context.Context) {
	conn, err := d.dialer.DialContext(ctx, N.NetworkUDP, d.designation)
	if err != nil {
		return nil, err, nil
	}
	usageConn := conn
	if v := ctx.Value(interfaces.ExtraOptionsUDPShouldMask); v != nil && v.(bool) == true {
		usageConn = masker2conn.NewMaskerAdopter(prependandxor.GetPrependAndPolyXorMask(string(d.masking), []byte{}), conn)
	}
	id := []byte(conn.LocalAddr().String())
	connctx := context.WithValue(d.ctx, interfaces.ExtraOptionsConnID, id)
	connctx = context.WithValue(connctx, interfaces.ExtraOptionsMessageBusByConn, ibus.NewMessageBus())
	return usageConn, nil, connctx
}
