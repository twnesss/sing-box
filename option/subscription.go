package option

type SubscriptionOptions struct {
	Listen        string                      `json:"listen,omitempty"`
	ListenPrefix  string                      `json:"listen_prefix,omitempty"`
	TLS           *InboundTLSOptions          `json:"tls,omitempty"`
	ServerAddress string                      `json:"server_address,omitempty"`
	Servers       []SubscriptionServerOptions `json:"servers,omitempty"`
}

type SubscriptionServerOptions struct {
	InboundTag    string `json:"inbound_tag,omitempty"`
	ServerAddress string `json:"server_address,omitempty"`
	Remarks       string `json:"remarks,omitempty"`
}
