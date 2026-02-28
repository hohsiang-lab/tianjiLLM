package scheduler

import (
	"testing"
)

func TestJobNames(t *testing.T) {
	tests := []struct {
		job  Job
		want string
	}{
		{&BudgetResetJob{}, "budget_reset"},
		{&SpendLogCleanupJob{}, "spend_log_cleanup"},
		{&PolicyHotReloadJob{}, "policy_hot_reload"},
		{&SpendArchivalJob{}, "spend_archival"},
		{&SpendBatchWriteJob{}, "spend_batch_write"},
		{&CredentialRefreshJob{}, "credential_refresh"},
		{&KeyRotationJob{}, "key_rotation"},
		{&HealthCheckJob{}, "health_check"},
	}
	for _, tt := range tests {
		if got := tt.job.Name(); got != tt.want {
			t.Errorf("%T.Name() = %q, want %q", tt.job, got, tt.want)
		}
	}
}
