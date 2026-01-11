package deploy

import (
	"context"
	"fmt"

	"github.com/The-Pirateship/runtime/pkg/gcpConnector"
	"github.com/The-Pirateship/runtime/pkg/utils"
	"github.com/spf13/cobra"
)

func RegisterCommand(rootCmd *cobra.Command) {
	deployCmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy your project to the cloud",
		Run:   runDeploy,
	}

	rootCmd.AddCommand(deployCmd)
}

func runDeploy(cmd *cobra.Command, args []string) {
	ctx := context.Background()

	// Parse config using shared utils
	parsedConfig := utils.ParseConfig("runtime.toml")
	if len(parsedConfig.Services) == 0 {
		fmt.Println("❌ No services found in runtime.toml")
		return
	}

	fmt.Println("config is", parsedConfig)

	// Validate that all services have runsOn field for deployment
	for _, service := range parsedConfig.Services {
		if service.RunsOn == "" {
			fmt.Printf("❌ Service '%s' is missing required 'runsOn' field for deployment\n", service.Name)
			fmt.Println("   Add 'runsOn = \"gcp.e2-micro\"' to each service in runtime.toml")
			return
		}

		// Validate runsOn value
		if service.RunsOn != "gcp.e2-micro" {
			fmt.Printf("❌ Invalid runsOn value '%s' for service '%s'. Only 'gcp.e2-micro' is supported\n", service.RunsOn, service.Name)
			return
		}
	}

	fmt.Printf("✅ All %d services are configured for deployment\n", len(parsedConfig.Services))
	fmt.Println("🚀 Starting deployment process...")

	if err := gcpConnector.ValidateProject(ctx, parsedConfig.Name); err != nil {
		fmt.Printf("❌ %v\n", err)
		return
	}

	// TODO: Add actual deployment logic here
	for _, service := range parsedConfig.Services {
		fmt.Printf("   📦 Deploying %s to %s\n", service.Name, service.RunsOn)
	}

	fmt.Println("✅ Deployment completed successfully!")
}
