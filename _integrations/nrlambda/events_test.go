package nrlambda

import (
	"testing"

	newrelic "github.com/newrelic/go-agent"
)

func TestProxyRequestWrapping(t *testing.T) {
	// First test an empty APIGatewayProxyRequest
	req := proxyRequest{}

	if h := req.Header(); len(h) != 0 {
		t.Error(h)
	}
	if u := req.URL().String(); u != "" {
		t.Error(u)
	}
	if m := req.Method(); m != "" {
		t.Error(m)
	}
	if tr := req.Transport(); tr != newrelic.TransportUnknown {
		t.Error(tr)
	}
	// Now test a populated request.
	req.request.Headers = map[string]string{
		"X-Forwarded-Port":  "4000",
		"X-Forwarded-Proto": "HTTPS",
	}
	req.request.HTTPMethod = "GET"
	req.request.Path = "the/path"
	if h := req.Header(); len(h) != 2 {
		t.Error(h)
	}
	if u := req.URL().String(); u != "//:4000/the/path" {
		t.Error(u)
	}
	if m := req.Method(); m != "GET" {
		t.Error(m)
	}
	if tr := req.Transport(); tr != newrelic.TransportHTTPS {
		t.Error(tr)
	}
}
