package strategy

import (
	"testing"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/router"
)

func TestUsageBasedPickEmpty(t *testing.T) {
	u := NewUsageBased(time.Minute)
	if u.Pick(nil) != nil {
		t.Fatal("nil for empty")
	}
}

func TestUsageBasedDefaultWindow(t *testing.T) {
	u := NewUsageBased(0)
	if u.window != time.Minute {
		t.Fatalf("default window = %v", u.window)
	}
}

func TestUsageBasedPick(t *testing.T) {
	u := NewUsageBased(time.Minute)
	rpm1 := int64(100)
	rpm2 := int64(100)

	deployments := []*router.Deployment{
		{ID: "d1", Config: &config.ModelConfig{TianjiParams: config.TianjiParams{RPM: &rpm1}}},
		{ID: "d2", Config: &config.ModelConfig{TianjiParams: config.TianjiParams{RPM: &rpm2}}},
	}

	// Record heavy usage on d1
	u.RecordUsage("d1", 5000)
	u.RecordUsage("d1", 5000)

	picked := u.Pick(deployments)
	if picked == nil {
		t.Fatal("nil pick")
	}
	// d2 should be preferred (less usage)
	if picked.ID != "d2" {
		t.Fatalf("expected d2, got %s", picked.ID)
	}
}

func TestUsageRatio(t *testing.T) {
	rpm := int64(60)
	tpm := int64(100000)
	d := &router.Deployment{Config: &config.ModelConfig{TianjiParams: config.TianjiParams{RPM: &rpm, TPM: &tpm}}}
	ratio := usageRatio(d, 30, 50000)
	if ratio != 0.5 {
		t.Fatalf("ratio = %f, want 0.5", ratio)
	}
}
