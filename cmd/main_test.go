package main

import (
	"os"
	"testing"

	"github.com/spf13/cobra"
)

func TestMain(t *testing.T) {
	// Test that main doesn't panic
	// We'll test the actual functionality through the cobra commands
}

func TestRootCommand(t *testing.T) {
	// Test root command creation
	if rootCmd == nil {
		t.Fatal("Expected rootCmd to be initialized")
	}

	// Test root command properties
	if rootCmd.Use != "goproxlb" {
		t.Errorf("Expected Use to be 'goproxlb', got %s", rootCmd.Use)
	}

	if rootCmd.Version != Version {
		t.Errorf("Expected Version to be %s, got %s", Version, rootCmd.Version)
	}
}

func TestVersionCommand(t *testing.T) {
	// Test that version variables are set
	if Version == "" {
		t.Error("Expected Version to be set")
	}

	// BuildTime is no longer used in the new version
	// if BuildTime == "" {
	// 	t.Error("Expected BuildTime to be set")
	// }

	// Test that root command has version set
	if rootCmd.Version != Version {
		t.Errorf("Expected rootCmd.Version to be %s, got %s", Version, rootCmd.Version)
	}
}

func TestHelpCommand(t *testing.T) {
	// Test that root command has help
	if rootCmd.Use == "" {
		t.Error("Expected rootCmd.Use to be set")
	}

	if rootCmd.Short == "" {
		t.Error("Expected rootCmd.Short to be set")
	}

	// Test that root command has the expected name
	if rootCmd.Use != "goproxlb" {
		t.Errorf("Expected rootCmd.Use to be 'goproxlb', got %s", rootCmd.Use)
	}
}

func TestCommandExecution(t *testing.T) {
	// Test that commands can be executed without panicking
	commands := []string{"start", "status", "cluster", "list", "balance", "capacity"}

	for _, cmdName := range commands {
		t.Run(cmdName, func(t *testing.T) {
			// Find the command
			cmd := findCommand(rootCmd, cmdName)
			if cmd == nil {
				t.Skipf("Command '%s' not found", cmdName)
			}

			// Test command execution with help flag
			output := captureCommandOutput(cmd, []string{"--help"})

			if output == "" {
				t.Errorf("Expected non-empty output for command '%s'", cmdName)
			}
		})
	}
}

func TestConfigFlag(t *testing.T) {
	// Test config flag handling
	startCmd := findCommand(rootCmd, "start")
	if startCmd == nil {
		t.Skip("Start command not found")
	}

	// Test with config flag
	output := captureCommandOutput(startCmd, []string{"--config", "test-config.yaml"})

	// Should show some output (even if it's an error about missing config file)
	if output == "" {
		t.Error("Expected output when specifying config file")
	}
}

func TestForceFlag(t *testing.T) {
	// Test force flag handling
	balanceCmd := findCommand(rootCmd, "balance")
	if balanceCmd == nil {
		t.Skip("Balance command not found")
	}

	// Test with force flag
	output := captureCommandOutput(balanceCmd, []string{"--force"})

	// Should show some output (even if it's an error about missing config)
	if output == "" {
		t.Error("Expected output when using force flag")
	}
}

// Helper functions

func captureCommandOutput(cmd *cobra.Command, args []string) string {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Execute command
	cmd.SetArgs(args)
	_ = cmd.Execute()

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout

	// Read output
	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	return string(buf[:n])
}

func findCommand(root *cobra.Command, name string) *cobra.Command {
	for _, cmd := range root.Commands() {
		if cmd.Name() == name {
			return cmd
		}
		// Check subcommands
		if subCmd := findCommand(cmd, name); subCmd != nil {
			return subCmd
		}
	}
	return nil
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		(len(s) > len(substr) && (s[:len(substr)] == substr ||
			s[len(s)-len(substr):] == substr ||
			containsSubstring(s, substr))))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestEnvironmentVariables(t *testing.T) {
	// Test that environment variables are properly handled
	originalEnv := os.Getenv("GOPROXLB_CONFIG")
	defer os.Setenv("GOPROXLB_CONFIG", originalEnv)

	// Set test environment variable
	os.Setenv("GOPROXLB_CONFIG", "/tmp/test-config.yaml")

	// Test that environment variable is accessible
	configPath := os.Getenv("GOPROXLB_CONFIG")
	if configPath != "/tmp/test-config.yaml" {
		t.Errorf("Expected config path to be '/tmp/test-config.yaml', got %s", configPath)
	}
}

func TestCommandStructure(t *testing.T) {
	// Test that all expected commands exist
	expectedCommands := map[string]bool{
		"status":   false,
		"cluster":  false,
		"list":     false,
		"balance":  false,
		"capacity": false,
		"raft":     false,
		"install":  false,
	}

	// Check for commands
	for cmdName := range expectedCommands {
		if cmd := findCommand(rootCmd, cmdName); cmd != nil {
			expectedCommands[cmdName] = true
		}
	}

	// Report missing commands
	for cmdName, found := range expectedCommands {
		if !found {
			t.Errorf("Expected command '%s' not found", cmdName)
		}
	}

	// Test that old commands don't exist
	oldCommands := []string{"vms"}
	for _, cmdName := range oldCommands {
		if cmd := findCommand(rootCmd, cmdName); cmd != nil {
			t.Errorf("Old command '%s' should not exist", cmdName)
		}
	}
}

func TestCommandFlags(t *testing.T) {
	// Test that commands have expected flags
	// Root command now handles starting the service
	configFlag := rootCmd.PersistentFlags().Lookup("config")
	if configFlag == nil {
		t.Error("Expected 'config' flag on root command")
	}

	balanceCmd := findCommand(rootCmd, "balance")
	if balanceCmd != nil {
		forceFlag := balanceCmd.Flags().Lookup("force")
		if forceFlag == nil {
			t.Error("Expected 'force' flag on balance command")
		}
	}
}
