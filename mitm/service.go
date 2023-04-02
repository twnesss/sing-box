package mitm

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"os"

	"github.com/sagernet/sing-box/adapter"
	sTLS "github.com/sagernet/sing-box/common/tls"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	N "github.com/sagernet/sing/common/network"

	"github.com/fsnotify/fsnotify"
)

var _ adapter.MITMService = (*Service)(nil)

type Service struct {
	router          adapter.Router
	logger          logger.ContextLogger
	tlsCertificate  *tls.Certificate
	certificate     []byte
	key             []byte
	certificatePath string
	keyPath         string
	watcher         *fsnotify.Watcher
	insecure        bool
	engines         []Engine
}

func NewService(router adapter.Router, logger logger.ContextLogger, options option.MITMServiceOptions) (*Service, error) {
	var tlsCertificate *tls.Certificate
	var certificate []byte
	var key []byte
	if options.Certificate != "" {
		certificate = []byte(options.Certificate)
	} else if options.CertificatePath != "" {
		content, err := os.ReadFile(options.CertificatePath)
		if err != nil {
			return nil, E.Cause(err, "read certificate")
		}
		certificate = content
	}
	if options.Key != "" {
		key = []byte(options.Key)
	} else if options.KeyPath != "" {
		content, err := os.ReadFile(options.KeyPath)
		if err != nil {
			return nil, E.Cause(err, "read key")
		}
		key = content
	}

	if certificate == nil && key != nil {
		return nil, E.New("missing certificate")
	} else if certificate != nil && key == nil {
		return nil, E.New("missing key")
	} else if certificate != nil && key != nil {
		keyPair, err := tls.X509KeyPair(certificate, key)
		if err != nil {
			return nil, E.Cause(err, "parse x509 key pair")
		}
		tlsCertificate = &keyPair
	}

	service := &Service{
		router:          router,
		logger:          logger,
		tlsCertificate:  tlsCertificate,
		certificate:     certificate,
		key:             key,
		certificatePath: options.CertificatePath,
		keyPath:         options.KeyPath,
		insecure:        options.Insecure,
	}

	if options.HTTP != nil && options.HTTP.Enabled {
		engine, err := NewHTTPEngine(logger, common.PtrValueOrDefault(options.HTTP))
		if err != nil {
			return nil, err
		}
		service.engines = append(service.engines, engine)
	}

	return service, nil
}

func (s *Service) ProcessConnection(ctx context.Context, conn net.Conn, dialer N.Dialer, metadata adapter.InboundContext) (net.Conn, error) {
	buffer := buf.NewPacket()
	buffer.FullReset()
	var clientHello *tls.ClientHelloInfo
	_ = tls.Server(bufio.NewReadOnlyConn(io.TeeReader(conn, buffer)), &tls.Config{
		GetConfigForClient: func(argHello *tls.ClientHelloInfo) (*tls.Config, error) {
			clientHello = argHello
			return nil, nil
		},
	}).HandshakeContext(ctx)
	if clientHello == nil {
		s.logger.DebugContext(ctx, "not a TLS connection")
		return bufio.NewCachedConn(conn, buffer), nil
	}
	ctx = adapter.WithContext(ctx, &metadata)
	var outConn net.Conn
	var err error
	if len(metadata.DestinationAddresses) > 0 {
		outConn, err = N.DialSerial(ctx, dialer, N.NetworkTCP, metadata.Destination, metadata.DestinationAddresses)
	} else {
		outConn, err = dialer.DialContext(ctx, N.NetworkTCP, metadata.Destination)
	}
	if err != nil {
		return nil, N.HandshakeFailure(conn, err)
	}
	tlsConfig := sTLS.ConfigFromClientHello(clientHello)
	tlsConfig.InsecureSkipVerify = s.insecure
	tlsConfig.Time = s.router.TimeFunc()
	if tlsConfig.ServerName == "" {
		tlsConfig.ServerName = metadata.Destination.AddrString()
	}
	serverConn := tls.Client(outConn, tlsConfig)
	err = serverConn.HandshakeContext(ctx)
	if err != nil {
		return nil, N.HandshakeFailure(conn, err)
	}
	var serverConfig tls.Config
	serverConfig.Time = s.router.TimeFunc()
	if serverConn.ConnectionState().NegotiatedProtocol != "" {
		serverConfig.NextProtos = []string{serverConn.ConnectionState().NegotiatedProtocol}
	}
	serverConfig.ServerName = clientHello.ServerName
	serverConfig.MinVersion = tls.VersionTLS10
	serverConfig.GetCertificate = func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
		return sTLS.GenerateKeyPair(nil, serverConfig.ServerName, s.tlsCertificate)
	}

	clientTLSConn := tls.Server(bufio.NewCachedConn(conn, buffer), &serverConfig)
	err = clientTLSConn.HandshakeContext(ctx)
	if err != nil {
		return nil, E.Cause(err, "mitm TLS handshake")
	}

	var clientConn net.Conn = clientTLSConn
	for _, engine := range s.engines {
		clientConn, err = engine.ProcessConnection(ctx, clientTLSConn, serverConn, metadata)
		if conn == nil {
			return nil, err
		}
	}
	s.logger.DebugContext(ctx, "mitm TLS handshake success")
	return nil, bufio.CopyConn(ctx, clientConn, serverConn)
}

func (s *Service) Start() error {
	if s.certificatePath != "" || s.keyPath != "" {
		err := s.startWatcher()
		if err != nil {
			s.logger.Warn("create fsnotify watcher: ", err)
		}
	}
	return nil
}

func (s *Service) Close() error {
	return common.Close(common.PtrOrNil(s.watcher))
}
