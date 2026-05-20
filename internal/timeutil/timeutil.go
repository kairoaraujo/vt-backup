// Package timeutil derives the ISO year+week string used as the backup suffix.
package timeutil

import (
	"fmt"
	"time"
)

// IsoYearWeek returns "YYYYWW" for the ISO week containing t.
// Example: t=2026-05-13 -> "202620". The ISO year is not always the calendar
// year (e.g. 2027-01-01 is ISO week 53 of 2026).
func IsoYearWeek(t time.Time) string {
	year, week := t.ISOWeek()
	return fmt.Sprintf("%d%02d", year, week)
}

// Now is overridable in tests.
var Now = func() time.Time { return time.Now().UTC() }

// Current returns the current ISO year+week.
func Current() string {
	return IsoYearWeek(Now())
}
