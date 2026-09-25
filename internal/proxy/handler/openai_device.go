package handler

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/openaioauth"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/openai"
)

const (
	deviceAuthRequestTimeout  = 20 * time.Second
	deviceAuthStateTimeout    = 5 * time.Second
	deviceAuthExchangeLease   = 2 * time.Minute
	deviceAuthMaxBackoff      = time.Minute
	deviceAuthSlowDownSeconds = 5
)

var ErrOpenAIConnectDisabled = errors.New("OpenAI Connect is disabled")

var ErrOpenAIDeviceAuthExchangeInProgress = errors.New("openai device authorization exchange is in progress")
var ErrOpenAIDeviceAuthPersistenceUnavailable = errors.New("openai device authorization persistence unavailable")
var ErrOpenAIDeviceAuthSessionRequired = errors.New("openai device authorization session binding required")

type OpenAIDeviceAuthStart struct {
	FlowID          string
	OrganizationID  string
	VerificationURI string
	UserCode        string
	ExpiresAt       time.Time
	IntervalSeconds int64
}

type OpenAIDeviceAuthStatus struct {
	FlowID          string
	OrganizationID  string
	Status          openaioauth.DeviceAuthStatus
	VerificationURI string
	UserCode        string
	ExpiresAt       time.Time
	NextPollAt      time.Time
	IntervalSeconds int64
	CredentialID    string
	ErrorCode       string
}

func (h *Handlers) StartOpenAIDeviceAuth(ctx context.Context, orgID, sessionBinding string) (OpenAIDeviceAuthStart, error) {
	if sessionBinding == "" {
		return OpenAIDeviceAuthStart{}, ErrOpenAIDeviceAuthSessionRequired
	}
	if h == nil || h.Config == nil {
		return OpenAIDeviceAuthStart{}, errors.New("OpenAI OAuth is not configured")
	}
	if !h.openAIOAuthConfig().Enabled {
		return OpenAIDeviceAuthStart{}, ErrOpenAIConnectDisabled
	}
	if err := h.withDeviceAuthTx(ctx, "", func(context.Context, *db.Queries) error { return nil }); err != nil {
		return OpenAIDeviceAuthStart{}, ErrOpenAIDeviceAuthPersistenceUnavailable
	}
	store := openaioauth.NewDeviceStore(h.Cache)
	if !store.Coordinated() {
		return OpenAIDeviceAuthStart{}, openaioauth.ErrDeviceAuthCacheUnavailable
	}
	cfg := h.openAIOAuthConfig()
	requestCtx, cancel := context.WithTimeout(ctx, deviceAuthRequestTimeout)
	defer cancel()
	device, err := openai.RequestDeviceCode(requestCtx, h.openAIOAuthHTTPClient(), cfg)
	if err != nil {
		return OpenAIDeviceAuthStart{}, err
	}
	if device.Interval >= openaioauth.DefaultDeviceAuthTTL {
		return OpenAIDeviceAuthStart{}, openai.ErrDeviceAuthProvider
	}
	intervalSeconds := int64(device.Interval / time.Second)
	if intervalSeconds <= 0 {
		intervalSeconds = 5
	}
	record, err := store.Create(requestCtx, openaioauth.DeviceAuthRecord{
		DeviceAuthID:    device.DeviceAuthID,
		UserCode:        device.UserCode,
		TokenPollURL:    device.TokenPollURL,
		VerificationURI: device.VerificationURI,
		RedirectURI:     device.RedirectURI,
		OrgID:           orgID,
		SessionBinding:  sessionBinding,
		IntervalSeconds: intervalSeconds,
	}, openaioauth.DefaultDeviceAuthTTL)
	if err != nil {
		return OpenAIDeviceAuthStart{}, err
	}
	return OpenAIDeviceAuthStart{
		FlowID:          record.FlowID,
		OrganizationID:  record.OrgID,
		VerificationURI: record.VerificationURI,
		UserCode:        record.UserCode,
		ExpiresAt:       record.ExpiresAt,
		IntervalSeconds: record.IntervalSeconds,
	}, nil
}

