package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	version   = "dev"
	buildDate = "unknown"
	gitCommit = "unknown"
)

// SetBuildInfo sets version info injected at build time.
func SetBuildInfo(v, date, commit string) {
	version = v
	buildDate = date
	gitCommit = commit
}

var rootCmd = &cobra.Command{
	Use:   "highclaw",
	Short: "HighClaw — Personal AI Assistant Gateway",
	Long: `🦀 HighClaw — Personal AI Assistant Gateway

Multi-channel AI gateway with extensible messaging integrations.
Run your personal AI assistant on your own devices.

Distributed as a single static binary — no Node.js, no npm, just run it.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("highclaw %s\n", version)
		fmt.Printf("  build:  %s\n", buildDate)
		fmt.Printf("  commit: %s\n", gitCommit)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(gatewayCmd)
	rootCmd.AddCommand(agentCmd)
	rootCmd.AddCommand(channelsCmd)
	rootCmd.AddCommand(configCmdGroup)
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(onboardCmd) // Primary onboard wizard

	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(sessionsCmd)
	rootCmd.AddCommand(cronCmd)
	rootCmd.AddCommand(skillsCmd)
	rootCmd.AddCommand(pluginsCmd)
	rootCmd.AddCommand(nodesCmd)
	rootCmd.AddCommand(devicesCmd)
	rootCmd.AddCommand(modelsCmd)
	rootCmd.AddCommand(securityCmd)
	rootCmd.AddCommand(hooksCmd)
	rootCmd.AddCommand(logsCmd)
	rootCmd.AddCommand(tasksCmd)
	rootCmd.AddCommand(browserCmdGroup)
	rootCmd.AddCommand(memoryCmd)
	rootCmd.AddCommand(daemonCmd)
	rootCmd.AddCommand(serviceCmd)
	rootCmd.AddCommand(integrationsCmd)
	rootCmd.AddCommand(migrateCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(tuiCmd)
	rootCmd.AddCommand(resetCmd)
	rootCmd.AddCommand(uninstallCmd)
	rootCmd.AddCommand(messageCmd)
	rootCmd.AddCommand(webhooksCmd)
	rootCmd.AddCommand(pairingCmd)
	rootCmd.AddCommand(dnsCmd)
	rootCmd.AddCommand(docsCmd)
	rootCmd.AddCommand(dashboardCmd)
	rootCmd.AddCommand(execApprovalsCmd)
}

// Execute runs the root cobra command.
func Execute() error {
	return rootCmd.Execute()
}
