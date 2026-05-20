package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/kairoaraujo/vt-backup/internal/timeutil"
	"github.com/kairoaraujo/vt-backup/internal/virtuoso"
	"github.com/spf13/cobra"
)

// Shared backup flags. Populated by addBackupFlags() on each subcommand.
type backupArgs struct {
	// connection
	host         string
	port         int
	user         string
	password     string
	passwordFile string
	isqlBin      string
	timeoutSecs  int

	// backup
	prefix       string
	week         string
	segmentBytes int64
	targetDir    string
	yes          bool

	// incremental-only
	requireFull bool
}

const maxOverwriteRetries = 50

const installHint = `OpenLink's isql binary was not found in any well-known install location.

  Pass --isql /absolute/path/to/isql, or install Virtuoso:

  Linux (RHEL/Rocky):  install OpenLink Virtuoso; isql ships in its bin dir.
  macOS (Homebrew):    brew install virtuoso  (isql at $(brew --prefix virtuoso)/bin/isql)

Note: unixODBC also ships a binary called 'isql'. We deliberately do NOT use
/usr/bin/isql — it requires a DSN registration and speaks differently. Always
point --isql at OpenLink's binary, typically next to virtodbc_r.so.`

// Search paths for OpenLink's isql. Order matters — first match wins.
// /usr/bin/isql is deliberately excluded: that's unixODBC's isql.
var isqlSearchPaths = []string{
	// Typical Linux Virtuoso installs
	"/usr/local/virtuoso-opensource/bin/isql",
	// macOS Homebrew
	"/opt/homebrew/opt/virtuoso/bin/isql",
	"/usr/local/opt/virtuoso/bin/isql",
}

func resolvePassword(a *backupArgs) (string, error) {
	if a.password != "" && a.passwordFile != "" {
		return "", Usage(errors.New("pass either --password or --password-file, not both"))
	}
	if a.passwordFile != "" {
		if a.passwordFile == "-" {
			reader := bufio.NewReader(os.Stdin)
			line, err := reader.ReadString('\n')
			if err != nil && !errors.Is(err, io.EOF) {
				return "", Usage(fmt.Errorf("reading password from stdin: %w", err))
			}
			return strings.TrimRight(line, "\n"), nil
		}
		b, err := os.ReadFile(a.passwordFile)
		if err != nil {
			return "", Usage(fmt.Errorf("reading password file: %w", err))
		}
		return strings.TrimRight(string(b), "\n"), nil
	}
	if a.password != "" {
		return a.password, nil
	}
	return "", Usage(errors.New("missing password: pass --password or --password-file"))
}

func resolveISQL(a *backupArgs) error {
	if a.isqlBin != "" {
		info, err := os.Stat(a.isqlBin)
		if err != nil {
			return Usage(fmt.Errorf("isql binary not found: %s", a.isqlBin))
		}
		if info.IsDir() {
			return Usage(fmt.Errorf("--isql points at a directory: %s", a.isqlBin))
		}
		slog.Info("using isql binary", "path", a.isqlBin, "source", "--isql")
		return nil
	}
	for _, candidate := range isqlSearchPaths {
		info, err := os.Stat(candidate)
		if err != nil || info.IsDir() {
			continue
		}
		// Best-effort executability check; on macOS Homebrew the bit is set,
		// on some package installs it might not be, but Stat success + non-dir
		// is good enough — exec.Command will surface the real error otherwise.
		if info.Mode()&0o111 != 0 {
			slog.Info("using isql binary", "path", candidate, "source", "auto-detect")
			a.isqlBin = candidate
			return nil
		}
	}
	return Usage(errors.New("no OpenLink isql binary found; pass --isql\n\n" + installHint))
}

func pattern(a *backupArgs) string {
	week := a.week
	if week == "" {
		week = timeutil.Current()
	}
	return fmt.Sprintf("%s-%s#", a.prefix, week)
}

func segmentOnePath(a *backupArgs) string {
	dir := strings.TrimRight(a.targetDir, "/")
	return fmt.Sprintf("%s/%s1.bp", dir, pattern(a))
}

