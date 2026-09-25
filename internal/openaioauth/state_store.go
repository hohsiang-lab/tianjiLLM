package openaioauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/cache"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/openai"
)

const (
	DefaultStateTTL = 10 * time.Minute
	stateKeyPrefix  = "openai_oauth_state:"
	stateLockTTL    = 30 * time.Second
)

var (
	ErrInvalidState = errors.New("invalid openai oauth state")
	ErrExpiredState = errors.New("expired openai oauth state")
)

type StateRecord struct {
	State        string    `json:"state"`
	CodeVerifier string    `json:"code_verifier"`
	OrgID        string    `json:"org_id"`
	RedirectURI  string    `json:"redirect_uri"`
	CreatedAt    time.Time `json:"created_at"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type StateStore struct {
	cache cache.Cache
	now   func() time.Time
}

func NewStateStore(c cache.Cache) *StateStore {
	return &StateStore{cache: c, now: time.Now}
}

func CacheKey(state string) string {
	return stateKeyPrefix + state
}

func (s *StateStore) Create(ctx context.Context, orgID, redirectURI string, ttl time.Duration) (StateRecord, error) {
	if s == nil || s.cache == nil {
		return StateRecord{}, errors.New("openai oauth state cache not configured")
	}
	if redirectURI == "" {
		return StateRecord{}, errors.New("redirect_uri required")
	}
	if ttl <= 0 {
		ttl = DefaultStateTTL
	}
	pkce, err := openai.GeneratePKCE()
	if err != nil {
		return StateRecord{}, err
	}
	state, err := openai.GenerateState()
	if err != nil {
		return StateRecord{}, err
	}
	now := s.now().UTC()
	record := StateRecord{
		State:        state,
		CodeVerifier: pkce.CodeVerifier,
		OrgID:        orgID,
		RedirectURI:  redirectURI,
		CreatedAt:    now,
		ExpiresAt:    now.Add(ttl),
	}
	body, err := json.Marshal(record)
	if err != nil {
		return StateRecord{}, fmt.Errorf("marshal openai oauth state: %w", err)
	}
	if err := s.cache.Set(ctx, CacheKey(state), body, ttl); err != nil {
		return StateRecord{}, fmt.Errorf("store openai oauth state: %w", err)
	}
	return record, nil
}

func (s *StateStore) Consume(ctx context.Context, state string) (StateRecord, error) {
	if s == nil || s.cache == nil {
		return StateRecord{}, errors.New("openai oauth state cache not configured")
	}
	if state == "" {
		return StateRecord{}, ErrInvalidState
	}
	key := CacheKey(state)
	if locker, ok := s.cache.(cache.LockCache); ok {
		lockToken, err := openai.GenerateState()
		if err != nil {
			return StateRecord{}, fmt.Errorf("generate openai oauth state lock token: %w", err)
		}
		acquired, err := locker.AcquireLock(ctx, key+":consume", lockToken, stateLockTTL)
		if err != nil {
			return StateRecord{}, fmt.Errorf("lock openai oauth state: %w", err)
		}
		if !acquired {
			return StateRecord{}, ErrInvalidState
		}
		defer func() { _ = locker.ReleaseLock(context.Background(), key+":consume", lockToken) }()
	}
	var body []byte
	var err error
	if shared, ok := s.cache.(cache.SharedCache); ok {
		body, err = shared.GetShared(ctx, key)
	} else {
		body, err = s.cache.Get(ctx, key)
	}
	if err != nil {
		return StateRecord{}, fmt.Errorf("load openai oauth state: %w", err)
	}
	if len(body) == 0 {
		return StateRecord{}, ErrInvalidState
	}

	var record StateRecord
	if err := json.Unmarshal(body, &record); err != nil {
		_ = s.cache.Delete(ctx, key)
		return StateRecord{}, fmt.Errorf("%w: malformed record", ErrInvalidState)
	}
	if !record.validFor(state) {
		_ = s.cache.Delete(ctx, key)
		return StateRecord{}, ErrInvalidState
	}
	ttl := record.ExpiresAt.Sub(s.now())
	// Redis CAS truncates to milliseconds; zero would create a permanent marker.
	if ttl < time.Millisecond {
		_ = s.cache.Delete(ctx, key)
		return StateRecord{}, ErrExpiredState
	}
	atomic, ok := s.cache.(cache.CompareAndSetCache)
	if !ok {
		return StateRecord{}, ErrInvalidState
	}
	// The lease can expire after the read; only the atomic winner may exchange.
	consumed, consumeErr := atomic.CompareAndSet(ctx, key, body, []byte(`{}`), ttl)
	if consumeErr != nil {
		return StateRecord{}, errors.New("consume openai oauth state: cache write failed")
	}
	if !consumed {
		return StateRecord{}, ErrInvalidState
	}
	return record, nil
}

func (r StateRecord) validFor(state string) bool {
	return r.State == state &&
		r.State != "" &&
		r.CodeVerifier != "" &&
		r.RedirectURI != "" &&
		!r.CreatedAt.IsZero() &&
		!r.ExpiresAt.IsZero()
}
