package openaioauth

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/cache"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/openai"
)

const (
	DefaultDeviceAuthTTL  = 15 * time.Minute
	deviceAuthKeyPrefix   = "openai_device_auth:"
	deviceAuthLockPrefix  = "openai_device_auth_lock:"
	deviceAuthLockTTL     = 2 * time.Minute
	deviceAuthExpiryGrace = time.Minute
	deviceAuthDefaultPoll = 5
	deviceAuthRetryLimit  = 3
)

var (
	ErrDeviceAuthNotFound         = errors.New("openai device authorization not found")
	ErrDeviceAuthExpired          = errors.New("openai device authorization expired")
	ErrDeviceAuthOwnership        = errors.New("openai device authorization ownership mismatch")
	ErrDeviceAuthCacheUnavailable = errors.New("openai device authorization cache coordination unavailable")
	ErrDeviceAuthInvalid          = errors.New("invalid openai device authorization record")
)

type DeviceAuthStatus string

const (
	DeviceAuthStatusPending    DeviceAuthStatus = "pending"
	DeviceAuthStatusExchanging DeviceAuthStatus = "exchanging"
	DeviceAuthStatusSuccess    DeviceAuthStatus = "success"
	DeviceAuthStatusFailed     DeviceAuthStatus = "failed"
	DeviceAuthStatusExpired    DeviceAuthStatus = "expired"
	DeviceAuthStatusDenied     DeviceAuthStatus = "denied"
	DeviceAuthStatusCancelled  DeviceAuthStatus = "cancelled"
)

type DeviceAuthRecord struct {
	FlowID          string           `json:"flow_id"`
	DeviceAuthID    string           `json:"device_auth_id"`
	UserCode        string           `json:"user_code"`
	TokenPollURL    string           `json:"token_poll_url"`
	VerificationURI string           `json:"verification_uri"`
	RedirectURI     string           `json:"redirect_uri"`
	OrgID           string           `json:"org_id"`
	SessionBinding  string           `json:"session_binding"`
	IntervalSeconds int64            `json:"interval_seconds"`
	NextPollAt      time.Time        `json:"next_poll_at"`
	CreatedAt       time.Time        `json:"created_at"`
	UpdatedAt       time.Time        `json:"updated_at"`
	ExpiresAt       time.Time        `json:"expires_at"`
	Status          DeviceAuthStatus `json:"status"`
	CredentialID    string           `json:"credential_id,omitempty"`
	ErrorCode       string           `json:"error_code,omitempty"`
	RetryCount      int              `json:"retry_count,omitempty"`
	Generation      string           `json:"generation,omitempty"`
}

func (r DeviceAuthRecord) OwnedBy(sessionBinding string) bool {
	if r.SessionBinding == "" || sessionBinding == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(r.SessionBinding), []byte(sessionBinding)) == 1
}

func (r DeviceAuthRecord) Terminal() bool {
	switch r.Status {
	case DeviceAuthStatusSuccess, DeviceAuthStatusFailed, DeviceAuthStatusExpired, DeviceAuthStatusDenied, DeviceAuthStatusCancelled:
		return true
	default:
		return false
	}
}

type DeviceStore struct {
	cache cache.Cache
	now   func() time.Time
}

func NewDeviceStore(c cache.Cache) *DeviceStore {
	return &DeviceStore{cache: c, now: time.Now}
}

func DeviceAuthCacheKey(flowID string) string {
	return deviceAuthKeyPrefix + flowID
}

func DeviceAuthLockKey(flowID string) string {
	return deviceAuthLockPrefix + flowID
}

func (s *DeviceStore) Create(ctx context.Context, record DeviceAuthRecord, ttl time.Duration) (DeviceAuthRecord, error) {
	if s == nil || s.cache == nil {
		return DeviceAuthRecord{}, ErrDeviceAuthCacheUnavailable
	}
	if record.DeviceAuthID == "" || record.UserCode == "" || record.SessionBinding == "" || !hasRequiredDeviceAuthEndpoints(record) {
		return DeviceAuthRecord{}, ErrDeviceAuthInvalid
	}
	if ttl <= 0 {
		ttl = DefaultDeviceAuthTTL
	}
	flowID, err := openai.GenerateState()
	if err != nil {
		return DeviceAuthRecord{}, fmt.Errorf("generate openai device flow ID: %w", err)
	}
	now := s.now().UTC()
	if record.IntervalSeconds <= 0 {
		record.IntervalSeconds = deviceAuthDefaultPoll
	}
	record.FlowID = flowID
	record.CreatedAt = now
	record.UpdatedAt = now
	record.ExpiresAt = now.Add(ttl)
	record.NextPollAt = now.Add(time.Duration(record.IntervalSeconds) * time.Second)
	record.Status = DeviceAuthStatusPending
	record.CredentialID = ""
	record.ErrorCode = ""
	record.RetryCount = 0
	record.Generation = "pending:" + flowID
	if err := s.saveWithTTL(ctx, record); err != nil {
		return DeviceAuthRecord{}, err
	}
	return record, nil
}