func (h *Handlers) PollOpenAIDeviceAuth(ctx context.Context, flowID, sessionBinding string) (OpenAIDeviceAuthStatus, error) {
	if sessionBinding == "" {
		return OpenAIDeviceAuthStatus{}, ErrOpenAIDeviceAuthSessionRequired
	}
	store := openaioauth.NewDeviceStore(h.Cache)
	record, err := store.Get(ctx, flowID)
	if err != nil && !errors.Is(err, openaioauth.ErrDeviceAuthExpired) {
		return OpenAIDeviceAuthStatus{}, err
	}
	if !record.OwnedBy(sessionBinding) {
		return OpenAIDeviceAuthStatus{}, openaioauth.ErrDeviceAuthOwnership
	}
	if record.Terminal() || record.Status == openaioauth.DeviceAuthStatusExchanging || errors.Is(err, openaioauth.ErrDeviceAuthExpired) {
		return h.reconcileStaleDeviceExchange(ctx, store, record)
	}
	if !h.openAIOAuthConfig().Enabled || time.Now().UTC().Before(record.NextPollAt) {
		return deviceAuthStatusFromRecord(record), nil
	}
	var result OpenAIDeviceAuthStatus
	acquired, err := store.WithLock(ctx, flowID, func() error {
		next := record
		next.Status, next.ErrorCode = openaioauth.DeviceAuthStatusExchanging, ""
		applied, stateErr := h.saveDeviceAuthStateIfCurrent(ctx, store, record, &next)
		if stateErr != nil {
			return stateErr
		}
		if !applied || next.Status != openaioauth.DeviceAuthStatusExchanging {
			result = deviceAuthStatusFromRecord(next)
			return nil
		}
		result, stateErr = h.pollAndCompleteDeviceAuth(ctx, store, next)
		return stateErr
	})
	if err != nil {
		return OpenAIDeviceAuthStatus{}, err
	}
	if acquired {
		return result, nil
	}
	// Busy paths are observers, never unconditional cache writers.
	return h.reconcileStaleDeviceExchange(ctx, store, record)
}

