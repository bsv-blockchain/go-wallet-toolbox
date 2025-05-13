package options

import "github.com/go-resty/resty/v2"

type Service struct {
	HttpClient *resty.Client
}

func Default() Service {
	return Service{
		HttpClient: resty.New(),
	}
}
