package adapter

type SubscriptionSupport interface {
	GenerateSubscription(options GenerateSubscriptionOptions) ([]byte, error)
}

type GenerateSubscriptionOptions struct {
	Format        string
	Application   string
	Remarks       string
	ServerAddress string
}

type SubscriptionServer interface {
	Service
}
