package search

import (
	"os"
	"testing"
)

func TestBraveValidateEnvironment(t *testing.T) {
	b := &Brave{}
	_, err := b.ValidateEnvironment("", "")
	if err == nil {
		t.Fatal("expected error for empty key")
	}
	h, err := b.ValidateEnvironment("key123", "")
	if err != nil {
		t.Fatal(err)
	}
	if h.Get("X-Subscription-Token") != "key123" {
		t.Fatalf("got %q", h.Get("X-Subscription-Token"))
	}
}

func TestBraveGetCompleteURL(t *testing.T) {
	b := &Brave{}
	u := b.GetCompleteURL("", SearchParams{Query: "test", MaxResults: 5})
	if u == "" {
		t.Fatal("empty url")
	}
	u2 := b.GetCompleteURL("", SearchParams{Query: "test", MaxResults: 0, SearchDomainFilter: []string{"example.com"}})
	if u2 == "" {
		t.Fatal("empty url")
	}
	u3 := b.GetCompleteURL("", SearchParams{Query: "test", MaxResults: 100})
	if u3 == "" {
		t.Fatal("empty url")
	}
}

func TestBraveHTTPMethod(t *testing.T) {
	b := &Brave{}
	if b.HTTPMethod() != "GET" {
		t.Fatalf("got %q", b.HTTPMethod())
	}
}

func TestBraveTransformRequest(t *testing.T) {
	b := &Brave{}
	r := b.TransformRequest(SearchParams{})
	if r != nil {
		t.Fatal("expected nil")
	}
}

func TestExaValidateEnvironment(t *testing.T) {
	e := &Exa{}
	_, err := e.ValidateEnvironment("", "")
	if err == nil {
		t.Fatal("expected error")
	}
	h, err := e.ValidateEnvironment("key", "")
	if err != nil {
		t.Fatal(err)
	}
	if h.Get("x-api-key") != "key" {
		t.Fatalf("got %q", h.Get("x-api-key"))
	}
}

func TestExaGetCompleteURL(t *testing.T) {
	e := &Exa{}
	u := e.GetCompleteURL("", SearchParams{})
	if u != "https://api.exa.ai/search" {
		t.Fatalf("got %q", u)
	}
	u2 := e.GetCompleteURL("http://custom", SearchParams{})
	if u2 != "http://custom" {
		t.Fatalf("got %q", u2)
	}
}

func TestExaTransformRequest(t *testing.T) {
	e := &Exa{}
	r := e.TransformRequest(SearchParams{Query: "test", MaxResults: 5})
	if r == nil {
		t.Fatal("expected non-nil")
	}
}

func TestDataForSEOValidateEnvironment(t *testing.T) {
	d := &DataForSEO{}
	_, err := d.ValidateEnvironment("", "")
	if err == nil {
		t.Fatal("expected error")
	}
	os.Setenv("DATAFORSEO_LOGIN", "user")
	os.Setenv("DATAFORSEO_PASSWORD", "pass")
	defer os.Unsetenv("DATAFORSEO_LOGIN")
	defer os.Unsetenv("DATAFORSEO_PASSWORD")
	h, err := d.ValidateEnvironment("", "")
	if err != nil {
		t.Fatal(err)
	}
	if h.Get("Authorization") == "" {
		t.Fatal("expected auth header")
	}
}

func TestDataForSEOGetCompleteURL(t *testing.T) {
	d := &DataForSEO{}
	u := d.GetCompleteURL("", SearchParams{})
	if u == "" {
		t.Fatal("empty")
	}
}

func TestDataForSEOTransformRequest(t *testing.T) {
	d := &DataForSEO{}
	r := d.TransformRequest(SearchParams{Query: "test", MaxResults: 5})
	if r == nil {
		t.Fatal("expected non-nil")
	}
}
