package dev

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml"
)

type Service struct {
	Name    string
	Path    string
	Command string
}

type Config struct {
	Name     string
	Services []Service
}

func parseConfig(filename string) Config {
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		fmt.Printf("❌ %s not found\n", filename)
		return Config{}
	}

	tree, err := toml.LoadFile(filename)
	if err != nil {
		fmt.Printf("❌ Failed to parse config: %v\n", err)
		return Config{}
	}

	configDir, _ := filepath.Abs(filepath.Dir(filename))
	services := []Service{}

	for _, key := range tree.Keys() {
		if key == "name" {
			continue
		}

		svc := tree.Get(key).(*toml.Tree)
		path := svc.Get("path")
		cmd := svc.Get("runCommand")

		if path != nil && cmd != nil {
			services = append(services, Service{
				Name:    key,
				Path:    filepath.Join(configDir, path.(string)),
				Command: cmd.(string),
			})
		}
	}

	return Config{"", services}
}

func generateZellijLayout(config Config) error {
	// Create .zellij directory if it doesn't exist
	zellijDir := ".zellij"
	if err := os.MkdirAll(zellijDir, 0755); err != nil {
		return fmt.Errorf("failed to create .zellij directory: %w", err)
	}

	// Generate KDL layout content
	var layoutBuilder strings.Builder
	layoutBuilder.WriteString("layout {\n")

	// Add default tab template with tab-bar and status-bar
	layoutBuilder.WriteString("    default_tab_template {\n")
	layoutBuilder.WriteString("        pane size=1 borderless=true {\n")
	layoutBuilder.WriteString("            plugin location=\"zellij:tab-bar\"\n")
	layoutBuilder.WriteString("        }\n")
	layoutBuilder.WriteString("        children\n")
	// removed cuz we dont wnat the bottom status bar
	// layoutBuilder.WriteString("        pane size=2 borderless=true {\n")
	// layoutBuilder.WriteString("            plugin location=\"zellij:status-bar\"\n")
	// layoutBuilder.WriteString("        }\n")
	layoutBuilder.WriteString("    }\n")

	// Add tabs for each service
	for _, service := range config.Services {
		layoutBuilder.WriteString(fmt.Sprintf("    tab name=\"%s\" {\n", service.Name))
		layoutBuilder.WriteString("        pane command=\"sh\" {\n")
		layoutBuilder.WriteString(fmt.Sprintf("            args \"-c\" \"cd %s && %s\"\n", service.Path, service.Command))
		layoutBuilder.WriteString("        }\n")
		layoutBuilder.WriteString("    }\n")
	}

	layoutBuilder.WriteString("}\n")

	// Write layout file
	layoutPath := filepath.Join(zellijDir, "layout.kdl")
	if err := os.WriteFile(layoutPath, []byte(layoutBuilder.String()), 0644); err != nil {
		return fmt.Errorf("failed to write layout file: %w", err)
	}

	fmt.Printf("✅ Generated Zellij layout: %s\n", layoutPath)
	return nil
}

func generateZellijConfig() error {
	// Create .zellij directory if it doesn't exist
	zellijDir := ".zellij"
	if err := os.MkdirAll(zellijDir, 0755); err != nil {
		return fmt.Errorf("failed to create .zellij directory: %w", err)
	}

	// Generate KDL config content with keybindings
	var configBuilder strings.Builder
	configBuilder.WriteString("keybinds {\n")
	configBuilder.WriteString("    normal {\n")
	configBuilder.WriteString("        bind \"Ctrl ,\" { GoToPreviousTab; }\n")
	configBuilder.WriteString("        bind \"Ctrl .\" { GoToNextTab; }\n")
	configBuilder.WriteString("        bind \"Ctrl t\" { NewTab; }\n")
	configBuilder.WriteString("        bind \"Ctrl q\" { Quit; }\n")
	configBuilder.WriteString("    }\n")
	configBuilder.WriteString("}\n")

	// Write config file
	configPath := filepath.Join(zellijDir, "config.kdl")
	if err := os.WriteFile(configPath, []byte(configBuilder.String()), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	fmt.Printf("✅ Generated Zellij config: %s\n", configPath)
	return nil
}
