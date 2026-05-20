package virtuoso

import (
	"strings"
	"testing"
)

func TestIB015RegexExtractsFilename(t *testing.T) {
	msg := "*** Error 42000: [OpenLink][Virtuoso ODBC Driver][Virtuoso Server]" +
		"IB015: directory ./backup/kairo/ contains backup file " +
		"backup-202620#1.bp, backup aborted"
	got := ExtractIB015Filename(msg)
	if got != "backup-202620#1.bp" {
		t.Fatalf("got %q, want %q", got, "backup-202620#1.bp")
	}
}

func TestIB015RegexNoMatchOnOtherErrors(t *testing.T) {
	msg := "*** Error 37000: Function backup_online needs a string as argument 1"
	if got := ExtractIB015Filename(msg); got != "" {
		t.Fatalf("got %q, want empty", got)
	}
}

func TestSplitPathStripsTrailingSlashAndKeepsDirSlash(t *testing.T) {
	dir, name, err := splitPath("/opt/mydb/prod-202620#1.bp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dir != "/opt/mydb/" {
		t.Fatalf("dir = %q, want %q", dir, "/opt/mydb/")
	}
	if name != "prod-202620#1.bp" {
		t.Fatalf("name = %q, want %q", name, "prod-202620#1.bp")
	}
}

func TestSplitPathRejectsTrailingSlash(t *testing.T) {
	_, _, err := splitPath("/opt/mydb/")
	if err == nil || !strings.Contains(err.Error(), "filename") {
		t.Fatalf("expected filename error, got %v", err)
	}
}

func TestSplitPathRejectsNoSlash(t *testing.T) {
	_, _, err := splitPath("file.bp")
	if err == nil || !strings.Contains(err.Error(), "absolute") {
		t.Fatalf("expected absolute error, got %v", err)
	}
}

func TestSplitPathHandlesRootFile(t *testing.T) {
	dir, name, err := splitPath("/file.bp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dir != "/" || name != "file.bp" {
		t.Fatalf("got (%q, %q), want (%q, %q)", dir, name, "/", "file.bp")
	}
}

func TestValidateInlineRejectsQuote(t *testing.T) {
	if err := validateInline("path", "/tmp/has'quote"); err == nil {
		t.Fatal("expected error for quote, got nil")
	}
}

func TestValidateInlineRejectsBackslash(t *testing.T) {
	if err := validateInline("path", "/tmp/has\\backslash"); err == nil {
		t.Fatal("expected error for backslash, got nil")
	}
}

func TestValidateInlineAcceptsHash(t *testing.T) {
	if err := validateInline("path", "/opt/mydb/prod-202620#1.bp"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// Sample isql output captured from Virtuoso 07.20.3241; the row of underscores
// in the real output is 79 chars but the regex accepts 10+.
const sampleSingleValueOutput = `OpenLink Virtuoso Interactive SQL (Virtuoso)
Version 07.20.3241 as of May 26 2025
Type HELP; for help and EXIT; to exit.
Connected to OpenLink Virtuoso
Driver: 07.20.3241 OpenLink Virtuoso ODBC Driver
SQL>
LONG VARCHAR
_______________________________________________________________________________

y

1 Rows. -- 4 msec.
SQL> `

func TestParseSingleValueExtractsValueBetweenSeparatorAndRowCount(t *testing.T) {
	got, err := parseSingleValue(sampleSingleValueOutput)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "y" {
		t.Fatalf("got %q, want %q", got, "y")
	}
}

func TestParseSingleValueErrorsWhenNoSeparator(t *testing.T) {
	_, err := parseSingleValue("no underscores here\n1 Rows. -- 1 msec.\n")
	if err == nil || !strings.Contains(err.Error(), "separator") {
		t.Fatalf("expected separator error, got %v", err)
	}
}

func TestParseSingleValueErrorsWhenNoRowCount(t *testing.T) {
	_, err := parseSingleValue("____________\n\nvalue\n\n(no row count)")
	if err == nil || !strings.Contains(err.Error(), "row count") {
		t.Fatalf("expected row count error, got %v", err)
	}
}

func TestDetectErrorMatchesStarStarStarErrorMarker(t *testing.T) {
	out := "SQL> backup_online('x', 1, 1, 'y')\n" +
		"*** Error 42000: IB015: directory ./backup/ contains backup file backup-202620#1.bp, backup aborted\n" +
		"SQL> "
	err := detectError(out, nil)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "IB015") {
		t.Fatalf("error should carry IB015 text, got %v", err)
	}
}

func TestDetectErrorReturnsNilWhenOutputClean(t *testing.T) {
	if err := detectError("SQL> ok\nSQL> ", nil); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestDetectErrorPropagatesRunErrWhenNoSQLError(t *testing.T) {
	runErr := &runtimeError{msg: "exit status 1"}
	err := detectError("(some unrelated output)", runErr)
	if err == nil {
		t.Fatal("expected runErr to surface as error")
	}
	if !strings.Contains(err.Error(), "isql exited with error") {
		t.Fatalf("error should mention isql exit, got %v", err)
	}
}

type runtimeError struct{ msg string }

func (e *runtimeError) Error() string { return e.msg }