func (h *Handlers) pollAndCompleteDeviceAuth(ctx context.Context, store *openaioauth.DeviceStore, record openaioauth.DeviceAuthRecord) (OpenAIDeviceAuthStatus, error) {
	pollCtx, cancel := context.WithTimeout(ctx, deviceAuthRequestTimeout)
	defer cancel()
	pollResult, err := openai.PollDeviceCodeAt(pollCtx, h.openAIOAuthHTTPClient(), record.TokenPollURL, openai.DeviceCode{
		DeviceAuthID: record.DeviceAuthID,
		UserCode:     record.UserCode,
	})
	if err != nil {
		if stateErr := h.applyDevicePollError(ctx, store, &record, err); stateErr != nil {
			return OpenAIDeviceAuthStatus{}, stateErr
		}
		return deviceAuthStatusFromRecord(record), nil
	}
	if openai.ChallengeFromVerifier(pollResult.CodeVerifier) != pollResult.CodeChallenge {
		if stateErr := h.failDeviceAuth(ctx, store, &record, "provider_error"); stateErr != nil {
			return OpenAIDeviceAuthStatus{}, stateErr
		}
		return deviceAuthStatusFromRecord(record), nil
	}

	expected := record
	record.Status = openaioauth.DeviceAuthStatusExchanging
	if applied, saveErr := h.saveDeviceAuthStateIfCurrent(ctx, store, expected, &record); saveErr != nil {
		return OpenAIDeviceAuthStatus{}, saveErr
	} else if !applied || record.Status != openaioauth.DeviceAuthStatusExchanging {
		return deviceAuthStatusFromRecord(record), nil
	}
	exchangeCtx, cancel := context.WithTimeout(ctx, deviceAuthRequestTimeout)
	defer cancel()
	tokenBundle, err := openai.ExchangeCode(exchangeCtx, h.openAIOAuthHTTPClient(), h.openAIOAuthConfig(), pollResult.AuthorizationCode, record.RedirectURI, pollResult.CodeVerifier)
	if err != nil {
		if failErr := h.failDeviceAuth(ctx, store, &record, "exchange_failed"); failErr != nil {
			return OpenAIDeviceAuthStatus{}, failErr
		}
		return deviceAuthStatusFromRecord(record), nil
	}

	now := time.Now().UTC()
	credentialBundle := openAISubscriptionTokenBundle(tokenBundle, now)
	if credentialBundle.AccountID == "" {
		if failErr := h.failDeviceAuth(ctx, store, &record, "missing_account_id"); failErr != nil {
			return OpenAIDeviceAuthStatus{}, failErr
		}
		return deviceAuthStatusFromRecord(record), nil
	}
	credentialInfo := openAISubscriptionCredentialInfo(tokenBundle, now)
	credential, err := h.SaveOpenAISubscriptionCredentialForFlow(ctx, &record, "OpenAI Subscription", credentialBundle, credentialInfo)
	if err != nil {
		if failErr := h.failDeviceAuth(ctx, store, &record, "save_failed"); failErr != nil {
			return OpenAIDeviceAuthStatus{}, failErr
		}
		return deviceAuthStatusFromRecord(record), nil
	}

	expected = record
	record.Status = openaioauth.DeviceAuthStatusSuccess
	record.CredentialID = credential.CredentialID
	record.ErrorCode = ""
	clearDeviceAuthProviderState(&record)
	if _, err := h.saveDeviceAuthStateIfCurrent(ctx, store, expected, &record); err != nil {
		return OpenAIDeviceAuthStatus{}, err
	}
	return deviceAuthStatusFromRecord(record), nil
}

func deviceAuthPersistenceContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), deviceAuthStateTimeout)
}

// Redis suppresses provider polling; only this short PG transaction arbitrates
// admission and lifecycle changes. Never hold it over provider HTTP work.
func (h *Handlers) withDeviceAuthTx(ctx context.Context, flowID string, fn func(context.Context, *db.Queries) error) error {
	if h == nil || h.DB == nil {
		return ErrOpenAIDeviceAuthPersistenceUnavailable
	}
	beginner, ok := h.DB.(interface {
		BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error)
	})
	if !ok {
		return ErrOpenAIDeviceAuthPersistenceUnavailable
	}
	txCtx, cancel := deviceAuthPersistenceContext(ctx)
	defer cancel()
	tx, err := beginner.BeginTx(txCtx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return err
	}
	if tx == nil {
		return ErrOpenAIDeviceAuthPersistenceUnavailable
	}
	defer func() {
		// BeginTx's context does not roll back an abandoned transaction.
		cleanup, cancel := deviceAuthPersistenceContext(ctx)
		defer cancel()
		_ = tx.Rollback(cleanup)
	}()
	if flowID != "" {
		if _, err := tx.Exec(txCtx, "SELECT pg_advisory_xact_lock(hashtextextended($1, 0))", "tianji/openai/device/"+flowID); err != nil {
			return err
		}
	}
	if err := fn(txCtx, db.New(tx)); err != nil {
		return err
	}
	return tx.Commit(txCtx)
}

func sameDeviceAuthIdentity(a, b openaioauth.DeviceAuthRecord) bool {
	return a.FlowID == b.FlowID && a.OwnedBy(b.SessionBinding) && a.OrgID == b.OrgID && a.ExpiresAt.Equal(b.ExpiresAt)
}

