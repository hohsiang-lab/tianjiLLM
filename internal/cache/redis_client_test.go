package cache

import (
	"os"
	"testing"
)

func TestEnvOr(t *testing.T) {
	key := "TEST_ENVOV_KEY_12345"
	os.Unsetenv(key)
	if got := envOr(key, "fallback"); got != "fallback" {
		t.Fatalf("envOr unset = %q, want fallback", got)
	}
	os.Setenv(key, "val")
	defer os.Unsetenv(key)
	if got := envOr(key, "fallback"); got != "val" {
		t.Fatalf("envOr set = %q, want val", got)
	}
}

func TestEnvInt(t *testing.T) {
	key := "TEST_ENVINT_KEY_12345"
	os.Unsetenv(key)
	if got := envInt(key, 42); got != 42 {
		t.Fatalf("envInt unset = %d, want 42", got)
	}
	os.Setenv(key, "7")
	defer os.Unsetenv(key)
	if got := envInt(key, 42); got != 7 {
		t.Fatalf("envInt set = %d, want 7", got)
	}
	os.Setenv(key, "notanumber")
	if got := envInt(key, 42); got != 42 {
		t.Fatalf("envInt invalid = %d, want 42", got)
	}
}

func TestDetectMode(t *testing.T) {
	os.Unsetenv("REDIS_CLUSTER_NODES")
	os.Unsetenv("REDIS_SENTINEL_NODES")
	if got := detectMode(); got != RedisModeStandalone {
		t.Fatalf("detectMode default = %d, want standalone", got)
	}
	os.Setenv("REDIS_CLUSTER_NODES", "a,b")
	defer os.Unsetenv("REDIS_CLUSTER_NODES")
	if got := detectMode(); got != RedisModeCluster {
		t.Fatalf("detectMode cluster = %d, want cluster", got)
	}
	os.Unsetenv("REDIS_CLUSTER_NODES")
	os.Setenv("REDIS_SENTINEL_NODES", "s1")
	defer os.Unsetenv("REDIS_SENTINEL_NODES")
	if got := detectMode(); got != RedisModeSentinel {
		t.Fatalf("detectMode sentinel = %d, want sentinel", got)
	}
}

func TestPoolSize(t *testing.T) {
	os.Unsetenv("REDIS_MAX_CONNECTIONS")
	if got := poolSize(); got != 10 {
		t.Fatalf("poolSize default = %d, want 10", got)
	}
}

func TestUseSSL(t *testing.T) {
	os.Unsetenv("REDIS_SSL")
	if useSSL() {
		t.Fatal("useSSL should be false by default")
	}
	os.Setenv("REDIS_SSL", "true")
	defer os.Unsetenv("REDIS_SSL")
	if !useSSL() {
		t.Fatal("useSSL should be true")
	}
}
