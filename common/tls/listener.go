package tls

import (
	"context"
	"net"

	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
)

type Listener struct {
	net.Listener
	ctx    context.Context
	logger logger.Logger
	config ServerConfig
}

func NewListener(ctx context.Context, logger logger.Logger, inner net.Listener, config ServerConfig) net.Listener {
	return &Listener{
		Listener: inner,
		ctx:      ctx,
		logger:   logger,
		config:   config,
	}
}

func (l *Listener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	tlsConn, err := ServerHandshake(l.ctx, conn, l.config)
	if err != nil {
		l.logger.Error(E.Cause(err, "accept connection from ", conn.RemoteAddr(), ": "))
		conn.Close()
		return l.Accept()
	}
	return tlsConn, err
}
