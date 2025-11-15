package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/wesnick/cmdg/pkg/cmdg"
)

// getConfigPath returns the config file path, expanding ~ if needed
func getConfigPath(configFlag string) (string, error) {
	if configFlag == "" {
		configFlag = "~/.cmdg/cmdg.conf"
	}

	// Expand ~
	if len(configFlag) > 0 && configFlag[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot determine home directory: %w", err)
		}
		configFlag = filepath.Join(home, configFlag[1:])
	}

	return configFlag, nil
}

// getConnection creates a CmdG connection with authentication
func getConnection(configPath string) (*cmdg.CmdG, error) {
	configPath, err := getConfigPath(configPath)
	if err != nil {
		return nil, err
	}

	// Check if config exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file not found: %s\nRun 'gwcli configure' to set up authentication", configPath)
	}

	conn, err := cmdg.New(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create Gmail connection: %w", err)
	}

	// Load labels so they're available for label-based operations
	ctx := context.Background()
	if err := conn.LoadLabels(ctx); err != nil {
		return nil, fmt.Errorf("failed to load labels: %w", err)
	}

	return conn, nil
}

// runConfigure runs the OAuth configuration flow
func runConfigure(configPath string) error {
	configPath, err := getConfigPath(configPath)
	if err != nil {
		return err
	}

	fmt.Printf("Configuring OAuth authentication...\n")
	fmt.Printf("Config will be saved to: %s\n\n", configPath)

	if err := cmdg.Configure(configPath); err != nil {
		return fmt.Errorf("configuration failed: %w", err)
	}

	fmt.Printf("\nConfiguration complete!\n")
	return nil
}
