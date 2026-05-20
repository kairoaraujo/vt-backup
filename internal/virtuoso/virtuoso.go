// Package virtuoso drives Virtuoso via OpenLink's bundled isql CLI as a
// subprocess.
//
// We invoke isql per call, feed the SQL on stdin, and parse the text output.
// This trades a typed database/sql interface for one os/exec round-trip, but
// it removes:
//   - the CGO + unixODBC build dependency,
//   - the alexbrainman/odbc driver-binding maintenance risk,
//   - the SQL_ATTR_AUTOCOMMIT gap that makes db.Close() block after long
//     procedures like backup_online().
//
// The transport (port 1111) is identical to what isql would use directly.
package virtuoso

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// Client runs SQL through isql against a single Virtuoso server.
// Cheap to construct; one per CLI invocation.
type Client struct {
	Binary   string // path to OpenLink's isql binary
	Host     string
	Port     int
	User     string
	Password string
}

// run invokes isql with the given SQL on stdin. Returns combined stdout+stderr
// and the process error. We always append EXIT; so isql terminates cleanly.
func (c *Client) run(ctx context.Context, sqlScript string) (string, error) {
	cmd := exec.CommandContext(ctx, c.Binary,
		fmt.Sprintf("%s:%d", c.Host, c.Port),
		c.User,
		c.Password,
	)
	script := strings.TrimRight(sqlScript, "\n ")
	if !strings.HasSuffix(script, ";") {
		script += ";"
	}
	script += "\nEXIT;\n"
	cmd.Stdin = strings.NewReader(script)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()
	return stdout.String() + stderr.String(), runErr
}

// errPatternRe matches isql's SQL error lines, e.g.:
//
//	*** Error 42000: [OpenLink][Virtuoso ODBC Driver][Virtuoso Server]IB015: ...
var errPatternRe = regexp.MustCompile(`(?m)^\*\*\* Error[^\n]*`)

// detectError translates a (output, runErr) pair into nil-or-error. isql does
// not always exit non-zero on a SQL error, so we scan output for the "*** Error"
// marker regardless of runErr. runErr alone (no SQL error in output) still
// counts: that's an isql crash or connectivity failure.
func detectError(output string, runErr error) error {
	if m := errPatternRe.FindString(output); m != "" {
		return errors.New(strings.TrimSpace(m))
	}
	if runErr != nil {
		return fmt.Errorf("isql exited with error: %w (output: %s)", runErr, strings.TrimSpace(output))
	}
	return nil
}

// Exec runs SQL that returns no row data (a procedure call).
func (c *Client) Exec(ctx context.Context, sql string) error {
	out, runErr := c.run(ctx, sql)
	return detectError(out, runErr)
}

// QueryString runs a SELECT that returns a single string column, single row.
// Returns the trimmed value cell.
func (c *Client) QueryString(ctx context.Context, sql string) (string, error) {
	out, runErr := c.run(ctx, sql)
	if err := detectError(out, runErr); err != nil {
		return "", err
	}
	return parseSingleValue(out)
}

var (
	separatorRe = regexp.MustCompile(`(?m)^_{10,}$`)
	rowCountRe  = regexp.MustCompile(`(?m)^\d+ Rows?\.`)
)

// parseSingleValue extracts the value cell from isql's text-table output.
//
// Output shape (the relevant bit):
//
//	<column type, e.g. LONG VARCHAR>
//	_______________________________________________________________________________
//
//	<value>
//
//	1 Rows. -- M msec.
func parseSingleValue(output string) (string, error) {
	sepLoc := separatorRe.FindStringIndex(output)
	if sepLoc == nil {
		return "", fmt.Errorf("isql output: no separator line found")
	}
	rest := output[sepLoc[1]:]
	rowCountLoc := rowCountRe.FindStringIndex(rest)
	if rowCountLoc == nil {
		return "", fmt.Errorf("isql output: no row count line found after separator")
	}
	return strings.TrimSpace(rest[:rowCountLoc[0]]), nil
}

// BackupContextClear resets the server's incremental-backup checkpoint.
// Called exactly once at the start of a full backup.
func (c *Client) BackupContextClear(ctx context.Context) error {
	return c.Exec(ctx, "backup_context_clear()")
}

// BackupOnline triggers Virtuoso's online backup with the given segment-name
// pattern, segment size cap, and target directory.
//
// Values come from CLI flags, not external input; we inline them and refuse
// single-quote / backslash characters.
func (c *Client) BackupOnline(ctx context.Context, pattern string, segmentBytes int64, targetDir string) error {
	if err := validateInline("pattern", pattern); err != nil {
		return err
	}
	if err := validateInline("target-dir", targetDir); err != nil {
		return err
	}
	stmt := fmt.Sprintf("backup_online('%s', %d, 1, '%s')", pattern, segmentBytes, targetDir)
	return c.Exec(ctx, stmt)
}

// DeleteFile removes a single file on the server via Virtuoso's file_delete().
// The path must be inside one of the directories listed in DirsAllowed.
func (c *Client) DeleteFile(ctx context.Context, path string) error {
	if err := validateInline("path", path); err != nil {
		return err
	}
	return c.Exec(ctx, fmt.Sprintf("file_delete('%s')", path))
}

// FileExists returns true if path exists in Virtuoso's filesystem.
//
// Implementation note: we use sys_dirlist()+position() rather than file_stat()
// because at least one Virtuoso build (07.20.3241) returns the file's mtime
// from file_stat(p, 0) instead of the byte size promised by the docs, which
// breaks numeric checks. The result is encoded as 'y'/'n' so parsing isql's
// text output is trivial. The parent directory must be listed in DirsAllowed.
func (c *Client) FileExists(ctx context.Context, path string) (bool, error) {
	if err := validateInline("path", path); err != nil {
		return false, err
	}
	directory, filename, err := splitPath(path)
	if err != nil {
		return false, err
	}
	stmt := fmt.Sprintf(
		"SELECT CASE WHEN position('%s', sys_dirlist('%s', 1)) > 0 THEN 'y' ELSE 'n' END",
		filename, directory,
	)
	result, err := c.QueryString(ctx, stmt)
	if err != nil {
		return false, err
	}
	return result == "y", nil
}

// splitPath returns (directory-with-trailing-slash, filename) for an absolute
// path with a non-empty filename component.
func splitPath(path string) (string, string, error) {
	idx := strings.LastIndex(path, "/")
	if idx == -1 {
		return "", "", fmt.Errorf("path must be absolute with a filename: %q", path)
	}
	filename := path[idx+1:]
	if filename == "" {
		return "", "", fmt.Errorf("path must include a filename: %q", path)
	}
	directory := path[:idx]
	if directory == "" {
		directory = "/"
	}
	if !strings.HasSuffix(directory, "/") {
		directory += "/"
	}
	return directory, filename, nil
}

func validateInline(label, value string) error {
	if strings.ContainsAny(value, "'\\") {
		return fmt.Errorf("invalid character in %s: %q", label, value)
	}
	return nil
}
