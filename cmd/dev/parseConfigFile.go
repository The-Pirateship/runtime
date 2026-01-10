package dev

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

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

	// Get service order from file
	serviceOrder, err := getServiceOrder(filename)
	if err != nil {
		fmt.Printf("❌ Failed to get service order: %v\n", err)
		return Config{}
	}

	// Process services in the order they appear in the file
	for _, key := range serviceOrder {
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

// getServiceOrder scans the TOML file and returns service names in definition order
func getServiceOrder(filename string) ([]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var serviceOrder []string
	scanner := bufio.NewScanner(file)
	sectionRegex := regexp.MustCompile(`^\[([^\]]+)\]`)

	for scanner.Scan() {
		line := scanner.Text()
		if matches := sectionRegex.FindStringSubmatch(line); matches != nil {
			serviceName := matches[1]
			if serviceName != "name" { // skip the global name section
				serviceOrder = append(serviceOrder, serviceName)
			}
		}
	}

	return serviceOrder, scanner.Err()
}
