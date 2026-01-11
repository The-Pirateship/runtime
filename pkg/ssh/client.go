package ssh

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Client struct {
	Host string // External IP address
	User string // SSH username (default: "runtime")
}

// WaitForSSH waits until SSH is ready on the instance
func (c *Client) WaitForSSH(maxWait time.Duration) error {
	fmt.Printf("   ⏳ Waiting for SSH to be ready...")

	deadline := time.Now().Add(maxWait)
	attempt := 0

	for time.Now().Before(deadline) {
		attempt++

		cmd := exec.Command("ssh",
			"-o", "StrictHostKeyChecking=no",
			"-o", "UserKnownHostsFile=/dev/null",
			"-o", "ConnectTimeout=5",
			"-o", "LogLevel=ERROR",
			fmt.Sprintf("%s@%s", c.User, c.Host),
			"echo 'ready'",
		)

		if err := cmd.Run(); err == nil {
			fmt.Printf("\r   ✅ SSH is ready                    \n")
			return nil
		}

		// Show spinner
		spinners := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		fmt.Printf("\r   %s Waiting for SSH to be ready... (attempt %d)", spinners[attempt%len(spinners)], attempt)

		time.Sleep(3 * time.Second)
	}

	return fmt.Errorf("\nSSH did not become ready within %v", maxWait)
}

// UploadDirectory uploads git-tracked files to the remote instance
func (c *Client) UploadDirectory(localPath, remotePath string) error {
	// Find git root
	gitRoot, err := findGitRoot(localPath)
	if err != nil {
		return fmt.Errorf("path must be in a git repository: %w\n\nRun: git init && git add . && git commit -m 'initial'", err)
	}

	// Get relative path from git root
	absLocalPath, err := filepath.Abs(localPath)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	relPath, err := filepath.Rel(gitRoot, absLocalPath)
	if err != nil {
		return fmt.Errorf("failed to get relative path: %w", err)
	}

	// Normalize to use forward slashes (git uses forward slashes even on Windows)
	relPath = filepath.ToSlash(relPath)
	if relPath == "." {
		relPath = ""
	}

	// Get list of tracked files
	lsFilesCmd := exec.Command("git", "ls-files", relPath)
	lsFilesCmd.Dir = gitRoot
	output, err := lsFilesCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to list git files: %w", err)
	}

	trackedFiles := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(trackedFiles) == 0 || (len(trackedFiles) == 1 && trackedFiles[0] == "") {
		return fmt.Errorf("no tracked files found in %s\n\nRun: git add . && git commit -m 'add files'", localPath)
	}

	fmt.Printf("   📦 Uploading %d files (respecting .gitignore)...", len(trackedFiles))

	// Show progress spinner
	done := make(chan bool)
	go func() {
		spinners := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		i := 0
		for {
			select {
			case <-done:
				return
			default:
				fmt.Printf("\r   %s Uploading %d files...", spinners[i%len(spinners)], len(trackedFiles))
				i++
				time.Sleep(100 * time.Millisecond)
			}
		}
	}()

	// Create tar archive of tracked files
	tmpDir := os.TempDir()
	tarFile := filepath.Join(tmpDir, fmt.Sprintf("runtime-deploy-%d.tar", time.Now().Unix()))
	defer os.Remove(tarFile)

	// Create archive with only the files in our subdirectory
	var tarCmd *exec.Cmd
	if relPath == "" {
		// Root of repo
		tarCmd = exec.Command("git", "archive", "--format=tar", "-o", tarFile, "HEAD")
	} else {
		// Subdirectory
		tarCmd = exec.Command("git", "archive", "--format=tar", "-o", tarFile, "HEAD", relPath)
	}
	tarCmd.Dir = gitRoot

	if err := tarCmd.Run(); err != nil {
		close(done)
		return fmt.Errorf("\nfailed to create archive: %w", err)
	}

	// Create remote directory
	if err := c.RunCommandQuiet(fmt.Sprintf("mkdir -p %s", remotePath)); err != nil {
		close(done)
		return fmt.Errorf("\nfailed to create remote directory: %w", err)
	}

	// Upload tar file
	scpCmd := exec.Command("scp",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=ERROR",
		tarFile,
		fmt.Sprintf("%s@%s:%s/archive.tar", c.User, c.Host, remotePath),
	)

	if err := scpCmd.Run(); err != nil {
		close(done)
		return fmt.Errorf("\nfailed to upload archive: %w", err)
	}

	// Extract tar on remote
	// Calculate strip-components based on depth
	stripComponents := 0
	if relPath != "" {
		stripComponents = len(strings.Split(relPath, "/"))
	}

	extractCmd := fmt.Sprintf("cd %s && tar -xf archive.tar --strip-components=%d && rm archive.tar",
		remotePath, stripComponents)

	if err := c.RunCommandQuiet(extractCmd); err != nil {
		close(done)
		return fmt.Errorf("\nfailed to extract archive: %w", err)
	}

	close(done)
	fmt.Printf("\r   ✅ Uploaded %d files successfully                    \n", len(trackedFiles))

	return nil
}

// findGitRoot walks up from the given path to find the git repository root
func findGitRoot(startPath string) (string, error) {
	absPath, err := filepath.Abs(startPath)
	if err != nil {
		return "", err
	}

	currentPath := absPath
	for {
		gitDir := filepath.Join(currentPath, ".git")
		if info, err := os.Stat(gitDir); err == nil && info.IsDir() {
			return currentPath, nil
		}

		// Move up one directory
		parentPath := filepath.Dir(currentPath)
		if parentPath == currentPath {
			// Reached root without finding .git
			return "", fmt.Errorf("not in a git repository")
		}
		currentPath = parentPath
	}
}

// RunCommand executes a command on the remote instance with output
func (c *Client) RunCommand(command string) error {
	cmd := exec.Command("ssh",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=ERROR",
		fmt.Sprintf("%s@%s", c.User, c.Host),
		command,
	)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// RunCommandQuiet executes a command without showing output
func (c *Client) RunCommandQuiet(command string) error {
	cmd := exec.Command("ssh",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=ERROR",
		fmt.Sprintf("%s@%s", c.User, c.Host),
		command,
	)

	return cmd.Run()
}
