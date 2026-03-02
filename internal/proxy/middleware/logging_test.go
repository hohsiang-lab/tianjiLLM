package middleware_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"

	mw "github.com/praxisllmlab/tianjiLLM/internal/proxy/middleware"
)

// Test 1: Middleware 存在且能 wrap handler，記錄必要欄位
func TestStructuredLoggingMiddleware(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf).With().Timestamp().Logger()

	r := chi.NewRouter()
	r.Use(chiMiddleware.RequestID)
	r.Use(mw.StructuredLogging(logger))
	r.Get("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	req := httptest.NewRequest("GET", "/v1/chat/completions", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	// Parse log output — expect at least one JSON line with required fields
	scanner := bufio.NewScanner(&buf)
	found := false
	for scanner.Scan() {
		var entry map[string]interface{}
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}
		requiredFields := []string{"request_id", "method", "path", "status", "latency_ms"}
		missing := []string{}
		for _, f := range requiredFields {
			if _, ok := entry[f]; !ok {
				missing = append(missing, f)
			}
		}
		if len(missing) == 0 {
			found = true
			// Verify handler_type field exists (AC #3)
			if _, ok := entry["handler_type"]; !ok {
				t.Error("log entry missing handler_type field")
			}
			break
		}
	}
	if !found {
		t.Fatal("no log entry found with all required fields: request_id, method, path, status, latency_ms")
	}
}

// Test 2: go.mod 包含 rs/zerolog dependency
func TestGoModContainsZerolog(t *testing.T) {
	gomod, err := os.ReadFile("../../../go.mod")
	if err != nil {
		gomod, err = os.ReadFile("go.mod")
		if err != nil {
			t.Fatal("cannot read go.mod:", err)
		}
	}
	if !strings.Contains(string(gomod), "github.com/rs/zerolog") {
		t.Fatal("go.mod does not contain github.com/rs/zerolog dependency")
	}
}

// Test 3: server.go 不再使用 chiMiddleware.Logger
func TestServerNoChiLogger(t *testing.T) {
	cmd := exec.Command("grep", "-n", "chiMiddleware.Logger", "../server.go")
	out, err := cmd.Output()
	if err == nil && len(out) > 0 {
		t.Fatalf("server.go still uses chiMiddleware.Logger:\n%s", string(out))
	}
}
