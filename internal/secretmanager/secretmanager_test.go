package secretmanager

import (
	"context"
	"fmt"
	"testing"
	"time"
)

type mockSM struct {
	name  string
	vals  map[string]string
	err   error
	calls int
}

func (m *mockSM) Name() string { return m.name }
func (m *mockSM) Get(_ context.Context, path string) (string, error) {
	m.calls++
	if m.err != nil {
		return "", m.err
	}
	v, ok := m.vals[path]
	if !ok {
		return "", fmt.Errorf("not found: %s", path)
	}
	return v, nil
}
func (m *mockSM) Health(_ context.Context) error { return m.err }

func TestNewUnknown(t *testing.T) {
	_, err := New("nonexistent", nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRegisterAndNew(t *testing.T) {
	Register("test_mock", func(cfg map[string]any) (SecretManager, error) {
		return &mockSM{name: "test_mock"}, nil
	})
	sm, err := New("test_mock", nil)
	if err != nil {
		t.Fatal(err)
	}
	if sm.Name() != "test_mock" {
		t.Fatalf("got %q", sm.Name())
	}
}

func TestNames(t *testing.T) {
	Register("test_names_a", func(cfg map[string]any) (SecretManager, error) {
		return &mockSM{name: "a"}, nil
	})
	names := Names()
	if len(names) == 0 {
		t.Fatal("expected names")
	}
}

func TestCachedSecretManager(t *testing.T) {
	inner := &mockSM{
		name: "cached",
		vals: map[string]string{"key1": "val1"},
	}
	csm := NewCachedSecretManager(inner, time.Hour)

	if csm.Name() != "cached" {
		t.Fatalf("name: %q", csm.Name())
	}

	// First call
	v, err := csm.Get(context.Background(), "key1")
	if err != nil {
		t.Fatal(err)
	}
	if v != "val1" {
		t.Fatalf("got %q", v)
	}
	if inner.calls != 1 {
		t.Fatalf("calls: %d", inner.calls)
	}

	// Cached
	v, err = csm.Get(context.Background(), "key1")
	if err != nil {
		t.Fatal(err)
	}
	if v != "val1" {
		t.Fatalf("got %q", v)
	}
	if inner.calls != 1 {
		t.Fatalf("should be cached, calls: %d", inner.calls)
	}
}

func TestCachedSecretManagerExpiry(t *testing.T) {
	inner := &mockSM{
		name: "expiry",
		vals: map[string]string{"k": "v"},
	}
	csm := NewCachedSecretManager(inner, time.Millisecond)

	_, err := csm.Get(context.Background(), "k")
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(5 * time.Millisecond)

	_, err = csm.Get(context.Background(), "k")
	if err != nil {
		t.Fatal(err)
	}
	if inner.calls != 2 {
		t.Fatalf("expected 2 calls after expiry, got %d", inner.calls)
	}
}

func TestCachedSecretManagerEmptyPath(t *testing.T) {
	inner := &mockSM{name: "empty"}
	csm := NewCachedSecretManager(inner, time.Hour)
	_, err := csm.Get(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestCachedSecretManagerError(t *testing.T) {
	inner := &mockSM{name: "err", err: fmt.Errorf("fail")}
	csm := NewCachedSecretManager(inner, time.Hour)
	_, err := csm.Get(context.Background(), "key")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCachedSecretManagerHealth(t *testing.T) {
	inner := &mockSM{name: "health"}
	csm := NewCachedSecretManager(inner, time.Hour)
	if err := csm.Health(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestCachedSecretManagerDefaultTTL(t *testing.T) {
	inner := &mockSM{name: "default", vals: map[string]string{"x": "y"}}
	csm := NewCachedSecretManager(inner, 0) // should default to 24h
	if csm.ttl != 86400*time.Second {
		t.Fatalf("ttl: %v", csm.ttl)
	}
}

func TestAWSSecretManagerName(t *testing.T) {
	sm := &AWSSecretManager{}
	if sm.Name() != "aws_secrets_manager" {
		t.Fatalf("got %q", sm.Name())
	}
}

func TestAzureKeyVaultName(t *testing.T) {
	sm := &AzureKeyVault{}
	if sm.Name() != "azure_key_vault" {
		t.Fatalf("got %q", sm.Name())
	}
}

func TestHashiCorpVaultName(t *testing.T) {
	sm := &HashiCorpVault{}
	if sm.Name() != "hashicorp_vault" {
		t.Fatalf("got %q", sm.Name())
	}
}

func TestGoogleSecretManagerName(t *testing.T) {
	sm := &GoogleSecretManager{}
	if sm.Name() != "google_secret_manager" {
		t.Fatalf("got %q", sm.Name())
	}
}

func TestConjurSecretManagerName(t *testing.T) {
	sm := &ConjurSecretManager{}
	if sm.Name() != "cyberark_conjur" {
		t.Fatalf("got %q", sm.Name())
	}
}
