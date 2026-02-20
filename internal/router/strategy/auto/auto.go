package auto

import (
	"context"
	"fmt"
	"sync"
)

// Route defines a named route with example prompts for semantic matching.
type Route struct {
	Name     string   `json:"name"`
	Model    string   `json:"model"`
	Examples []string `json:"examples"`
}

// AutoRouter selects the best model group by comparing the user's prompt
// embedding against pre-computed route example embeddings.
type AutoRouter struct {
	mu           sync.RWMutex
	routes       []Route
	routeVectors [][]float32 // averaged embedding per route
	encoder      *Encoder
	defaultModel string
	threshold    float64
	initialized  bool
}

// New creates an AutoRouter.
func New(routes []Route, encoder *Encoder, defaultModel string, threshold float64) *AutoRouter {
	if threshold == 0 {
		threshold = 0.5
	}
	return &AutoRouter{
		routes:       routes,
		encoder:      encoder,
		defaultModel: defaultModel,
		threshold:    threshold,
	}
}

// initVectors lazily computes route embeddings on first call.
func (ar *AutoRouter) initVectors(ctx context.Context) error {
	ar.mu.Lock()
	defer ar.mu.Unlock()

	if ar.initialized {
		return nil
	}

	ar.routeVectors = make([][]float32, len(ar.routes))
	for i, route := range ar.routes {
		if len(route.Examples) == 0 {
			continue
		}
		vecs, err := ar.encoder.Encode(ctx, route.Examples)
		if err != nil {
			return fmt.Errorf("auto router: encode route %q: %w", route.Name, err)
		}
		ar.routeVectors[i] = averageVectors(vecs)
	}

	ar.initialized = true
	return nil
}

// Route selects the best model group for the given messages.
// Returns the model group name to route to.
func (ar *AutoRouter) Route(ctx context.Context, lastUserMessage string) (string, error) {
	if lastUserMessage == "" {
		return ar.defaultModel, nil
	}

	if err := ar.initVectors(ctx); err != nil {
		return ar.defaultModel, nil // fallback on init error
	}

	vecs, err := ar.encoder.Encode(ctx, []string{lastUserMessage})
	if err != nil {
		return ar.defaultModel, nil // fallback on encoding error
	}
	if len(vecs) == 0 || len(vecs[0]) == 0 {
		return ar.defaultModel, nil
	}

	ar.mu.RLock()
	candidates := ar.routeVectors
	ar.mu.RUnlock()

	bestIdx, _ := BestMatch(vecs[0], candidates, ar.threshold)
	if bestIdx < 0 || bestIdx >= len(ar.routes) {
		return ar.defaultModel, nil
	}

	return ar.routes[bestIdx].Model, nil
}

// averageVectors computes the element-wise average of multiple vectors.
func averageVectors(vecs [][]float32) []float32 {
	if len(vecs) == 0 {
		return nil
	}
	dim := len(vecs[0])
	avg := make([]float32, dim)
	for _, v := range vecs {
		for i := range v {
			if i < dim {
				avg[i] += v[i]
			}
		}
	}
	n := float32(len(vecs))
	for i := range avg {
		avg[i] /= n
	}
	return avg
}
