package cmd

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check local setup",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Bronto Doctor")
		fmt.Println("------------")
		fmt.Printf("OS:   %s/%s\n", runtime.GOOS, runtime.GOARCH)

		// Optional dependency checks (safe and useful for debugging installs)
		if _, err := exec.LookPath("git"); err != nil {
			fmt.Println("git:  NOT FOUND (optional)")
		} else {
			fmt.Println("git:  OK")
		}

		fmt.Println("✅ Done.")
		return nil
	},
}