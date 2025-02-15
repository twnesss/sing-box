package provider

import (
	"context"
	"os"
	"runtime"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/urltest"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	"github.com/sagernet/sing/common/rw"
	"github.com/sagernet/sing/service"
	"github.com/sagernet/sing/service/pause"

	R "github.com/dlclark/regexp2"
)

var (
	_ adapter.OutboundProvider        = (*LocalProvider)(nil)
	_ adapter.InterfaceUpdateListener = (*LocalProvider)(nil)
)

type LocalProvider struct {
	myProviderAdapter
}

func NewLocalProvider(ctx context.Context, manager *Manager, router adapter.Router, options option.OutboundProvider, path string) (*LocalProvider, error) {
	localOptions := options.LocalOptions
	interval := time.Duration(localOptions.HealthcheckInterval)
	healthcheckUrl := localOptions.HealthcheckUrl
	if healthcheckUrl == "" {
		healthcheckUrl = "https://www.gstatic.com/generate_204"
	}
	if interval == 0 {
		interval = C.DefaultURLTestInterval
	}
	ctx, cancel := context.WithCancel(ctx)
	provider := &LocalProvider{
		myProviderAdapter: myProviderAdapter{
			ctx:                 ctx,
			manager:             manager,
			cancel:              cancel,
			router:              router,
			logger:              manager.logFactory.NewLogger(F.ToString("provider", "[", options.Tag, "]")),
			tag:                 options.Tag,
			path:                path,
			enableHealthcheck:   localOptions.EnableHealthcheck,
			healthcheckUrl:      localOptions.HealthcheckUrl,
			healthcheckInterval: interval,
			outboundOverride:    options.OutboundOverride,
			types:               options.Types,
			ports:               make(map[int]bool),
			providerType:        C.ProviderTypeLocal,
			close:               make(chan struct{}),
			pauseManager:        service.FromContext[pause.Manager](ctx),
			subInfo:             SubInfo{},
			outbounds:           []adapter.Outbound{},
			outboundByTag:       make(map[string]adapter.Outbound),
		},
	}
	if len(options.Includes) > 0 {
		includes := make([]*R.Regexp, 0, len(options.Includes))
		for i, include := range options.Includes {
			regex, err := R.Compile(include, R.IgnoreCase)
			if err != nil {
				return nil, E.Cause(err, "parse includes[", i, "]")
			}
			includes = append(includes, regex)
		}
		provider.includes = includes
	}
	if options.Excludes != "" {
		regex, err := R.Compile(options.Excludes, R.IgnoreCase)
		if err != nil {
			return nil, E.Cause(err, "parse excludes")
		}
		provider.excludes = regex
	}
	if err := provider.firstStart(options.Ports); err != nil {
		return nil, err
	}
	return provider, nil
}

func (p *LocalProvider) Start() error {
	var history *urltest.HistoryStorage
	if history = service.PtrFromContext[urltest.HistoryStorage](p.ctx); history != nil {
	} else if clashServer := service.FromContext[adapter.ClashServer](p.ctx); clashServer != nil {
		history = clashServer.HistoryStorage()
	} else {
		history = urltest.NewHistoryStorage()
	}
	p.healchcheckHistory = history
	return nil
}

func (p *LocalProvider) PostStart() error {
	go p.loopHealthCheck()
	return nil
}

func (p *LocalProvider) UpdateProvider(ctx context.Context, router adapter.Router) error {
	defer runtime.GC()
	ctx = log.ContextWithNewID(ctx)
	if p.updating.Swap(true) {
		return E.New("provider is updating")
	}
	defer p.updating.Store(false)
	p.logger.DebugContext(ctx, "updating outbound provider ", p.tag, " from local file")
	if !rw.FileExists(p.path) {
		return nil
	}
	fileInfo, _ := os.Stat(p.path)
	fileModeTime := fileInfo.ModTime()
	if fileModeTime == p.lastUpdated {
		return nil
	}

	info, content := p.getContentFromFile(router)
	if len(content) == 0 {
		return nil
	}

	_, err := p.updateProviderFromContent(ctx, router, decodeBase64Safe(content))
	if err != nil {
		p.logger.ErrorContext(ctx, E.Cause(err, "updating outbound provider ", p.tag, " from local file"))
		return err
	}

	p.subInfo = info
	p.lastUpdated = fileModeTime
	p.logger.InfoContext(ctx, "update outbound provider ", p.tag, " success")

	return nil
}
