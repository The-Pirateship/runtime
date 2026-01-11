package gcpConnector

// gcpConnector package handles GCP deployment operations
import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"google.golang.org/api/cloudresourcemanager/v1"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/option"
)

// GetComputeService creates an authenticated GCP Compute client
func GetComputeService(ctx context.Context) (*compute.Service, error) {
	service, err := compute.NewService(ctx, option.WithScopes(
		compute.ComputeScope,
	))
	if err != nil {
		return nil, fmt.Errorf("failed to create GCP compute service: %w\n\nMake sure you've run: gcloud auth application-default login", err)
	}

	return service, nil
}

// ValidateProject checks if the project exists and user has access
func ValidateProject(ctx context.Context, projectID string) error {
	fmt.Printf("🔍 Validating GCP project '%s'...\n", projectID)
	
	// First check if user is authenticated
	if !isAuthenticated() {
		return fmt.Errorf("not authenticated with GCP\n\nPlease run:\n  gcloud auth application-default login")
	}

	// Create resource manager service to check projects
	service, err := cloudresourcemanager.NewService(ctx)
	if err != nil {
		return fmt.Errorf("failed to create resource manager service: %w", err)
	}

	// Try to get the project
	project, err := service.Projects.Get(projectID).Context(ctx).Do()
	if err != nil {
		// Project doesn't exist or user doesn't have access
		return fmt.Errorf("project '%s' not found or you don't have access\n\nAvailable options:\n"+
			"1. Create project: https://console.cloud.google.com/projectcreate\n"+
			"2. Update runtime.toml with an existing project name\n"+
			"3. Run 'gcloud projects list' to see your projects", projectID)
	}

	// Check if project is empty or has no ProjectId (means it doesn't exist)
	if project == nil || project.ProjectId == "" {
		return fmt.Errorf("project '%s' not found\n\nAvailable options:\n"+
			"1. Create project: https://console.cloud.google.com/projectcreate\n"+
			"2. Update runtime.toml with an existing project name\n"+
			"3. Run 'gcloud projects list' to see your projects", projectID)
	}

	// Check if project is active
	if project.LifecycleState != "ACTIVE" {
		return fmt.Errorf("project '%s' exists but is not active (status: %s)", projectID, project.LifecycleState)
	}

	fmt.Printf("✅ Project '%s' validated successfully\n", project.ProjectId)
	return nil
}

// isAuthenticated checks if user has valid credentials
func isAuthenticated() bool {
	cmd := exec.Command("gcloud", "auth", "application-default", "print-access-token")
	err := cmd.Run()
	return err == nil
}

// ListUserProjects shows available projects (helpful for error messages)
func ListUserProjects(ctx context.Context) ([]string, error) {
	cmd := exec.Command("gcloud", "projects", "list", "--format=value(projectId)")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	projects := strings.Split(strings.TrimSpace(string(output)), "\n")
	return projects, nil
}
