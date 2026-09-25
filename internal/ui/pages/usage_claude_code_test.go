package pages

import "testing"

func TestCcCompositeScoreStyle_Boundaries(t *testing.T) {
	tests := []struct {
		name  string
		score float64
		want  string
	}{
		{"zero", 0.0, "color:#22c55e"},               // 0 < 1.0 → green
		{"just below green", 0.99, "color:#22c55e"},  // < 1.0 → green
		{"green boundary 1.0", 1.0, "color:#f97316"}, // 1.0 is not < 1.0 → orange
		{"mid orange", 2.0, "color:#f97316"},         // 1.0 ≤ 2.0 < 3.0 → orange
		{"just below red", 2.99, "color:#f97316"},    // < 3.0 → orange
		{"red boundary 3.0", 3.0, "color:#ef4444"},   // 3.0 is not < 3.0 → red
		{"high score", 10.0, "color:#ef4444"},        // 10.0 → red
	}
	for _, tc := range tests {
		v := tc.score
		got := ccCompositeScoreStyle(&v)
		if got != tc.want {
			t.Errorf("%s: ccCompositeScoreStyle(%.3f) = %q, want %q", tc.name, tc.score, got, tc.want)
		}
	}

	// nil pointer returns empty string
	if got := ccCompositeScoreStyle(nil); got != "" {
		t.Errorf("nil: ccCompositeScoreStyle(nil) = %q, want \"\"", got)
	}
}
