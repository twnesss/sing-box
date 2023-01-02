package sniff

import (
	"bytes"
	"context"
	"crypto/tls"
	"io"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing/common/bufio"

	"github.com/open-ch/ja3"
)

func TLSClientHello(ctx context.Context, reader io.Reader) (*adapter.InboundContext, error) {
	var clientHello *tls.ClientHelloInfo
	var buffer bytes.Buffer
	err := tls.Server(bufio.NewReadOnlyConn(io.TeeReader(reader, &buffer)), &tls.Config{
		GetConfigForClient: func(argHello *tls.ClientHelloInfo) (*tls.Config, error) {
			clientHello = argHello
			return nil, nil
		},
	}).HandshakeContext(ctx)
	if clientHello != nil {
		metadata := &adapter.InboundContext{Protocol: C.ProtocolTLS, Domain: clientHello.ServerName}
		ja3Result, err := ja3.ComputeJA3FromSegment(buffer.Bytes())
		if err == nil {
			metadata.JA3Fingerprint = ja3Result.GetJA3Hash()
		}
		return metadata, nil
	}
	return nil, err
}
