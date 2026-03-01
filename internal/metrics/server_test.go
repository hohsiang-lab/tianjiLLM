package metrics

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	_ "github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetricsServer_StartsAndServesMetrics(t *testing.T) {
	// Use a random high port to avoid conflicts
	t.Setenv("METRICS_PORT", ":19090")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go ListenAndServe(ctx)

	// Wait for server to start
	var resp *http.Response
	var err error
	for i := 0; i < 20; i++ {
		time.Sleep(50 * time.Millisecond)
		resp, err = http.Get("http://localhost:19090/metrics")
		if err == nil {
			break
		}
	}
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	// Should contain at least go runtime metrics
	assert.True(t, len(body) > 0)
}

func TestMetricsServer_DisabledWhenEmpty(t *testing.T) {
	t.Setenv("METRICS_PORT", "")

	addr := Addr()
	assert.Equal(t, "", addr)
}

func TestMetricsServer_MetricNamesExist(t *testing.T) {
	t.Setenv("METRICS_PORT", ":19091")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize prometheus callback to register metrics
	callbackPkg := "github.com/praxisllmlab/tianjiLLM/internal/callback"
	_ = callbackPkg

	go ListenAndServe(ctx)

	var resp *http.Response
	var err error
	for i := 0; i < 20; i++ {
		time.Sleep(50 * time.Millisecond)
		resp, err = http.Get("http://localhost:19091/metrics")
		if err == nil {
			break
		}
	}
	require.NoError(t, err)
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)
	fmt.Println(bodyStr[:min(500, len(bodyStr))])

	// go runtime metrics should always exist
	assert.True(t, strings.Contains(bodyStr, "go_goroutines") || strings.Contains(bodyStr, "go_gc"),
		"Should contain Go runtime metrics")
}
