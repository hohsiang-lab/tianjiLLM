package openaitest

import (
	"fmt"
	"net/http"
)

var forbiddenOpenAIHosts = map[string]struct{}{
	"auth.openai.com": {},
	"api.openai.com":  {},
}

type noRealOpenAIGuard struct {
	base         http.RoundTripper
	allowedHosts map[string]struct{}
}

func NewNoRealOpenAIGuard(base http.RoundTripper, allowedHosts ...string) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	allowed := make(map[string]struct{}, len(allowedHosts))
	for _, host := range allowedHosts {
		allowed[host] = struct{}{}
	}
	return &noRealOpenAIGuard{base: base, allowedHosts: allowed}
}

func NewGuardedClient(allowedHosts ...string) *http.Client {
	return &http.Client{Transport: NewNoRealOpenAIGuard(http.DefaultTransport, allowedHosts...)}
}

func (g *noRealOpenAIGuard) RoundTrip(req *http.Request) (*http.Response, error) {
	host := req.URL.Host
	if _, ok := g.allowedHosts[host]; ok {
		return g.base.RoundTrip(req)
	}
	if _, forbidden := forbiddenOpenAIHosts[req.URL.Hostname()]; forbidden {
		return nil, fmt.Errorf("real OpenAI network call blocked in test: %s", req.URL.Hostname())
	}
	return g.base.RoundTrip(req)
}