func doBackup(a *backupArgs, full bool) error {
	password, err := resolvePassword(a)
	if err != nil {
		return err
	}
	if err := resolveISQL(a); err != nil {
		return err
	}

	client := &virtuoso.Client{
		Binary:   a.isqlBin,
		Host:     a.host,
		Port:     a.port,
		User:     a.user,
		Password: password,
	}

	kind := "incremental"
	if full {
		kind = "full"
	}
	slog.Info("starting backup",
		"kind", kind,
		"pattern", pattern(a),
		"target_dir", a.targetDir,
		"segment_bytes", a.segmentBytes,
	)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(a.timeoutSecs)*time.Second)
	defer cancel()

	if !full && a.requireFull {
		seg := segmentOnePath(a)
		ok, err := client.FileExists(ctx, seg)
		if err != nil {
			return Fail(fmt.Errorf("--require-full check failed: %w", err))
		}
		if !ok {
			return Usage(fmt.Errorf(
				"--require-full: segment 1 not found at %s. A full backup for this week must run before incrementals can chain. Run `vt-backup full -y ...` first, then re-run incremental.",
				seg,
			))
		}
		slog.Info("--require-full check passed", "path", seg)
	}

	if full {
		if err := client.BackupContextClear(ctx); err != nil {
			return Fail(fmt.Errorf("backup_context_clear() failed: %w", err))
		}
		slog.Info("backup_context_clear() ok")
		if err := runBackupWithOverwrite(ctx, a, client); err != nil {
			return err
		}
	} else {
		slog.Info("running backup_online()")
		if err := client.BackupOnline(ctx, pattern(a), a.segmentBytes, a.targetDir); err != nil {
			return Fail(fmt.Errorf("incremental backup failed: %w", err))
		}
	}
	slog.Info("backup ok", "kind", kind)
	return nil
}

// runBackupWithOverwrite handles Virtuoso's IB015 collision: when a leftover
// segment file from a previous (failed) run blocks a fresh full, delete it
// and retry. Bounded at maxOverwriteRetries.
func runBackupWithOverwrite(ctx context.Context, a *backupArgs, client *virtuoso.Client) error {
	targetClean := strings.TrimRight(a.targetDir, "/")
	for attempt := 0; attempt < maxOverwriteRetries; attempt++ {
		slog.Info("running backup_online()", "attempt", attempt+1)
		err := client.BackupOnline(ctx, pattern(a), a.segmentBytes, a.targetDir)
		if err == nil {
			return nil
		}
		filename := virtuoso.ExtractIB015Filename(err.Error())
		if filename == "" {
			return Fail(fmt.Errorf("backup failed: %w", err))
		}
		path := targetClean + "/" + filename
		slog.Warn("collision detected", "path", path)

		switch {
		case a.yes:
			slog.Info("-y given: deleting and retrying", "path", path)
		case confirmTTY(fmt.Sprintf("Overwrite %s?", path)):
			slog.Info("user confirmed: deleting and retrying", "path", path)
		default:
			if isStdinTTY() {
				return Usage(errors.New("aborted by user; backup not run"))
			}
			return Usage(fmt.Errorf("pre-existing segment %s and stdin is not a TTY. Pass -y to allow overwriting.", path))
		}
		if err := client.DeleteFile(ctx, path); err != nil {
			return Fail(fmt.Errorf("failed to delete %s: %w", path, err))
		}
	}
	return Fail(fmt.Errorf("too many collision retries (%d); giving up", maxOverwriteRetries))
}

func isStdinTTY() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func confirmTTY(message string) bool {
	if !isStdinTTY() {
		return false
	}
	fmt.Fprintf(os.Stderr, "%s [y/N] ", message)
	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "y" || answer == "yes"
}

// addBackupFlags wires the shared full/incremental flag set onto cmd.
func addBackupFlags(cmd *cobra.Command, a *backupArgs) {
	f := cmd.Flags()
	f.StringVar(&a.host, "host", "127.0.0.1", "Virtuoso host")
	f.IntVar(&a.port, "port", 1111, "Virtuoso ODBC port")
	f.StringVar(&a.user, "user", "dba", "Virtuoso user")
	f.StringVar(&a.password, "password", "", "DBA password (visible in `ps`; prefer --password-file)")
	f.StringVar(&a.passwordFile, "password-file", "", "Read password from FILE (use \"-\" for stdin). Recommended for cron.")
	f.StringVar(&a.isqlBin, "isql", "", "Absolute path to OpenLink's isql binary. Default: auto-detect from common install paths.")
	f.IntVar(&a.timeoutSecs, "timeout", 14400, "Overall isql call timeout in seconds")
	f.StringVar(&a.prefix, "prefix", "backup", "Backup name prefix; final name is \"<prefix>-YYYYWW#\"")
	f.StringVar(&a.week, "week", "", "Override ISO year+week (e.g. 202519). Default: current.")
	f.Int64Var(&a.segmentBytes, "segment-bytes", 10_000_000, "Per-segment cap, bytes")
	f.StringVar(&a.targetDir, "target-dir", "backup", "Virtuoso-side BackupDir name (from virtuoso.ini)")
	f.BoolVarP(&a.yes, "yes", "y", false, "Assume yes for destructive prompts. Required for non-TTY runs with leftover segments.")
}
