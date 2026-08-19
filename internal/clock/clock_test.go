package clock

import "testing"

func TestFixedClock(t *testing.T) {
	c := NewFixed(1000)
	if c.NowMillis() != 1000 {
		t.Fatalf("expected 1000, got %d", c.NowMillis())
	}
	c.Advance(500)
	if c.NowMillis() != 1500 {
		t.Fatalf("expected 1500, got %d", c.NowMillis())
	}
}

func TestRealClockPositive(t *testing.T) {
	var c RealClock
	if c.NowMillis() <= 0 {
		t.Fatal("real clock should be positive")
	}
}
