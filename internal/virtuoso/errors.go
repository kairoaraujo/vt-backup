package virtuoso

import "regexp"

// IB015FileRe captures the offending filename from a Virtuoso IB015 collision
// error message, e.g.:
//
//	IB015: directory ./backup/ contains backup file backup-202620#1.bp, backup aborted
var IB015FileRe = regexp.MustCompile(`contains backup file (\S+),`)

// ExtractIB015Filename returns the offending filename if msg matches IB015,
// otherwise the empty string.
func ExtractIB015Filename(msg string) string {
	m := IB015FileRe.FindStringSubmatch(msg)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}
