package spend

import (
	"testing"
	"time"
)

func TestPgDate(t *testing.T) {
	now := time.Now()
	d := pgDate(now)
	if !d.Valid {
		t.Fatal("pgDate should be valid")
	}
	if !d.Time.Equal(now) {
		t.Fatalf("pgDate time = %v, want %v", d.Time, now)
	}
}

func TestPgTSTZ(t *testing.T) {
	now := time.Now()
	ts := pgTSTZ(now)
	if !ts.Valid {
		t.Fatal("pgTSTZ should be valid")
	}
	if !ts.Time.Equal(now) {
		t.Fatalf("pgTSTZ time = %v, want %v", ts.Time, now)
	}
}
