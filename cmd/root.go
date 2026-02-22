package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	Version = "dev"    // overridden at build time
	Commit  = "none"   // overridden at build time
	Date    = "unknown" // overridden at build time
)

var rootCmd = &cobra.Command{
	Use:   "bronto",
	Short: "Bronto CLI",
	Long:  "Bronto CLI is a command-line tool for running Bronto terminal dashboards.",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(serveCmd)
}