package dev

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/The-Pirateship/runtime/pkg/utils"
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

func runDev(cmd *cobra.Command, args []string) {
	// Parse config using shared utils
	parsedConfig := utils.ParseConfig("runtime.toml")
	if len(parsedConfig.Services) == 0 {
		fmt.Println("❌ No services found in runtime.toml")
		return
	}

	// Generate Zellij layout
	if err := generateZellijLayout(parsedConfig); err != nil {
		fmt.Printf("❌ Failed to generate Zellij layout: %v\n", err)
		return
	}

	// Generate Zellij config
	if err := generateZellijConfig(); err != nil {
		fmt.Printf("❌ Failed to generate Zellij config: %v\n", err)
		return
	}

	// Check if Zellij is installed
	if _, err := exec.LookPath("zellij"); err != nil {
		fmt.Println("❌ Zellij not found. Please install Zellij first:")
		fmt.Println("   brew install zellij")
		fmt.Println("   or visit: https://zellij.dev/documentation/installation")
		return
	}

	// Launch Zellij with the generated config and layout
	configPath := filepath.Join(".zellij", "config.kdl")
	layoutPath := filepath.Join(".zellij", "layout.kdl")
	fmt.Printf("🚀 Launching Zellij with config: %s and layout: %s\n", configPath, layoutPath)

	zellijCmd := exec.Command("zellij", "--config", configPath, "--layout", layoutPath)
	zellijCmd.Stdin = os.Stdin
	zellijCmd.Stdout = os.Stdout
	zellijCmd.Stderr = os.Stderr

	if err := zellijCmd.Run(); err != nil {
		fmt.Printf("❌ Failed to run Zellij: %v\n", err)
		return
	}
}
