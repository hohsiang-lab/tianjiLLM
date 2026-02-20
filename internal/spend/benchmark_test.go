package spend

import (
	"testing"
	"time"
)

func BenchmarkGroupByColumn(b *testing.B) {
	cols := []string{"team", "tag", "model", "user", "key"}
	b.ResetTimer()
	for range b.N {
		for _, c := range cols {
			_, _ = groupByColumn(c)
		}
	}
}

func BenchmarkAnalyticsQueryDefaults(b *testing.B) {
	b.ResetTimer()
	for range b.N {
		_ = AnalyticsQuery{
			GroupBy:   "model",
			StartDate: time.Now().AddDate(0, -1, 0),
			EndDate:   time.Now(),
			TopN:      10,
		}
	}
}
