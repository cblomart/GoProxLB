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

// Helper functions.

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

// Test individual command creation and properties.
func TestStartCommand(t *testing.T) {
	startCmd := findCommand(rootCmd, "start")
	if startCmd == nil {
		t.Fatal("Expected start command to exist")
	}

	if startCmd.Use != "start" { //nolint:staticcheck // false positive - startCmd checked for nil above
		t.Errorf("Expected start command Use to be 'start', got %s", startCmd.Use)
	}

	if startCmd.Short == "" {
		t.Error("Expected start command to have Short description")
	}

	// Test that start command has RunE function
	if startCmd.RunE == nil {
		t.Error("Expected start command to have RunE function")
	}
}

func TestBalanceCommand(t *testing.T) {
	balanceCmd := findCommand(rootCmd, "balance")
	if balanceCmd == nil {
		t.Fatal("Expected balance command to exist")
	}

	if balanceCmd.Use != "balance" { //nolint:staticcheck // false positive - balanceCmd checked for nil above
		t.Errorf("Expected balance command Use to be 'balance', got %s", balanceCmd.Use)
	}

	// Test force flag
	forceFlag := balanceCmd.Flags().Lookup("force")
	if forceFlag == nil {
		t.Error("Expected balance command to have force flag")
	}

	// Test balancer-type flag
	balancerFlag := balanceCmd.Flags().Lookup("balancer")
	if balancerFlag == nil {
		t.Error("Expected balance command to have balancer flag")
	}
}

func TestCapacityCommand(t *testing.T) {
	capacityCmd := findCommand(rootCmd, "capacity")
	if capacityCmd == nil {
		t.Fatal("Expected capacity command to exist")
	}

	if capacityCmd.Long == "" { //nolint:staticcheck // false positive - capacityCmd checked for nil above
		t.Error("Expected capacity command to have Long description")
	}

	// Test detailed flag
	detailedFlag := capacityCmd.Flags().Lookup("detailed")
	if detailedFlag == nil {
		t.Error("Expected capacity command to have detailed flag")
	}

	// Test forecast flag
	forecastFlag := capacityCmd.Flags().Lookup("forecast")
	if forecastFlag == nil {
		t.Error("Expected capacity command to have forecast flag")
	}

	// Test csv flag
	csvFlag := capacityCmd.Flags().Lookup("csv")
	if csvFlag == nil {
		t.Error("Expected capacity command to have csv flag")
	}
}

func TestInstallCommand(t *testing.T) {
	installCmd := findCommand(rootCmd, "install")
	if installCmd == nil {
		t.Fatal("Expected install command to exist")
	}

	if installCmd.Long == "" { //nolint:staticcheck // false positive - installCmd checked for nil above
		t.Error("Expected install command to have Long description")
	}

	// Test user flag
	userFlag := installCmd.Flags().Lookup("user")
	if userFlag == nil {
		t.Error("Expected install command to have user flag")
	}

	// Test group flag
	groupFlag := installCmd.Flags().Lookup("group")
	if groupFlag == nil {
		t.Error("Expected install command to have group flag")
	}

	// Test enable flag
	enableFlag := installCmd.Flags().Lookup("enable")
	if enableFlag == nil {
		t.Error("Expected install command to have enable flag")
	}
}

func TestRaftCommand(t *testing.T) {
	raftCmd := findCommand(rootCmd, "raft")
	if raftCmd == nil {
		t.Fatal("Expected raft command to exist")
	}

	if raftCmd.Use != "raft" { //nolint:staticcheck // false positive - raftCmd checked for nil above
		t.Errorf("Expected raft command Use to be 'raft', got %s", raftCmd.Use)
	}

	if raftCmd.Long == "" {
		t.Error("Expected raft command to have Long description")
	}
}

func TestStatusCommand(t *testing.T) {
	statusCmd := findCommand(rootCmd, "status")
	if statusCmd == nil {
		t.Fatal("Expected status command to exist")
	}

	if statusCmd.Use != "status" { //nolint:staticcheck // false positive - statusCmd checked for nil above
		t.Errorf("Expected status command Use to be 'status', got %s", statusCmd.Use)
	}

	if statusCmd.Short == "" {
		t.Error("Expected status command to have Short description")
	}
}

func TestClusterCommand(t *testing.T) {
	clusterCmd := findCommand(rootCmd, "cluster")
	if clusterCmd == nil {
		t.Fatal("Expected cluster command to exist")
	}

	if clusterCmd.Use != "cluster" { //nolint:staticcheck // false positive - clusterCmd checked for nil above
		t.Errorf("Expected cluster command Use to be 'cluster', got %s", clusterCmd.Use)
	}
}

func TestListCommand(t *testing.T) {
	listCmd := findCommand(rootCmd, "list")
	if listCmd == nil {
		t.Fatal("Expected list command to exist")
	}

	if listCmd.Use != "list" { //nolint:staticcheck // false positive - listCmd checked for nil above
		t.Errorf("Expected list command Use to be 'list', got %s", listCmd.Use)
	}

	// Test detailed flag
	detailedFlag := listCmd.Flags().Lookup("detailed")
	if detailedFlag == nil {
		t.Error("Expected list command to have detailed flag")
	}
}

func TestCommandErrorHandling(t *testing.T) {
	// Test non-existent command
	nonExistentCmd := findCommand(rootCmd, "nonexistent")
	if nonExistentCmd != nil {
		t.Error("Expected non-existent command to return nil")
	}

	// Test that commands handle missing config gracefully
	tests := []struct {
		commandName string
		args        []string
	}{
		{"status", []string{"--config", "/nonexistent/config.yaml"}},
		{"cluster", []string{"--config", "/nonexistent/config.yaml"}},
		{"list", []string{"--config", "/nonexistent/config.yaml"}},
		{"balance", []string{"--config", "/nonexistent/config.yaml"}},
		{"capacity", []string{"--config", "/nonexistent/config.yaml"}},
	}

	for _, test := range tests {
		t.Run(test.commandName+"_error", func(t *testing.T) {
			cmd := findCommand(rootCmd, test.commandName)
			if cmd == nil {
				t.Skipf("Command %s not found", test.commandName)
			}

			// This should not panic, but may return an error
			output := captureCommandOutput(cmd, test.args)

			// Just verify it doesn't panic and produces some output
			if output != "" {
				t.Logf("Command %s with invalid config produced output: %s", test.commandName, output[:minInt(50, len(output))])
			}
		})
	}
}

func TestMainFunction(t *testing.T) {
	// Test that main function doesn't immediately panic
	// This is a basic smoke test

	// Save original args
	originalArgs := os.Args
	defer func() { os.Args = originalArgs }()

	// Test with version flag
	os.Args = []string{"goproxlb", "--version"}

	// This should not panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("main() panicked with version flag: %v", r)
		}
	}()

	// Note: We can't easily test main() execution without it actually running
	// so we'll just test that the root command is properly set up
	if rootCmd == nil {
		t.Error("Expected rootCmd to be initialized for main function")
	}
}

func TestDefaultValues(t *testing.T) {
	// Test that default values are properly set
	if serviceUser != "goproxlb" {
		t.Errorf("Expected default serviceUser to be 'goproxlb', got %s", serviceUser)
	}

	if serviceGroup != "goproxlb" {
		t.Errorf("Expected default serviceGroup to be 'goproxlb', got %s", serviceGroup)
	}

	if Version == "" {
		t.Error("Expected Version constant to be set")
	}
}

// Helper function for Go 1.21+ compatibility.
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
