package callback

import "sync"

// DisabledTokenStore tracks manually disabled OAuth tokens.
// A disabled token is skipped by the upstream selector regardless of utilization.
type DisabledTokenStore interface {
	Disable(tokenKey string)
	Enable(tokenKey string)
	IsDisabled(tokenKey string) bool
}

type inMemoryDisabledTokenStore struct {
	m sync.Map
}

// NewInMemoryDisabledTokenStore returns a thread-safe in-memory DisabledTokenStore.
func NewInMemoryDisabledTokenStore() DisabledTokenStore {
	return &inMemoryDisabledTokenStore{}
}

func (s *inMemoryDisabledTokenStore) Disable(k string) { s.m.Store(k, struct{}{}) }
func (s *inMemoryDisabledTokenStore) Enable(k string)  { s.m.Delete(k) }
func (s *inMemoryDisabledTokenStore) IsDisabled(k string) bool {
	_, ok := s.m.Load(k)
	return ok
}
