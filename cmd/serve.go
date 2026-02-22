package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

var specPath string

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the Bronto TUI (coming next)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if specPath == "" {
			return errors.New("missing required flag: --spec <path>")
		}
		fmt.Printf("serve: received --spec %q\n", specPath)
		fmt.Println("TUI renderer will be implemented in the next step.")
		return nil
	},
}

func init() {
	serveCmd.Flags().StringVar(&specPath, "spec", "", "Path to dashboard spec JSON (required)")
}