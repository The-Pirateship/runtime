package cmd

import (
	"os"

	"github.com/The-Pirateship/runtime/cmd/dev"
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "rt",
	Short: "The one place to run your code",
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// RegisterAllCommands registers all available commands with the root command
func RegisterAllCommands(rootCmd *cobra.Command) {
	dev.RegisterCommand(rootCmd)
}

func init() {
	// Register all commands using the centralized registry
	RegisterAllCommands(rootCmd)

	// Hide default completion command
	rootCmd.CompletionOptions.HiddenDefaultCmd = true
}
