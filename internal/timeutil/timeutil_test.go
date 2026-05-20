package timeutil

import (
	"testing"
	"time"
)

func TestIsoYearWeekFormat(t *testing.T) {
	got := IsoYearWeek(time.Date(2026, 5, 13, 0, 0, 0, 0, time.UTC))
	if got != "202620" {
		t.Fatalf("got %q, want %q", got, "202620")
	}
}

func TestIsoYearWeekYearBoundary(t *testing.T) {
	// 2027-01-01 is a Friday -> ISO week 53 of 2026, NOT week 01 of 2027.
	got := IsoYearWeek(time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC))
	if got != "202653" {
		t.Fatalf("got %q, want %q", got, "202653")
	}
}

func TestIsoYearWeekPadded(t *testing.T) {
	got := IsoYearWeek(time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC))
	if got != "202602" {
		t.Fatalf("got %q, want %q", got, "202602")
	}
}
