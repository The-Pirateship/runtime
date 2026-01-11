package gcpConnector

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/api/compute/v1"
)

// EnsureFirewallRules creates necessary firewall rules if they don't exist
func EnsureFirewallRules(ctx context.Context, service *compute.Service, projectID string) error {
	fmt.Println("🔒 Checking firewall rules...")

	// Rule 1: Allow SSH (port 22)
	if err := ensureFirewallRule(ctx, service, projectID, &compute.Firewall{
		Name:    "runtime-allow-ssh",
		Network: "global/networks/default",
		Allowed: []*compute.FirewallAllowed{
			{
				IPProtocol: "tcp",
				Ports:      []string{"22"},
			},
		},
		SourceRanges: []string{"0.0.0.0/0"},
		TargetTags:   []string{"runtime-instance"},
		Description:  "Allow SSH access to Runtime instances",
	}); err != nil {
		return err
	}

	// Rule 2: Allow HTTP traffic (common ports)
	if err := ensureFirewallRule(ctx, service, projectID, &compute.Firewall{
		Name:    "runtime-allow-http",
		Network: "global/networks/default",
		Allowed: []*compute.FirewallAllowed{
			{
				IPProtocol: "tcp",
				Ports:      []string{"80", "443", "3000", "8000", "8080"},
			},
		},
		SourceRanges: []string{"0.0.0.0/0"},
		TargetTags:   []string{"runtime-instance"},
		Description:  "Allow HTTP traffic to Runtime instances",
	}); err != nil {
		return err
	}

	fmt.Println("✅ Firewall rules configured\n")
	return nil
}

func ensureFirewallRule(ctx context.Context, service *compute.Service, projectID string, rule *compute.Firewall) error {
	// Check if rule already exists
	_, err := service.Firewalls.Get(projectID, rule.Name).Context(ctx).Do()
	if err == nil {
		// Rule already exists
		return nil
	}

	// Create the rule
	_, err = service.Firewalls.Insert(projectID, rule).Context(ctx).Do()
	if err != nil && !isAlreadyExistsError(err) {
		return fmt.Errorf("failed to create firewall rule '%s': %w", rule.Name, err)
	}

	return nil
}

func isAlreadyExistsError(err error) bool {
	return strings.Contains(err.Error(), "alreadyExists")
}