// Call only after obtaining the flow lock: ReadCommitted's next statement must
// see the preceding writer's COMMIT, not a snapshot taken before lock contention.
func deviceAuthCredential(ctx context.Context, q *db.Queries, record openaioauth.DeviceAuthRecord) (db.CredentialTable, bool, error) {
	id := openAIDeviceCredentialID(record.FlowID)
	credential, err := q.GetCredential(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return db.CredentialTable{}, false, nil
	}
	if err != nil {
		return db.CredentialTable{}, false, err
	}
	if credential.CredentialID != id || credential.CredentialType != CredentialTypeOpenAISubscription ||
		(record.OrgID == "" && credential.OrganizationID != nil) || (record.OrgID != "" && (credential.OrganizationID == nil || *credential.OrganizationID != record.OrgID)) {
		return db.CredentialTable{}, false, openaioauth.ErrDeviceAuthOwnership
	}
	return credential, true, nil
}

func (h *Handlers) saveDeviceAuthStateIfCurrent(ctx context.Context, store *openaioauth.DeviceStore, expected openaioauth.DeviceAuthRecord, record *openaioauth.DeviceAuthRecord) (bool, error) {
	current, applied, err := h.transitionDeviceAuth(ctx, store, expected, record)
	if err == nil {
		*record = current
	}
	return applied, err
}

// desired == nil is an observer's fresh reconciliation/recovery decision.
// Workers retain their original generation; a reread never grants ownership.
func (h *Handlers) transitionDeviceAuth(ctx context.Context, store *openaioauth.DeviceStore, expected openaioauth.DeviceAuthRecord, desired *openaioauth.DeviceAuthRecord) (openaioauth.DeviceAuthRecord, bool, error) {
	var current openaioauth.DeviceAuthRecord
	applied := false
	err := h.withDeviceAuthTx(ctx, expected.FlowID, func(ctx context.Context, q *db.Queries) error {
		var err error
		current, err = store.Get(ctx, expected.FlowID)
		if err != nil && !errors.Is(err, openaioauth.ErrDeviceAuthExpired) {
			return err
		}
		if !sameDeviceAuthIdentity(current, expected) {
			return openaioauth.ErrDeviceAuthOwnership
		}
		if desired != nil && (!sameDeviceAuthIdentity(*desired, expected) || desired.Generation != expected.Generation) {
			return openaioauth.ErrDeviceAuthOwnership
		}
		if desired != nil && (current.Terminal() || current.Generation != expected.Generation) {
			return nil
		}
		credential, found, err := deviceAuthCredential(ctx, q, current)
		if err != nil {
			return err
		}
		next := current
		now := time.Now().UTC()
		switch {
		case found:
			if !strings.HasPrefix(current.Generation, "admitted:") {
				return openaioauth.ErrDeviceAuthInvalid
			}
			if current.Status == openaioauth.DeviceAuthStatusSuccess && current.CredentialID == credential.CredentialID {
				return nil
			}
			next.Status, next.CredentialID, next.ErrorCode = openaioauth.DeviceAuthStatusSuccess, credential.CredentialID, ""
		case current.Terminal():
			return nil
		case !now.Before(current.ExpiresAt):
			next.Status, next.ErrorCode = openaioauth.DeviceAuthStatusExpired, "expired"
		case current.Generation == "" || (current.Status == openaioauth.DeviceAuthStatusExchanging && !now.Before(current.UpdatedAt.Add(deviceAuthExchangeLease))):
			next.Status, next.ErrorCode = openaioauth.DeviceAuthStatusFailed, "exchange_interrupted"
		case desired == nil:
			return nil
		case desired.Status == openaioauth.DeviceAuthStatusCancelled && current.Status == openaioauth.DeviceAuthStatusExchanging:
			return ErrOpenAIDeviceAuthExchangeInProgress
		default:
			if current.Status != expected.Status {
				return nil
			}
			next = *desired
			if current.Status == openaioauth.DeviceAuthStatusPending && next.Status == openaioauth.DeviceAuthStatusExchanging {
				if !h.openAIOAuthConfig().Enabled || now.Before(current.NextPollAt) {
					return nil
				}
				next.Generation = "attempt:" + uuid.NewString()
			} else if current.Status == openaioauth.DeviceAuthStatusExchanging && !next.Terminal() && !strings.HasPrefix(current.Generation, "attempt:") {
				return nil
			}
		}
		if next.Terminal() {
			clearDeviceAuthProviderState(&next)
		}
		if found {
			applied, err = store.SaveIfCurrentForCredentialReconciliation(ctx, current, next)
		} else {
			applied, err = store.SaveIfCurrent(ctx, current, next)
		}
		if err != nil {
			return err
		}
		if !applied {
			return openaioauth.ErrDeviceAuthOwnership
		}
		current = next
		return nil
	})
	if err == nil && applied && current.Terminal() && current.Status != openaioauth.DeviceAuthStatusCancelled {
		status := "failure"
		if current.Status == openaioauth.DeviceAuthStatusSuccess {
			status = "success"
		}
		h.auditOpenAISubscriptionLifecycle(ctx, callback.OpenAISubscriptionAttribution{CredentialID: current.CredentialID, Provider: "openai", OrganizationID: current.OrgID, Action: "connect", Status: status, ReasonCode: current.ErrorCode})
	}
	return current, applied, err
}

