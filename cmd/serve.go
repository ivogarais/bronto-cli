package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/ivogarais/bronto-cli/spec"
	"github.com/ivogarais/bronto-cli/tui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var specPath string

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the Bronto TUI",
	RunE: func(cmd *cobra.Command, args []string) error {
		if specPath == "" {
			return errors.New("missing required flag: --spec <path>")
		}

		s, err := spec.Load(specPath)
		if err != nil {
			return err
		}

		m := tui.NewModel(s, specPath)
		p := tea.NewProgram(m, tea.WithAltScreen())

		if _, err := p.Run(); err != nil {
			fmt.Fprintln(os.Stderr, "failed to start TUI:", err)
			return err
		}
		return nil
	},
}

func init() {
	serveCmd.Flags().StringVar(&specPath, "spec", "", "Path to dashboard spec JSON (required)")
}
