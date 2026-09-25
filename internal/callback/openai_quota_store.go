package callback

import (
	"math"
	"time"
)

type OpenAIQuotaStatus string

const (
	OpenAIQuotaStatusUnknown   OpenAIQuotaStatus = ""
	OpenAIQuotaStatusAllowed   OpenAIQuotaStatus = "allowed"
	OpenAIQuotaStatusRejected  OpenAIQuotaStatus = "rejected"
	OpenAIQuotaStatusExhausted OpenAIQuotaStatus = "exhausted"
)

type OpenAIQuotaDimension struct {
	Limit            int
	LimitKnown       bool
	Remaining        int
	RemainingKnown   bool
	ResetAt          time.Time
	ResetKnown       bool
	Utilization      float64
	UtilizationKnown bool
}

type OpenAIQuotaState struct {
	SubjectID string
	Status    OpenAIQuotaStatus

	Requests OpenAIQuotaDimension
	Tokens   OpenAIQuotaDimension

	QuotaResetAt          time.Time
	QuotaResetKnown       bool
	QuotaUtilization      float64
	QuotaUtilizationKnown bool

	UpdatedAt time.Time
}

func (s OpenAIQuotaState) Gated(now time.Time) bool {
	if s.Status == OpenAIQuotaStatusRejected && s.QuotaResetKnown && s.QuotaResetAt.After(now) {
		return true
	}
	if dimensionGated(s.Requests, now) || dimensionGated(s.Tokens, now) {
		return true
	}
	return false
}

func (s OpenAIQuotaState) Normalize(now time.Time) OpenAIQuotaState {
	s.Requests = normalizeOpenAIQuotaDimension(s.Requests, now)
	s.Tokens = normalizeOpenAIQuotaDimension(s.Tokens, now)
	if s.QuotaResetKnown && !s.QuotaResetAt.After(now) {
		s.Status = OpenAIQuotaStatusUnknown
		s.QuotaResetAt = time.Time{}
		s.QuotaResetKnown = false
	}
	return s
}

func MergeOpenAIQuotaState(existing, incoming OpenAIQuotaState) OpenAIQuotaState {
	if incoming.SubjectID == "" {
		incoming.SubjectID = existing.SubjectID
	}
	if incoming.Status == OpenAIQuotaStatusUnknown {
		incoming.Status = existing.Status
	}
	incoming.Requests = mergeOpenAIQuotaDimension(existing.Requests, incoming.Requests)
	incoming.Tokens = mergeOpenAIQuotaDimension(existing.Tokens, incoming.Tokens)
	if !incoming.QuotaResetKnown && existing.QuotaResetKnown {
		incoming.QuotaResetAt = existing.QuotaResetAt
		incoming.QuotaResetKnown = true
	}
	if !incoming.QuotaUtilizationKnown && existing.QuotaUtilizationKnown {
		incoming.QuotaUtilization = existing.QuotaUtilization
		incoming.QuotaUtilizationKnown = true
	}
	if incoming.UpdatedAt.IsZero() {
		incoming.UpdatedAt = existing.UpdatedAt
	}
	return incoming
}

func DeriveOpenAIQuotaUtilization(limit, remaining int) (float64, bool) {
	if limit <= 0 || remaining < 0 {
		return 0, false
	}
	return ClampOpenAIQuotaUtilization(float64(limit-remaining) / float64(limit)), true
}

func ClampOpenAIQuotaUtilization(v float64) float64 {
	if math.IsNaN(v) {
		return 0
	}
	return math.Max(0, math.Min(1, v))
}

func dimensionGated(d OpenAIQuotaDimension, now time.Time) bool {
	return d.RemainingKnown && d.Remaining == 0 && d.ResetKnown && d.ResetAt.After(now)
}

func normalizeOpenAIQuotaDimension(d OpenAIQuotaDimension, now time.Time) OpenAIQuotaDimension {
	if d.ResetKnown && !d.ResetAt.After(now) {
		d.ResetAt = time.Time{}
		d.ResetKnown = false
		if d.RemainingKnown && d.Remaining == 0 {
			d.RemainingKnown = false
		}
		if d.UtilizationKnown && d.Utilization == 1 {
			d.UtilizationKnown = false
			d.Utilization = 0
		}
	}
	return d
}

func mergeOpenAIQuotaDimension(existing, incoming OpenAIQuotaDimension) OpenAIQuotaDimension {
	if !incoming.LimitKnown && existing.LimitKnown {
		incoming.Limit = existing.Limit
		incoming.LimitKnown = true
	}
	if !incoming.RemainingKnown && existing.RemainingKnown {
		incoming.Remaining = existing.Remaining
		incoming.RemainingKnown = true
	}
	if !incoming.ResetKnown && existing.ResetKnown {
		incoming.ResetAt = existing.ResetAt
		incoming.ResetKnown = true
	}
	if !incoming.UtilizationKnown && existing.UtilizationKnown {
		incoming.Utilization = existing.Utilization
		incoming.UtilizationKnown = true
	}
	return incoming
}
