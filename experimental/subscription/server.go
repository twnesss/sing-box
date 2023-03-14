package subscription

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/tls"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	sHttp "github.com/sagernet/sing/protocol/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"golang.org/x/net/http2"
)

type Server struct {
	ctx        context.Context
	router     adapter.Router
	logger     log.Logger
	httpServer *http.Server
	tlsConfig  tls.ServerConfig
	servers    []ServerItem
}

type ServerItem struct {
	Remarks       string
	ServerAddress string
	Interface     adapter.SubscriptionSupport
}

func NewServer(ctx context.Context, router adapter.Router, logger logger.ContextLogger, options option.SubscriptionOptions) (adapter.SubscriptionServer, error) {
	chiRouter := chi.NewRouter()
	server := &Server{
		ctx:    ctx,
		router: router,
		logger: logger,
		httpServer: &http.Server{
			Addr:    options.Listen,
			Handler: chiRouter,
		},
	}
	if options.TLS != nil && options.TLS.Enabled {
		tlsConfig, err := tls.NewServer(ctx, router, logger, common.PtrValueOrDefault(options.TLS))
		if err != nil {
			return nil, err
		}
		server.tlsConfig = tlsConfig
	}
	listenPrefix := options.ListenPrefix
	if !strings.HasPrefix(listenPrefix, "/") {
		listenPrefix = "/" + listenPrefix
	}
	chiRouter.Get("/", server.handleSubscription)

	for i, serverOptions := range options.Servers {
		serverAddress := serverOptions.ServerAddress
		if serverAddress == "" {
			serverAddress = options.ServerAddress
		}
		if serverAddress == "" {
			return nil, E.New("parse servers[", i, "]: ", "missing server address")
		}
		if serverOptions.InboundTag != "" {
			inbound, loaded := router.Inbound(serverOptions.InboundTag)
			if !loaded {
				return nil, E.New("parse servers[", i, "]: ", "inbound not found: ", serverOptions.InboundTag)
			}
			subscriptionSupport, loaded := inbound.(adapter.SubscriptionSupport)
			if !loaded {
				return nil, E.New("parse servers[", i, "]: ", "inbound have no subscription support: ", serverOptions.InboundTag)
			}
			server.servers = append(server.servers, ServerItem{
				Remarks:       serverOptions.Remarks,
				ServerAddress: serverAddress,
				Interface:     subscriptionSupport,
			})
		}
	}

	return server, nil
}

func (s *Server) Start() error {
	listener, err := net.Listen("tcp", s.httpServer.Addr)
	if err != nil {
		return E.Cause(err, "subscription server listen error")
	}
	if s.tlsConfig != nil {
		s.tlsConfig.SetNextProtos([]string{http2.NextProtoTLS, "http/1.1"})
		err = s.tlsConfig.Start()
		if err != nil {
			return err
		}
		listener = tls.NewListener(s.ctx, s.logger, listener, s.tlsConfig)
	}
	s.logger.Info("subscription server listening at ", listener.Addr())
	go func() {
		err = s.httpServer.Serve(listener)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.logger.Error("subscription server serve error: ", err)
		}
	}()
	return nil
}

func (s *Server) Close() error {
	return common.Close(
		common.PtrOrNil(s.httpServer),
		s.tlsConfig,
	)
}

func (s *Server) handleSubscription(writer http.ResponseWriter, request *http.Request) {
	userAgent := request.Header.Get("User-Agent")
	s.logger.Info("accepted request from ", sHttp.SourceAddress(request), " with User-Agent: ", userAgent)

	subscriptionFormat := chi.URLParam(request, "type")
	if subscriptionFormat == "" {
		subscriptionFormat = C.SubscriptionTypeRaw
	}

	application := chi.URLParam(request, "application")
	if application == "" {
		if strings.Contains(userAgent, "Shadowrocket") {
			application = C.SubscriptionApplicationShadowrocket
		}
	}

	generateOptions := adapter.GenerateSubscriptionOptions{
		Format:      subscriptionFormat,
		Application: application,
	}

	var contentList [][]byte
	for _, serverItem := range s.servers {
		generateOptions.Remarks = serverItem.Remarks
		generateOptions.ServerAddress = serverItem.ServerAddress
		content, err := serverItem.Interface.GenerateSubscription(generateOptions)
		if err != nil {
			// TODO: process error
			continue
		}
		contentList = append(contentList, content)
	}

	switch subscriptionFormat {
	case C.SubscriptionTypeSIP008:
		render.JSON(writer, request, render.M{
			"version": 1,
			"servers": common.Map(contentList, func(content []byte) json.RawMessage {
				return content
			}),
		})
	case C.SubscriptionTypeRaw:
		rawConfig := strings.Join(common.Map(contentList, func(it []byte) string { return string(it) }), "\n")
		render.PlainText(writer, request, base64.URLEncoding.EncodeToString([]byte(rawConfig)))
	default:
		render.Status(request, http.StatusBadRequest)
		render.PlainText(writer, request, "unsupported subscription format: "+subscriptionFormat)
	}
}
