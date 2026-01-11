package gcpConnector

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/api/compute/v1"
)

type InstanceConfig struct {
	Name      string
	Zone      string
	ProjectID string
	SSHKey    string // Public SSH key to add
}

// CreateInstance creates an e2-micro instance
func CreateInstance(ctx context.Context, service *compute.Service, cfg InstanceConfig) (*compute.Instance, error) {
	fmt.Printf("   🔧 Creating instance '%s' in zone '%s'...\n", cfg.Name, cfg.Zone)

	// Define the instance specification
	instance := &compute.Instance{
		Name:        cfg.Name,
		MachineType: fmt.Sprintf("zones/%s/machineTypes/e2-micro", cfg.Zone),

		// Boot disk with Debian 11
		Disks: []*compute.AttachedDisk{
			{
				Boot:       true,
				AutoDelete: true,
				InitializeParams: &compute.AttachedDiskInitializeParams{
					SourceImage: "projects/debian-cloud/global/images/family/debian-11",
					DiskSizeGb:  10,
					DiskType:    fmt.Sprintf("zones/%s/diskTypes/pd-standard", cfg.Zone),
				},
			},
		},

		// Network with external IP
		NetworkInterfaces: []*compute.NetworkInterface{
			{
				Network: "global/networks/default",
				AccessConfigs: []*compute.AccessConfig{
					{
						Type: "ONE_TO_ONE_NAT",
						Name: "External NAT",
					},
				},
			},
		},

		// Add SSH key for access
		Metadata: &compute.Metadata{
			Items: []*compute.MetadataItems{
				{
					Key:   "ssh-keys",
					Value: stringPtr(fmt.Sprintf("runtime:%s", cfg.SSHKey)),
				},
			},
		},

		// Tags for firewall rules
		Tags: &compute.Tags{
			Items: []string{"runtime-instance", "http-server"},
		},
	}

	// Make API call to create instance
	op, err := service.Instances.Insert(cfg.ProjectID, cfg.Zone, instance).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to create instance: %w", err)
	}

	fmt.Printf("   ⏳ Waiting for instance to be ready...\n")

	// Wait for the operation to complete
	if err := waitForOperation(ctx, service, cfg.ProjectID, cfg.Zone, op.Name); err != nil {
		return nil, err
	}

	// Get the created instance to retrieve details
	inst, err := service.Instances.Get(cfg.ProjectID, cfg.Zone, cfg.Name).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get instance details: %w", err)
	}

	fmt.Printf("   ✅ Instance created successfully!\n")

	return inst, nil
}

// GetExternalIP extracts the external IP from an instance
func GetExternalIP(instance *compute.Instance) string {
	if len(instance.NetworkInterfaces) > 0 &&
		len(instance.NetworkInterfaces[0].AccessConfigs) > 0 {
		return instance.NetworkInterfaces[0].AccessConfigs[0].NatIP
	}
	return ""
}

// DeleteInstance removes an instance
func DeleteInstance(ctx context.Context, service *compute.Service, projectID, zone, name string) error {
	fmt.Printf("   🗑️  Deleting instance '%s'...\n", name)

	op, err := service.Instances.Delete(projectID, zone, name).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to delete instance: %w", err)
	}

	return waitForOperation(ctx, service, projectID, zone, op.Name)
}

// waitForOperation polls until a GCP operation completes
func waitForOperation(ctx context.Context, service *compute.Service, project, zone, opName string) error {
	for {
		op, err := service.ZoneOperations.Get(project, zone, opName).Context(ctx).Do()
		if err != nil {
			return fmt.Errorf("failed to get operation status: %w", err)
		}

		if op.Status == "DONE" {
			if op.Error != nil {
				return fmt.Errorf("operation failed: %v", op.Error.Errors)
			}
			return nil
		}

		// Poll every 2 seconds
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

// Helper function to convert string to pointer
func stringPtr(s string) *string {
	return &s
}
