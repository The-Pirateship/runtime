package dev

import (
	"fmt"
	"os"
	"path/filepath"

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
