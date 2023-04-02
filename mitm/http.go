package mitm

import (
	std_bufio "bufio"
	"context"
	"crypto/tls"
	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"io"
	"net"
	"net/http"
	"os"
)

var _ Engine = (*HTTPEngine)(nil)

type HTTPEngine struct {
	logger          logger.ContextLogger
	urlRewriteRules []HTTPHandlerFunc
}

func NewHTTPEngine(logger logger.ContextLogger, options option.MITMHTTPOptions) (*HTTPEngine, error) {
	engine := &HTTPEngine{
		logger: logger,
	}
	for i, urlRewritePath := range options.URLRewritePath {
		urlRewriteFile, err := os.Open(C.BasePath(urlRewritePath))
		if err != nil {
			return nil, E.Cause(err, "read url rewrite configuration[", i, "]")
		}
		urlRewriteRules, err := readSurgeURLRewriteRules(urlRewriteFile)
		if err != nil {
			return nil, E.Cause(err, "read url rewrite configuration[", i, "] at ", urlRewritePath)
		}
		engine.urlRewriteRules = append(engine.urlRewriteRules, urlRewriteRules...)
	}
	return engine, nil
}

func (e *HTTPEngine) ProcessConnection(ctx context.Context, clientConn net.Conn, serverConn *tls.Conn, metadata adapter.InboundContext) (net.Conn, error) {
	buffer := buf.NewPacket()
	httpRequest, err := http.ReadRequest(std_bufio.NewReader(io.TeeReader(clientConn, buffer)))
	if err != nil {
		return nil, err
	}
	e.logger.DebugContext(ctx, "HTTP ", httpRequest.Method, " ", httpRequest.URL.String(), " ", httpRequest.Proto)
	var httpServer http.Server
	var handled bool
	httpConn := &httpMITMConn{Conn: bufio.NewCachedConn(clientConn, buffer.ToOwned()), readOnly: true}
	processCtx, cancel := context.WithCancel(ctx)
	httpServer.Handler = http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method == "PRI" && len(request.Header) == 0 && request.URL.Path == "*" && request.Proto == "HTTP/2.0" {
			httpConn.readOnly = false
			h2c.NewHandler(httpServer.Handler, new(http2.Server)).ServeHTTP(writer, request)
			return
		}
		defer cancel()
		httpConn.readOnly = false
		url := *request.URL
		url.Scheme = "https"
		url.Host = request.Host
		urlString := url.String()
		for _, rule := range e.urlRewriteRules {
			if rule(writer, request, urlString) {
				handled = true
				break
			}
		}
		if !handled {
			httpConn.readOnly = true
		}
	})
	_ = httpServer.Serve(&fixedListener{conn: httpConn})
	<-processCtx.Done()
	if !handled {
		if !httpConn.readOnly {
			return nil, E.New("http2 description failed")
		}
		return bufio.NewCachedConn(clientConn, buffer), nil
	}
	serverConn.Close()
	buffer.Release()
	return nil, nil
}

type httpMITMConn struct {
	net.Conn
	readOnly bool
	closed   bool
}

func (c *httpMITMConn) Write(p []byte) (n int, err error) {
	if c.readOnly {
		return 0, os.ErrInvalid
	}
	return c.Conn.Write(p)
}

func (c *httpMITMConn) Close() error {
	c.closed = true
	return nil
}

func (c *httpMITMConn) Upstream() any {
	return c.Conn
}

type fixedListener struct {
	conn net.Conn
}

func (l *fixedListener) Accept() (net.Conn, error) {
	conn := l.conn
	l.conn = nil
	if conn != nil {
		return conn, nil
	}
	return nil, os.ErrClosed
}

func (l *fixedListener) Addr() net.Addr {
	return M.Socksaddr{}
}

func (l *fixedListener) Close() error {
	return nil
}
