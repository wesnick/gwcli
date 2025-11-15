package main

import (
	"context"
	"fmt"

	"github.com/wesnick/gwcli/pkg/gwcli"
)

// getConnection creates a CmdG connection with authentication
func getConnection(configDir string, verbose bool) (*gwcli.CmdG, error) {
	conn, err := gwcli.New(configDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection: %w", err)
	}

	// Load labels so they're available for label-based operations
	ctx := context.Background()
	if err := conn.LoadLabels(ctx, verbose); err != nil {
		return nil, fmt.Errorf("failed to load labels: %w", err)
	}

	return conn, nil
}

// runConfigure runs the OAuth configuration flow
func runConfigure(configDir string) error {
	paths, err := gwcli.GetConfigPaths(configDir)
	if err != nil {
		return err
	}

	fmt.Printf("Configuring OAuth authentication...\n")
	fmt.Printf("Config directory: %s\n\n", paths.Dir)
	fmt.Printf("Required files:\n")
	fmt.Printf("  - %s (OAuth credentials from Google Console)\n", paths.Credentials)
	fmt.Printf("  - %s (will be auto-generated)\n\n", paths.Token)

	ctx := context.Background()
	if err := gwcli.ConfigureAuth(ctx, paths, 8080); err != nil {
		return fmt.Errorf("configuration failed: %w", err)
	}

	fmt.Printf("\nConfiguration complete!\n")
	fmt.Printf("You can now use gwcli commands.\n")
	return nil
}
