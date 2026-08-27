package httpclient

import "net/http"

type Option func(*options)

type options struct {
	httpClient *http.Client
	requestID  func() string
}

func WithHTTPClient(client *http.Client) Option {
	return func(o *options) {
		o.httpClient = client
	}
}

func WithRequestIDGenerator(gen func() string) Option {
	return func(o *options) {
		o.requestID = gen
	}
}
