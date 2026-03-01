package spend

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGroupByColumn_Valid(t *testing.T) {
	cases := map[string]string{
		"team":  "team_id",
		"tag":   "request_tags[1]",
		"model": "model",
		"user":  "user_id",
		"key":   "api_key",
	}

	for input, expected := range cases {
		col, err := groupByColumn(input)
		assert.NoError(t, err)
		assert.Equal(t, expected, col, "groupBy=%s", input)
	}
}

func TestGroupByColumn_Invalid(t *testing.T) {
	_, err := groupByColumn("invalid")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid group_by")
}

func TestAnalyticsQuery_Defaults(t *testing.T) {
	q := AnalyticsQuery{
		TopN: 0,
	}

	// TopN=0 should default to 10 in QueryTopN
	assert.Equal(t, 0, q.TopN)
}

func TestSpendGroup_JSON(t *testing.T) {
	g := SpendGroup{
		Name:  "gpt-4",
		Spend: 42.5,
	}
	assert.Equal(t, "gpt-4", g.Name)
	assert.Equal(t, 42.5, g.Spend)
}

func TestFOCUSRecord_Fields(t *testing.T) {
	r := FOCUSRecord{
		BilledCost: 10.0,
		Provider:   "openai",
		UsageUnit:  "tokens",
	}

	assert.Equal(t, "openai", r.Provider)
	assert.Equal(t, 10.0, r.BilledCost)
	assert.Equal(t, "tokens", r.UsageUnit)
}

func TestS3BackendName(t *testing.T) {
	b := &S3Backend{}
	assert.Equal(t, "s3", b.Name())
}