func (h *Handlers) applyDevicePollError(ctx context.Context, store *openaioauth.DeviceStore, record *openaioauth.DeviceAuthRecord, err error) error {
	expected := *record
	now := time.Now().UTC()
	switch {
	case errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded):
		record.Status = openaioauth.DeviceAuthStatusFailed
		record.ErrorCode = "exchange_failed"
	case errors.Is(err, openai.ErrDeviceAuthPending):
		record.Status = openaioauth.DeviceAuthStatusPending
		record.ErrorCode = ""
		record.NextPollAt = now.Add(time.Duration(record.IntervalSeconds) * time.Second)
	case errors.Is(err, openai.ErrDeviceAuthSlowDown):
		record.Status = openaioauth.DeviceAuthStatusPending
		record.ErrorCode = "slow_down"
		record.IntervalSeconds += deviceAuthSlowDownSeconds
		record.NextPollAt = now.Add(time.Duration(record.IntervalSeconds) * time.Second)
	case errors.Is(err, openai.ErrDeviceAuthExpired):
		record.Status = openaioauth.DeviceAuthStatusExpired
		record.ErrorCode = "expired_token"
	case errors.Is(err, openai.ErrDeviceAuthDenied):
		record.Status = openaioauth.DeviceAuthStatusDenied
		record.ErrorCode = "access_denied"
	case errors.Is(err, openai.ErrDeviceAuthTemporary):
		record.RetryCount++
		if record.RetryCount >= openaioauth.DeviceAuthRetryLimit {
			record.Status = openaioauth.DeviceAuthStatusFailed
			record.ErrorCode = "provider_unavailable"
		} else {
			record.Status = openaioauth.DeviceAuthStatusPending
			record.ErrorCode = "temporarily_unavailable"
			record.NextPollAt = now.Add(deviceAuthBackoff(record.IntervalSeconds, record.RetryCount))
		}
	default:
		record.Status = openaioauth.DeviceAuthStatusFailed
		record.ErrorCode = "provider_error"
	}
	_, saveErr := h.saveDeviceAuthStateIfCurrent(ctx, store, expected, record)
	return saveErr
}

func clearDeviceAuthProviderState(record *openaioauth.DeviceAuthRecord) {
	record.DeviceAuthID = ""
	record.UserCode = ""
	record.TokenPollURL = ""
	record.VerificationURI = ""
	record.RedirectURI = ""
}

