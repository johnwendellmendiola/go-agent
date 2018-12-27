package nrlambda

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	newrelic "github.com/newrelic/go-agent"
)

type proxyRequest struct{ request events.APIGatewayProxyRequest }

var _ newrelic.WebRequest = &proxyRequest{}

func (r proxyRequest) Header() http.Header {
	// In the future there might be a method to do this:
	// https://github.com/aws/aws-lambda-go/issues/131
	h := make(http.Header, len(r.request.Headers))
	for k, v := range r.request.Headers {
		h.Set(k, v)
	}
	return h
}

func (r proxyRequest) URL() *url.URL {
	var host string
	if port := r.request.Headers["X-Forwarded-Port"]; port != "" {
		host = ":" + port
	}
	return &url.URL{
		Path: r.request.Path,
		Host: host,
	}
}

func (r proxyRequest) Method() string {
	return r.request.HTTPMethod
}

func (r proxyRequest) Transport() newrelic.TransportType {
	proto := strings.ToLower(r.request.Headers["X-Forwarded-Proto"])
	switch proto {
	case "https":
		return newrelic.TransportHTTPS
	case "http":
		return newrelic.TransportHTTP
	}
	return newrelic.TransportUnknown
}
