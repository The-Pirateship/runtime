package ssh

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GetOrCreateSSHKey gets existing SSH key or creates a new one
func GetOrCreateSSHKey() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	sshDir := filepath.Join(home, ".ssh")
	publicKeyPath := filepath.Join(sshDir, "id_rsa.pub")
	privateKeyPath := filepath.Join(sshDir, "id_rsa")

	// Check if key already exists
	if _, err := os.Stat(publicKeyPath); err == nil {
		// Read existing key
		data, err := os.ReadFile(publicKeyPath)
		if err != nil {
			return "", fmt.Errorf("failed to read SSH public key: %w", err)
		}
		return strings.TrimSpace(string(data)), nil
	}

	// Key doesn't exist, create it
	fmt.Println("🔑 Generating SSH key pair...")

	// Ensure .ssh directory exists
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		return "", err
	}

	// Generate key
	cmd := exec.Command("ssh-keygen",
		"-t", "rsa",
		"-b", "4096",
		"-f", privateKeyPath,
		"-N", "", // No passphrase
		"-C", "runtime-cli",
	)

	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("failed to generate SSH key: %w\nOutput: %s", err, output)
	}

	// Read the newly created public key
	data, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return "", err
	}

	fmt.Println("✅ SSH key generated\n")
	return strings.TrimSpace(string(data)), nil
}