func (s *DeviceStore) Get(ctx context.Context, flowID string) (DeviceAuthRecord, error) {
	if s == nil || s.cache == nil {
		return DeviceAuthRecord{}, ErrDeviceAuthCacheUnavailable
	}
	if flowID == "" {
		return DeviceAuthRecord{}, ErrDeviceAuthNotFound
	}
	body, err := s.read(ctx, DeviceAuthCacheKey(flowID))
	if err != nil {
		return DeviceAuthRecord{}, fmt.Errorf("load openai device authorization: %w", err)
	}
	if len(body) == 0 {
		return DeviceAuthRecord{}, ErrDeviceAuthNotFound
	}
	var record DeviceAuthRecord
	if err := json.Unmarshal(body, &record); err != nil {
		return DeviceAuthRecord{}, fmt.Errorf("%w: malformed record", ErrDeviceAuthInvalid)
	}
	if !validDeviceAuthRecord(record, flowID) {
		return DeviceAuthRecord{}, ErrDeviceAuthInvalid
	}
	now := s.now().UTC()
	if !now.Before(record.ExpiresAt.Add(deviceAuthExpiryGrace)) {
		return DeviceAuthRecord{}, ErrDeviceAuthNotFound
	}
	// Return the unmodified snapshot: lifecycle expiry needs PG arbitration.
	if !now.Before(record.ExpiresAt) && record.Status != DeviceAuthStatusSuccess {
		return record, ErrDeviceAuthExpired
	}
	return record, nil
}

func (s *DeviceStore) Save(ctx context.Context, record DeviceAuthRecord) error {
	if s == nil || s.cache == nil {
		return ErrDeviceAuthCacheUnavailable
	}
	if !validDeviceAuthRecord(record, record.FlowID) {
		return ErrDeviceAuthInvalid
	}
	if !s.now().UTC().Before(record.ExpiresAt) {
		return ErrDeviceAuthExpired
	}
	record.UpdatedAt = s.now().UTC()
	return s.saveWithTTL(ctx, record)
}

func (s *DeviceStore) SaveIfCurrent(ctx context.Context, expected, record DeviceAuthRecord) (bool, error) {
	return s.saveIfCurrent(ctx, expected, record, false)
}

// SaveIfCurrentForCredentialReconciliation permits a verified credential to
// repair an exchange even after its lease elapsed.
func (s *DeviceStore) SaveIfCurrentForCredentialReconciliation(ctx context.Context, expected, record DeviceAuthRecord) (bool, error) {
	if record.Status != DeviceAuthStatusSuccess || record.CredentialID == "" {
		return false, ErrDeviceAuthInvalid
	}
	return s.saveIfCurrent(ctx, expected, record, true)
}

func (s *DeviceStore) saveIfCurrent(ctx context.Context, expected, record DeviceAuthRecord, allowExpiredExchange bool) (bool, error) {
	if s == nil || s.cache == nil {
		return false, ErrDeviceAuthCacheUnavailable
	}
	if expected.FlowID == "" || record.FlowID != expected.FlowID || !validDeviceAuthRecord(expected, expected.FlowID) || !validDeviceAuthRecord(record, record.FlowID) {
		return false, ErrDeviceAuthInvalid
	}
	if record.OrgID != expected.OrgID || !record.OwnedBy(expected.SessionBinding) || !record.ExpiresAt.Equal(expected.ExpiresAt) {
		return false, ErrDeviceAuthInvalid
	}
	now := s.now().UTC()
	if !now.Before(record.ExpiresAt) && ((!allowExpiredExchange && record.Status != DeviceAuthStatusExpired) || !now.Before(record.ExpiresAt.Add(deviceAuthExpiryGrace))) {
		return false, ErrDeviceAuthExpired
	}
	comparer, ok := s.cache.(cache.CompareAndSetCache)
	if !ok {
		return false, ErrDeviceAuthCacheUnavailable
	}
	if exchangeLeaseExpired(now, expected) && !allowExpiredExchange && record.Status != DeviceAuthStatusExpired && (record.Status != DeviceAuthStatusFailed || record.ErrorCode != "exchange_interrupted") {
		return false, nil
	}
	expectedBody, err := json.Marshal(expected)
	if err != nil {
		return false, fmt.Errorf("marshal expected openai device authorization: %w", err)
	}
	record.UpdatedAt = now
	body, err := json.Marshal(record)
	if err != nil {
		return false, fmt.Errorf("marshal openai device authorization: %w", err)
	}
	ttl := record.ExpiresAt.Sub(now) + deviceAuthExpiryGrace
	// Redis CAS truncates to milliseconds; zero would retain the record forever.
	if ttl < time.Millisecond {
		return false, ErrDeviceAuthExpired
	}
	updated, err := comparer.CompareAndSet(ctx, DeviceAuthCacheKey(record.FlowID), expectedBody, body, ttl)
	if err != nil {
		return false, fmt.Errorf("compare-and-save openai device authorization: %w", err)
	}
	return updated, nil
}

