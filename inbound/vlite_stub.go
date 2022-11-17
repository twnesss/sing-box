//go:build !with_vlite

package inbound

import (
	"context"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
)

func NewVLite(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.VLiteInboundOptions) (adapter.Inbound, error) {
	return nil, E.New(`VLite is not included in this build, rebuild with -tags with_vlite`)
}
