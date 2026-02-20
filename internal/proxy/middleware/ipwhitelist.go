package middleware

import (
	"net"
	"net/http"
	"sync"
)

// IPWhitelist middleware rejects requests from non-whitelisted IPs.
type IPWhitelist struct {
	mu      sync.RWMutex
	allowed map[string]bool
	enabled bool
}

// NewIPWhitelist creates an IP whitelist. Pass nil for no initial IPs (disabled).
func NewIPWhitelist(ips []string) *IPWhitelist {
	w := &IPWhitelist{
		allowed: make(map[string]bool, len(ips)),
		enabled: len(ips) > 0,
	}
	for _, ip := range ips {
		w.allowed[ip] = true
	}
	return w
}

// Add adds an IP to the whitelist.
func (w *IPWhitelist) Add(ip string) {
	w.mu.Lock()
	w.allowed[ip] = true
	w.enabled = true
	w.mu.Unlock()
}

// Remove removes an IP from the whitelist.
func (w *IPWhitelist) Remove(ip string) {
	w.mu.Lock()
	delete(w.allowed, ip)
	w.enabled = len(w.allowed) > 0
	w.mu.Unlock()
}

// List returns all whitelisted IPs.
func (w *IPWhitelist) List() []string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	ips := make([]string, 0, len(w.allowed))
	for ip := range w.allowed {
		ips = append(ips, ip)
	}
	return ips
}

// Middleware returns the HTTP middleware handler.
func (w *IPWhitelist) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		w.mu.RLock()
		enabled := w.enabled
		w.mu.RUnlock()

		if !enabled {
			next.ServeHTTP(rw, r)
			return
		}

		clientIP := extractIP(r)
		w.mu.RLock()
		allowed := w.allowed[clientIP]
		w.mu.RUnlock()

		if !allowed {
			http.Error(rw, `{"error":"IP not whitelisted"}`, http.StatusForbidden)
			return
		}

		next.ServeHTTP(rw, r)
	})
}

func extractIP(r *http.Request) string {
	// Use RemoteAddr directly. chi's RealIP middleware (if configured)
	// already sets RemoteAddr from trusted proxy headers. Parsing
	// X-Forwarded-For here would let clients bypass the whitelist by
	// spoofing the header.
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
