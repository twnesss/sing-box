package inbound

import (
	"encoding/base64"
	"net/url"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/json"
	C "github.com/sagernet/sing-box/constant"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"github.com/gofrs/uuid"
)

var _ adapter.SubscriptionSupport = (*Shadowsocks)(nil)

func (h *Shadowsocks) GenerateSubscription(options adapter.GenerateSubscriptionOptions) ([]byte, error) {
	serverAddress := options.ServerAddress
	if serverAddress == "" {
		serverAddress = h.listenOptions.Listen.Build().String()
	}
	switch options.Format {
	case C.SubscriptionTypeRaw:
		if options.Application == C.SubscriptionApplicationShadowrocket {
			hostString := base64.URLEncoding.EncodeToString([]byte(
				h.service.Name() + ":" + h.service.Password() + "@" + M.ParseSocksaddrHostPort(serverAddress, h.listenOptions.ListenPort).String(),
			))
			shadowrocketURL := &url.URL{
				Scheme:   "ss",
				Host:     "$",
				Fragment: options.Remarks,
			}
			requestParams := make(url.Values)
			if len(h.network) == 1 && h.network[0] == N.NetworkTCP {
				requestParams.Set("uot", "1")
				requestParams.Set("udp-over-tcp", "1")
				requestParams.Set("udp_over_tcp", "1")
			}
			if len(requestParams) > 0 {
				shadowrocketURL.RawQuery = requestParams.Encode()
			}
			return []byte(strings.ReplaceAll(shadowrocketURL.String(), "$", hostString)), nil
		}

		var useBase64Format bool
		if options.Application != "" || !strings.HasPrefix(h.service.Name(), "2022-") {
			useBase64Format = true
		}

		sip002URL := &url.URL{
			Scheme:   "ss",
			Host:     M.ParseSocksaddrHostPort(serverAddress, h.listenOptions.ListenPort).String(),
			Fragment: options.Remarks,
		}
		var sip002URI string
		if !useBase64Format {
			sip002URL.User = url.UserPassword(h.service.Name(), h.service.Password())
			sip002URI = sip002URL.String()
		} else {
			sip002URL.User = url.User("$")
			sip002URI = strings.ReplaceAll(sip002URL.String(), "$", base64.URLEncoding.EncodeToString([]byte(h.service.Name()+":"+h.service.Password())))
		}
		return []byte(sip002URI), nil
	case C.SubscriptionTypeSIP008:
		return json.Marshal(map[string]any{
			"id":          uuid.NewV5(uuid.Nil, options.Remarks).String(),
			"remarks":     options.Remarks,
			"server":      serverAddress,
			"server_port": h.listenOptions.ListenPort,
			"password":    h.service.Password(),
			"method":      h.service.Name(),
		})
	default:
		return nil, E.New("unknown subscription format ", options.Format)
	}
}
