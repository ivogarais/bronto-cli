package cmd

import (
	"errors"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/ivogarais/bronto-cli/spec"
	"github.com/ivogarais/bronto-cli/tui"
	"github.com/spf13/cobra"
)

var specPath string
var refreshMS = 5000

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the Bronto TUI",
	RunE: func(cmd *cobra.Command, args []string) error {
		if specPath == "" {
			return errors.New("missing required flag: --spec <path>")
		}

		s, err := spec.LoadStrict(specPath)
		if err != nil {
			return err
		}

		m := tui.NewModel(s, specPath, refreshMS)
		p := tea.NewProgram(m)

		if _, err := p.Run(); err != nil {
			fmt.Fprintln(os.Stderr, "failed to start TUI:", err)
			return err
		}
		return nil
	},
}

func init() {
	serveCmd.Flags().StringVar(&specPath, "spec", "", "Path to dashboard spec JSON (required)")
	serveCmd.Flags().IntVar(
		&refreshMS,
		"refresh-ms",
		5000,
		"Advanced: auto-reload interval in milliseconds (default: 5000)",
	)
	_ = serveCmd.Flags().MarkHidden("refresh-ms")
}
