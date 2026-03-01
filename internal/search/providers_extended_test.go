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

func TestDataForSEO_NameAndMethod(t *testing.T) {
	p := &DataForSEO{}
	if p.Name() != "dataforseo" {
		t.Fatalf("name: %q", p.Name())
	}
	if p.HTTPMethod() != "POST" {
		t.Fatalf("method: %q", p.HTTPMethod())
	}
}

func TestDataForSEO_TransformResponse_InvalidJSON(t *testing.T) {
	p := &DataForSEO{}
	_, err := p.TransformResponse([]byte("not-json"))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDataForSEO_TransformResponse_Valid(t *testing.T) {
	p := &DataForSEO{}
	body := `{"tasks":[{"result":[{"items":[{"type":"organic","url":"https://example.com","title":"Example","description":"Test"}]}]}]}`
	resp, err := p.TransformResponse([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Results) == 0 {
		t.Fatal("expected results")
	}
}

func TestExa_NameAndMethod(t *testing.T) {
	p := &Exa{}
	if p.Name() != "exa_ai" {
		t.Fatalf("name: %q", p.Name())
	}
	if p.HTTPMethod() != "POST" {
		t.Fatalf("method: %q", p.HTTPMethod())
	}
}

func TestExa_TransformResponse_Valid(t *testing.T) {
	p := &Exa{}
	body := `{"results":[{"url":"https://example.com","title":"Test","text":"Content"}]}`
	resp, err := p.TransformResponse([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Results) == 0 {
		t.Fatal("expected results")
	}
}

func TestGooglePSE_TransformResponse_Valid(t *testing.T) {
	p := &GooglePSE{}
	body := `{"items":[{"link":"https://example.com","title":"Google Result","snippet":"Snippet"}]}`
	resp, err := p.TransformResponse([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Results) == 0 {
		t.Fatal("expected results")
	}
}

func TestSearXNG_DefaultAPIBase(t *testing.T) {
	p := &SearXNG{}
	if p.DefaultAPIBase() == "" {
		t.Fatal("empty base")
	}
}

func TestSearXNG_TransformResponse_Valid(t *testing.T) {
	p := &SearXNG{}
	body := `{"results":[{"url":"https://example.com","title":"SearXNG Result","content":"Content"}]}`
	resp, err := p.TransformResponse([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Results) == 0 {
		t.Fatal("expected results")
	}
}

func TestTavily_NameAndBase(t *testing.T) {
	p := &Tavily{}
	if p.Name() != "tavily" {
		t.Fatalf("name: %q", p.Name())
	}
	if p.DefaultAPIBase() == "" {
		t.Fatal("empty base")
	}
}
