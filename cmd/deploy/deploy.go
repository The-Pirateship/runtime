package deploy

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/The-Pirateship/runtime/pkg/gcpConnector"
	"github.com/The-Pirateship/runtime/pkg/ssh"
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

	// Parse config
	parsedConfig := utils.ParseConfig("runtime.toml")
	if len(parsedConfig.Services) == 0 {
		fmt.Println("❌ No services found in runtime.toml")
		return
	}

	// Validate services
	for _, service := range parsedConfig.Services {
		if service.RunsOn == "" {
			fmt.Printf("❌ Service '%s' is missing required 'runsOn' field for deployment\n", service.Name)
			fmt.Println("   Add 'runsOn = \"gcp.e2-micro\"' to each service in runtime.toml")
			return
		}

		if service.RunsOn != "gcp.e2-micro" {
			fmt.Printf("❌ Invalid runsOn value '%s' for service '%s'. Only 'gcp.e2-micro' is supported\n", service.RunsOn, service.Name)
			return
		}
	}

	fmt.Printf("🚀 Deploying %d service(s) to GCP...\n\n", len(parsedConfig.Services))

	// Validate project
	if err := gcpConnector.ValidateProject(ctx, parsedConfig.Name); err != nil {
		fmt.Printf("❌ %v\n", err)
		return
	}

	// Setup SSH keys
	fmt.Println("\n🔑 Setting up SSH access...")
	sshPublicKey, err := ssh.GetOrCreateSSHKey()
	if err != nil {
		fmt.Printf("❌ Failed to setup SSH: %v\n", err)
		return
	}

	// Get compute service
	fmt.Println("🔐 Authenticating with GCP...")
	computeService, err := gcpConnector.GetComputeService(ctx)
	if err != nil {
		fmt.Printf("❌ %v\n", err)
		return
	}
	fmt.Println("✅ Authenticated successfully\n")

	// Setup firewall rules
	if err := gcpConnector.EnsureFirewallRules(ctx, computeService, parsedConfig.Name); err != nil {
		fmt.Printf("❌ Failed to setup firewall: %v\n", err)
		return
	}

	// Deploy each service
	zone := "us-central1-a"

	for _, service := range parsedConfig.Services {
		fmt.Printf("📦 Deploying service: %s\n", service.Name)

		// Create instance
		instanceName := fmt.Sprintf("runtime-%s-%s", parsedConfig.Name, service.Name)
		instance, err := gcpConnector.CreateInstance(ctx, computeService, gcpConnector.InstanceConfig{
			Name:      instanceName,
			Zone:      zone,
			ProjectID: parsedConfig.Name,
			SSHKey:    sshPublicKey,
		})
		if err != nil {
			fmt.Printf("❌ Failed to create instance: %v\n", err)
			return
		}

		externalIP := gcpConnector.GetExternalIP(instance)
		fmt.Printf("   🌐 Instance IP: %s\n", externalIP)

		// Setup SSH client
		sshClient := &ssh.Client{
			Host: externalIP,
			User: "runtime",
		}

		// Wait for SSH to be ready
		if err := sshClient.WaitForSSH(2 * time.Minute); err != nil {
			fmt.Printf("❌ %v\n", err)
			return
		}

		// Upload code
		absPath, err := filepath.Abs(service.Path)
		if err != nil {
			fmt.Printf("❌ Failed to resolve path: %v\n", err)
			return
		}

		if err := sshClient.UploadDirectory(absPath, "/home/runtime/app"); err != nil {
			fmt.Printf("❌ Failed to upload code: %v\n", err)
			return
		}

		fmt.Printf("   ✅ %s deployed to instance\n\n", service.Name)
	}

	fmt.Println("🎉 All services deployed successfully!")
}
