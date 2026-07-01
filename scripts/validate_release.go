package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: validate_release <target_dir>")
		os.Exit(1)
	}
	targetDir := os.Args[1]
	configPath := filepath.Join(targetDir, "config.json")

	b, err := os.ReadFile(configPath)
	if err != nil {
		fmt.Printf("Failed to read config.json: %v\n", err)
		os.Exit(1)
	}

	var config struct {
		UpdateDirectories []string `json:"update_directories"`
		UpdateFiles       []string `json:"update_files"`
	}
	if err := json.Unmarshal(b, &config); err != nil {
		fmt.Printf("Failed to parse config.json: %v\n", err)
		os.Exit(1)
	}

	expected := make(map[string]bool)
	for _, d := range config.UpdateDirectories {
		expected[d] = true
	}
	for _, f := range config.UpdateFiles {
		expected[f] = true
	}

	found := make(map[string]bool)
	err = filepath.Walk(targetDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if path == targetDir {
			return nil
		}
		rel, err := filepath.Rel(targetDir, path)
		if err != nil {
			return err
		}
		
		// Ignore tarballs and signatures if they are in the dir
		if strings.HasSuffix(rel, ".tar.gz") || strings.HasSuffix(rel, ".sig") {
			return nil
		}

		found[rel] = true
		return nil
	})
	if err != nil {
		fmt.Printf("Failed to walk target directory: %v\n", err)
		os.Exit(1)
	}

	failed := false
	for e := range expected {
		if !found[e] {
			fmt.Printf("Error: Expected file/directory missing from release: %s\n", e)
			failed = true
		}
	}
	for f := range found {
		if !expected[f] {
			fmt.Printf("Error: Extra file/directory found in release not listed in config.json: %s\n", f)
			failed = true
		}
	}

	if failed {
		fmt.Println("Release validation failed.")
		os.Exit(1)
	}
	fmt.Println("Release validation passed successfully.")
}
