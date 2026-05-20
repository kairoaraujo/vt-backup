package cmd

import "github.com/spf13/cobra"

var fullArgs backupArgs

var fullCmd = &cobra.Command{
	Use:   "full",
	Short: "Clear context and run a full online backup.",
	Long: "Run backup_context_clear() then backup_online(). Call once per backup cycle.\n\n" +
		"`-y` is required for non-TTY runs (cron/systemd) when the target dir already\n" +
		"holds segments for the same week: those will be wiped before the new backup.",
	RunE: func(_ *cobra.Command, _ []string) error {
		return doBackup(&fullArgs, true)
	},
}

func init() {
	addBackupFlags(fullCmd, &fullArgs)
}
