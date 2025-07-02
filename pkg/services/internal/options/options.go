package options

import (
	"github.com/4chain-ag/go-wallet-toolbox/pkg/services/internal/httpx"
)

type Service struct {
	RestyClientFactory *httpx.RestyClientFactory
}

func Default() Service {
	return Service{
		RestyClientFactory: httpx.NewRestyClientFactory(),
	}
}