// WithLock runs fn once when the cache backend can coordinate this flow.
// A false result means another replica is already polling the same flow.
func (s *DeviceStore) WithLock(ctx context.Context, flowID string, fn func() error) (bool, error) {
	if s == nil || s.cache == nil {
		return false, ErrDeviceAuthCacheUnavailable
	}
	locker, ok := s.cache.(cache.LockCache)
	if !ok {
		return false, ErrDeviceAuthCacheUnavailable
	}
	if flowID == "" || fn == nil {
		return false, ErrDeviceAuthInvalid
	}
	lockToken, err := openai.GenerateState()
	if err != nil {
		return false, fmt.Errorf("generate openai device lock token: %w", err)
	}
	acquired, err := locker.AcquireLock(ctx, DeviceAuthLockKey(flowID), lockToken, deviceAuthLockTTL)
	if err != nil || !acquired {
		return acquired, err
	}
	defer func() { _ = locker.ReleaseLock(context.Background(), DeviceAuthLockKey(flowID), lockToken) }()
	return true, fn()
}

func (s *DeviceStore) Coordinated() bool {
	if s == nil || s.cache == nil {
		return false
	}
	coordinated, ok := s.cache.(cache.SharedCoordinationCache)
	return ok && coordinated.SharedCoordinationAvailable()
}

func (s *DeviceStore) read(ctx context.Context, key string) ([]byte, error) {
	if shared, ok := s.cache.(cache.SharedCache); ok {
		return shared.GetShared(ctx, key)
	}
	return s.cache.Get(ctx, key)
}

func (s *DeviceStore) saveWithTTL(ctx context.Context, record DeviceAuthRecord) error {
	body, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal openai device authorization: %w", err)
	}
	ttl := record.ExpiresAt.Sub(s.now().UTC())
	if ttl <= 0 {
		return ErrDeviceAuthExpired
	}
	ttl += deviceAuthExpiryGrace
	if err := s.cache.Set(ctx, DeviceAuthCacheKey(record.FlowID), body, ttl); err != nil {
		return fmt.Errorf("store openai device authorization: %w", err)
	}
	return nil
}

func exchangeLeaseExpired(now time.Time, record DeviceAuthRecord) bool {
	return record.Status == DeviceAuthStatusExchanging && !now.Before(record.UpdatedAt.Add(deviceAuthLockTTL))
}

func validDeviceAuthRecord(record DeviceAuthRecord, flowID string) bool {
	base := record.FlowID != "" && record.FlowID == flowID && record.SessionBinding != "" &&
		record.IntervalSeconds > 0 && !record.CreatedAt.IsZero() &&
		!record.UpdatedAt.IsZero() && !record.ExpiresAt.IsZero() &&
		record.ExpiresAt.After(record.CreatedAt) && record.Status != ""
	if !base {
		return false
	}
	switch record.Status {
	case DeviceAuthStatusPending, DeviceAuthStatusExchanging, DeviceAuthStatusSuccess, DeviceAuthStatusFailed, DeviceAuthStatusExpired, DeviceAuthStatusDenied, DeviceAuthStatusCancelled:
	default:
		return false
	}
	if record.Terminal() {
		return record.Status != DeviceAuthStatusSuccess || record.CredentialID != ""
	}
	return record.DeviceAuthID != "" && record.UserCode != "" && hasRequiredDeviceAuthEndpoints(record)
}

func hasRequiredDeviceAuthEndpoints(record DeviceAuthRecord) bool {
	return strings.TrimSpace(record.TokenPollURL) != "" &&
		strings.TrimSpace(record.VerificationURI) != "" &&
		strings.TrimSpace(record.RedirectURI) != ""
}

const DeviceAuthRetryLimit = deviceAuthRetryLimit
