package cmd

import "github.com/spf13/cobra"

var incrementalArgs backupArgs

var incrementalCmd = &cobra.Command{
	Use:   "incremental",
	Short: "Run an incremental backup (no context clear).",
	Long: "Run backup_online() without clearing context. Call between fulls.\n\n" +
		"--require-full refuses to run if segment 1 of the current week's pattern is\n" +
		"not in target-dir. Catches missed-full scenarios (e.g. server outage skipped\n" +
		"the scheduler's `full` slot). Only verifies segment 1 exists; does NOT\n" +
		"validate that the prior full completed.",
	RunE: func(_ *cobra.Command, _ []string) error {
		return doBackup(&incrementalArgs, false)
	},
}

func init() {
	addBackupFlags(incrementalCmd, &incrementalArgs)
	incrementalCmd.Flags().BoolVar(&incrementalArgs.requireFull, "require-full",
		false,
		"Refuse to run if segment 1 of the current week's pattern is not in target-dir. "+
			"Catches missed-full scenarios. Does NOT validate prior-full completion.",
	)
}
