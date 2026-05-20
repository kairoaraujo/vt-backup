// Package cmd implements the Cobra-based CLI surface.
//
// Subcommands mirror the Python tool one-to-one:
//
//	full          Clear backup context, then run backup_online().
//	incremental   Run backup_online() without clearing context.
//	restore-cmd   Print the offline restore command to stdout.
//
// Exit codes mirror the Python tool's contract:
//
//	0  success
//	1  backup failed (ODBC / Virtuoso error)
//	2  usage error, missing driver, missing password, or precondition unmet
package cmd

import (
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Version is set at link time via -ldflags "-X github.com/.../cmd.Version=...".
var Version = "dev"

var verbose int

// ExitCodeError signals a specific process exit code. Returned by RunE
// callbacks; Execute() unwraps it and calls os.Exit accordingly.
type ExitCodeError struct {
	Code int
	Err  error
}

func (e *ExitCodeError) Error() string {
	if e.Err == nil {
		return ""
	}
	return e.Err.Error()
}

func (e *ExitCodeError) Unwrap() error { return e.Err }

// Usage marks an error as exit code 2 (usage / precondition).
func Usage(err error) error { return &ExitCodeError{Code: 2, Err: err} }

// Fail marks an error as exit code 1 (operation failed).
func Fail(err error) error { return &ExitCodeError{Code: 1, Err: err} }

var rootCmd = &cobra.Command{
	Use:           "vt-backup",
	Short:         "CLI for Virtuoso backups (port 1111).",
	Long:          "CLI for Virtuoso backups (port 1111).\n\nThe scheduler (cron, systemd timer) decides when to run `full` vs `incremental`.\nThis tool does not maintain state and does not decide for you.",
	Version:       Version,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRun: func(_ *cobra.Command, _ []string) {
		configureLogging(verbose)
	},
}

// Execute runs the root command and exits with the appropriate code.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		var ece *ExitCodeError
		if errors.As(err, &ece) {
			if ece.Err != nil {
				slog.Error(ece.Err.Error())
			}
			os.Exit(ece.Code)
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().CountVarP(&verbose, "verbose", "v", "Increase log verbosity (repeatable: -v info, -vv debug)")
	rootCmd.AddCommand(fullCmd, incrementalCmd, restoreCmd)
}

func initConfig() {
	// Viper is wired in for future env var or config-file support. The Python
	// tool's design is "flags only", so we mirror that for now: no AutomaticEnv,
	// no config-file lookup. All values come from the CLI.
	viper.SetEnvPrefix("VT_BACKUP")
}

func configureLogging(verbose int) {
	level := slog.LevelWarn
	switch {
	case verbose >= 2:
		level = slog.LevelDebug
	case verbose == 1:
		level = slog.LevelInfo
	}
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	slog.SetDefault(slog.New(handler))
}
