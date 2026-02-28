package search

import (
	"testing"
)

func TestRegistryList(t *testing.T) {
	names := List()
	if len(names) == 0 {
		t.Fatal("expected registered providers from init()")
	}
	// brave should be registered via init
	found := false
	for _, n := range names {
		if n == "brave" {
			found = true
		}
	}
	if !found {
		t.Fatalf("brave not found in %v", names)
	}
}

func TestRegistryGet(t *testing.T) {
	p, err := Get("brave")
	if err != nil {
		t.Fatal(err)
	}
	if p.Name() != "brave" {
		t.Fatalf("got %q", p.Name())
	}
}

func TestRegistryGetNotFound(t *testing.T) {
	_, err := Get("nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
}
