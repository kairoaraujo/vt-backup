package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

var (
	restorePrefix    string
	restoreWeek      string
	restoreBackupDir string
)

var restoreCmd = &cobra.Command{
	Use:   "restore-cmd",
	Short: "Print the offline restore command to stdout. Does not execute.",
	Long: "Emits the `virtuoso-t +restore-backup ... +backup-dirs ...` command line\n" +
		"matching --prefix and --week. Stop Virtuoso and run that command manually,\n" +
		"then start Virtuoso back up.",
	RunE: func(_ *cobra.Command, _ []string) error {
		if restoreWeek == "" {
			return Usage(errors.New("--week is required for restore-cmd (e.g. --week 202519)"))
		}
		pattern := fmt.Sprintf("%s-%s#", restorePrefix, restoreWeek)
		fmt.Printf("virtuoso-t +restore-backup %s +backup-dirs %s\n", pattern, restoreBackupDir)
		return nil
	},
}

func init() {
	restoreCmd.Flags().StringVar(&restorePrefix, "prefix", "backup", "Backup name prefix")
	restoreCmd.Flags().StringVar(&restoreWeek, "week", "", "ISO year+week, e.g. 202519 (required)")
	restoreCmd.Flags().StringVar(&restoreBackupDir, "backup-dir", "backup", "Value passed to virtuoso-t +backup-dirs")
}
