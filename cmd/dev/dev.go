package dev

import (
	"github.com/spf13/cobra"
)

func RegisterCommand(rootCmd *cobra.Command) {
	runCmd := &cobra.Command{
		Use:   "dev",
		Short: "Run your project locally",
		Run:   runDev,
	}
	
	rootCmd.AddCommand(runCmd)
}