func (h *Handlers) failDeviceAuth(ctx context.Context, store *openaioauth.DeviceStore, record *openaioauth.DeviceAuthRecord, errorCode string) error {
	expected := *record
	record.Status = openaioauth.DeviceAuthStatusFailed
	record.ErrorCode = errorCode
	_, err := h.saveDeviceAuthStateIfCurrent(ctx, store, expected, record)
	return err
}

func openAIDeviceCredentialID(flowID string) string {
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte("tianji/openai/device/"+flowID)).String()
}

func (h *Handlers) reconcileStaleDeviceExchange(ctx context.Context, store *openaioauth.DeviceStore, record openaioauth.DeviceAuthRecord) (OpenAIDeviceAuthStatus, error) {
	current, _, err := h.transitionDeviceAuth(ctx, store, record, nil)
	if err != nil {
		return OpenAIDeviceAuthStatus{}, err
	}
	return deviceAuthStatusFromRecord(current), nil
}

func (h *Handlers) CancelOpenAIDeviceAuth(ctx context.Context, flowID, sessionBinding string) error {
	if sessionBinding == "" {
		return ErrOpenAIDeviceAuthSessionRequired
	}
	store := openaioauth.NewDeviceStore(h.Cache)
	record, err := store.Get(ctx, flowID)
	if errors.Is(err, openaioauth.ErrDeviceAuthNotFound) {
		return nil
	}
	if err != nil && !errors.Is(err, openaioauth.ErrDeviceAuthExpired) {
		return err
	}
	if !record.OwnedBy(sessionBinding) {
		return openaioauth.ErrDeviceAuthOwnership
	}
	if errors.Is(err, openaioauth.ErrDeviceAuthExpired) {
		_, reconcileErr := h.reconcileStaleDeviceExchange(ctx, store, record)
		return reconcileErr
	}
	if record.Status == openaioauth.DeviceAuthStatusExchanging {
		return ErrOpenAIDeviceAuthExchangeInProgress
	}
	if record.Terminal() {
		return nil
	}
	acquired, err := store.WithLock(ctx, flowID, func() error {
		next := record
		next.Status, next.ErrorCode = openaioauth.DeviceAuthStatusCancelled, "cancelled"
		_, stateErr := h.saveDeviceAuthStateIfCurrent(ctx, store, record, &next)
		if stateErr == nil && !next.Terminal() {
			return ErrOpenAIDeviceAuthExchangeInProgress
		}
		return stateErr
	})
	if err != nil {
		return err
	}
	if !acquired {
		status, err := h.reconcileStaleDeviceExchange(ctx, store, record)
		if err != nil {
			return err
		}
		if status.Status == openaioauth.DeviceAuthStatusExchanging || status.Status == openaioauth.DeviceAuthStatusPending {
			return ErrOpenAIDeviceAuthExchangeInProgress
		}
	}
	return nil
}

func deviceAuthStatusFromRecord(record openaioauth.DeviceAuthRecord) OpenAIDeviceAuthStatus {
	return OpenAIDeviceAuthStatus{
		FlowID:          record.FlowID,
		OrganizationID:  record.OrgID,
		Status:          record.Status,
		VerificationURI: record.VerificationURI,
		UserCode:        record.UserCode,
		ExpiresAt:       record.ExpiresAt,
		NextPollAt:      record.NextPollAt,
		IntervalSeconds: record.IntervalSeconds,
		CredentialID:    record.CredentialID,
		ErrorCode:       record.ErrorCode,
	}
}

func deviceAuthBackoff(intervalSeconds int64, retryCount int) time.Duration {
	minimum := time.Duration(intervalSeconds) * time.Second
	backoff := minimum
	for i := 1; i < retryCount; i++ {
		if backoff >= deviceAuthMaxBackoff/2 {
			return max(minimum, deviceAuthMaxBackoff)
		}
		backoff *= 2
	}
	return max(minimum, min(backoff, deviceAuthMaxBackoff))
}
