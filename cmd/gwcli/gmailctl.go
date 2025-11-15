package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/wesnick/gwcli/pkg/gwcli"
)

// runGmailctlDownload downloads filters from Gmail to config.jsonnet
func runGmailctlDownload(configDir, outputFile string, out *outputWriter) error {
	// Get config paths
	paths, err := gwcli.GetConfigPaths(configDir)
	if err != nil {
		return err
	}

	// Check if gmailctl is installed
	gmailctlPath, err := exec.LookPath("gmailctl")
	if err != nil {
		return fmt.Errorf("gmailctl not found in PATH. Install it with: go install github.com/mbrt/gmailctl/cmd/gmailctl@latest")
	}

	// Build command: gmailctl --config <dir> download -o <output>
	args := []string{"--config", paths.Dir, "download"}
	if outputFile != "" {
		args = append(args, "-o", outputFile)
	}

	if out.verbose {
		fmt.Fprintf(os.Stderr, "Running: %s %v\n", gmailctlPath, args)
	}

	// Execute gmailctl with passthrough I/O
	cmd := exec.Command(gmailctlPath, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		return err
	}

	return nil
}

// runGmailctlApply applies config.jsonnet to Gmail filters
func runGmailctlApply(configDir string, skipConfirm bool, out *outputWriter) error {
	// Get config paths
	paths, err := gwcli.GetConfigPaths(configDir)
	if err != nil {
		return err
	}

	// Check if gmailctl is installed
	gmailctlPath, err := exec.LookPath("gmailctl")
	if err != nil {
		return fmt.Errorf("gmailctl not found in PATH. Install it with: go install github.com/mbrt/gmailctl/cmd/gmailctl@latest")
	}

	// Build command: gmailctl --config <dir> apply [--yes]
	args := []string{"--config", paths.Dir, "apply"}
	if skipConfirm {
		args = append(args, "--yes")
	}

	if out.verbose {
		fmt.Fprintf(os.Stderr, "Running: %s %v\n", gmailctlPath, args)
	}

	// Execute gmailctl with passthrough I/O
	cmd := exec.Command(gmailctlPath, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		return err
	}

	return nil
}

// runGmailctlDiff shows diff between local config and Gmail
func runGmailctlDiff(configDir string, out *outputWriter) error {
	// Get config paths
	paths, err := gwcli.GetConfigPaths(configDir)
	if err != nil {
		return err
	}

	// Check if gmailctl is installed
	gmailctlPath, err := exec.LookPath("gmailctl")
	if err != nil {
		return fmt.Errorf("gmailctl not found in PATH. Install it with: go install github.com/mbrt/gmailctl/cmd/gmailctl@latest")
	}

	// Build command: gmailctl --config <dir> diff
	args := []string{"--config", paths.Dir, "diff"}

	if out.verbose {
		fmt.Fprintf(os.Stderr, "Running: %s %v\n", gmailctlPath, args)
	}

	// Execute gmailctl with passthrough I/O
	cmd := exec.Command(gmailctlPath, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		return err
	}

	return nil
}
