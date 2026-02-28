package search

import (
	"os"
	"testing"
)

func TestFirecrawlProvider(t *testing.T) {
	p := &FirecrawlProvider{}
	if p.Name() != "firecrawl" {
		t.Fatalf("name: %q", p.Name())
	}
	if p.HTTPMethod() != "POST" {
		t.Fatalf("method: %q", p.HTTPMethod())
	}
	if p.DefaultAPIBase() == "" {
		t.Fatal("empty base")
	}

	_, err := p.ValidateEnvironment("", "")
	if err == nil {
		t.Fatal("expected error")
	}
	h, err := p.ValidateEnvironment("key", "")
	if err != nil {
		t.Fatal(err)
	}
	if h.Get("Authorization") != "Bearer key" {
		t.Fatalf("auth: %q", h.Get("Authorization"))
	}

	u := p.GetCompleteURL("", SearchParams{})
	if u == "" {
		t.Fatal("empty url")
	}

	r := p.TransformRequest(SearchParams{Query: "test", MaxResults: 5})
	if r == nil {
		t.Fatal("nil request")
	}
}

func TestGooglePSE(t *testing.T) {
	p := &GooglePSE{}
	if p.Name() != "google_pse" {
		t.Fatalf("name: %q", p.Name())
	}
	if p.HTTPMethod() != "GET" {
		t.Fatalf("method: %q", p.HTTPMethod())
	}

	_, err := p.ValidateEnvironment("", "")
	if err == nil {
		t.Fatal("expected error")
	}

	os.Setenv("GOOGLE_PSE_ENGINE_ID", "eid")
	defer os.Unsetenv("GOOGLE_PSE_ENGINE_ID")

	h, err := p.ValidateEnvironment("key", "")
	if err != nil {
		t.Fatal(err)
	}
	if h == nil {
		t.Fatal("nil headers")
	}

	u := p.GetCompleteURL("", SearchParams{Query: "test", MaxResults: 5})
	if u == "" {
		t.Fatal("empty url")
	}

	r := p.TransformRequest(SearchParams{})
	if r != nil {
		t.Fatal("expected nil")
	}
}

func TestLinkupProvider(t *testing.T) {
	p := &LinkupProvider{}
	if p.Name() != "linkup" {
		t.Fatalf("name: %q", p.Name())
	}

	_, err := p.ValidateEnvironment("", "")
	if err == nil {
		t.Fatal("expected error")
	}
	h, err := p.ValidateEnvironment("key", "")
	if err != nil {
		t.Fatal(err)
	}
	if h.Get("Authorization") != "Bearer key" {
		t.Fatalf("auth: %q", h.Get("Authorization"))
	}

	u := p.GetCompleteURL("http://custom", SearchParams{})
	if u != "http://custom" {
		t.Fatalf("url: %q", u)
	}

	r := p.TransformRequest(SearchParams{Query: "q", MaxResults: 3})
	if r == nil {
		t.Fatal("nil request")
	}
}

func TestSearXNG(t *testing.T) {
	p := &SearXNG{}
	if p.Name() != "searxng" {
		t.Fatalf("name: %q", p.Name())
	}
	if p.HTTPMethod() != "GET" {
		t.Fatalf("method: %q", p.HTTPMethod())
	}

	_, err := p.ValidateEnvironment("", "")
	if err == nil {
		t.Fatal("expected error for empty base")
	}
	h, err := p.ValidateEnvironment("", "http://localhost:8888/search")
	if err != nil {
		t.Fatal(err)
	}
	if h == nil {
		t.Fatal("nil headers")
	}

	u := p.GetCompleteURL("http://localhost:8888/search", SearchParams{Query: "test", Country: "us"})
	if u == "" {
		t.Fatal("empty url")
	}

	r := p.TransformRequest(SearchParams{})
	if r != nil {
		t.Fatal("expected nil for GET")
	}
}
