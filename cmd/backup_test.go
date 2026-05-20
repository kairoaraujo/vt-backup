package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolvePasswordFromFile(t *testing.T) {
	dir := t.TempDir()
	pwFile := filepath.Join(dir, "pw")
	if err := os.WriteFile(pwFile, []byte("s3cret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	a := &backupArgs{passwordFile: pwFile}
	got, err := resolvePassword(a)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "s3cret" {
		t.Fatalf("got %q, want %q (trailing newline must be stripped)", got, "s3cret")
	}
}

func TestResolvePasswordRejectsBoth(t *testing.T) {
	a := &backupArgs{password: "x", passwordFile: "/tmp/p"}
	_, err := resolvePassword(a)
	var ece *ExitCodeError
	if !errors.As(err, &ece) || ece.Code != 2 {
		t.Fatalf("expected exit-2 ExitCodeError, got %v", err)
	}
}

func TestResolvePasswordMissingExitsTwo(t *testing.T) {
	a := &backupArgs{}
	_, err := resolvePassword(a)
	var ece *ExitCodeError
	if !errors.As(err, &ece) || ece.Code != 2 {
		t.Fatalf("expected exit-2 ExitCodeError, got %v", err)
	}
}

func TestResolveISQLWithExplicitPathErrorsWhenMissing(t *testing.T) {
	a := &backupArgs{isqlBin: "/nonexistent/path/isql"}
	err := resolveISQL(a)
	var ece *ExitCodeError
	if !errors.As(err, &ece) || ece.Code != 2 {
		t.Fatalf("expected exit-2 ExitCodeError, got %v", err)
	}
}

func TestResolveISQLWithExplicitPathAccepted(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "isql")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	a := &backupArgs{isqlBin: bin}
	if err := resolveISQL(a); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.isqlBin != bin {
		t.Fatalf("isqlBin mutated unexpectedly: %q", a.isqlBin)
	}
}

func TestResolveISQLRejectsDirectory(t *testing.T) {
	dir := t.TempDir()
	a := &backupArgs{isqlBin: dir}
	err := resolveISQL(a)
	var ece *ExitCodeError
	if !errors.As(err, &ece) || ece.Code != 2 {
		t.Fatalf("expected exit-2 ExitCodeError, got %v", err)
	}
}

func TestPatternUsesArgsWeekWhenSet(t *testing.T) {
	a := &backupArgs{prefix: "prod", week: "202519"}
	if got := pattern(a); got != "prod-202519#" {
		t.Fatalf("got %q, want %q", got, "prod-202519#")
	}
}

func TestPatternFallsBackToCurrentISOWeek(t *testing.T) {
	a := &backupArgs{prefix: "prod"}
	got := pattern(a)
	if !strings.HasPrefix(got, "prod-") || !strings.HasSuffix(got, "#") {
		t.Fatalf("got %q, want shape prod-YYYYWW#", got)
	}
	if len(got) != len("prod-")+6+1 {
		t.Fatalf("got %q, expected length matching prod-YYYYWW#", got)
	}
}

func TestSegmentOnePathStripsTrailingSlash(t *testing.T) {
	a := &backupArgs{prefix: "prod", week: "202519", targetDir: "/opt/mydb/"}
	if got := segmentOnePath(a); got != "/opt/mydb/prod-202519#1.bp" {
		t.Fatalf("got %q", got)
	}
}

func TestSegmentOnePathNoTrailingSlash(t *testing.T) {
	a := &backupArgs{prefix: "prod", week: "202519", targetDir: "/opt/mydb"}
	if got := segmentOnePath(a); got != "/opt/mydb/prod-202519#1.bp" {
		t.Fatalf("got %q", got)
	}
}
