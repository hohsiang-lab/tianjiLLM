package metrics

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/callback"
)

// ListenAndServe starts a dedicated HTTP server for Prometheus metrics.
// It reads METRICS_PORT env var (default ":9090"). If METRICS_PORT is empty
// string "", metrics server is disabled. The server shuts down gracefully
// when ctx is cancelled.
func ListenAndServe(ctx context.Context) {
	port := os.Getenv("METRICS_PORT")
	if port == "" {
		// Not set at all → use default
		if _, ok := os.LookupEnv("METRICS_PORT"); ok {
			// Explicitly set to "" → disable
			log.Println("METRICS_PORT is empty, metrics server disabled")
			return
		}
		port = ":9090"
	}

	// Ensure port has colon prefix
	if len(port) > 0 && port[0] != ':' {
		port = ":" + port
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", callback.Handler())

	srv := &http.Server{
		Addr:         port,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("metrics server shutdown error: %v", err)
		}
	}()

	log.Printf("metrics server listening on %s", port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("metrics server error: %v", err)
	}
}

// Addr returns the metrics server address from env, or default.
func Addr() string {
	port := os.Getenv("METRICS_PORT")
	if port == "" {
		if _, ok := os.LookupEnv("METRICS_PORT"); ok {
			return ""
		}
		return ":9090"
	}
	if len(port) > 0 && port[0] != ':' {
		return fmt.Sprintf(":%s", port)
	}
	return port
}
