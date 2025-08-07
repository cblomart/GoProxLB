/*
GoProxLB - Intelligent Load Balancer for Proxmox Clusters
Copyright (C) 2024 GoProxLB Contributors

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

package main

import (
	"fmt"
	"os"

	"github.com/cblomart/GoProxLB/internal/app"
	"github.com/spf13/cobra"
)

const Version = "1.0.0"

var (
	configPath   string
	detailed     bool
	forecast     string
	csvOutput    string
	force        bool
	balancerType string
	serviceName  = "goproxlb"
	serviceUser  = "goproxlb"
	serviceGroup = "goproxlb"
)

var rootCmd = &cobra.Command{
	Use:   "goproxlb",
	Short: "GoProxLB - Advanced Load Balancer for Proxmox",
	Long: `GoProxLB is an intelligent load balancer for Proxmox clusters.
It provides advanced load balancing with capacity planning, 
workload profiling, and distributed operation capabilities.

Features:
- Advanced load balancing with capacity planning
- Workload profiling and VM analysis
- Distributed mode with leader election
- Auto-detection of cluster configuration
- Conservative defaults for trust building

Examples:
  goproxlb                    # Start with defaults (auto-detects everything)
  goproxlb --config config.yaml  # Use specific config file
  goproxlb list              # List VMs
  goproxlb capacity          # Show capacity planning
  goproxlb cluster           # Show cluster info
  goproxlb raft              # Show Raft cluster status`,
	Version: Version,
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the load balancer daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath, _ := cmd.Flags().GetString("config")
		balancerType, _ := cmd.Flags().GetString("balancer-type")
		return app.StartWithBalancerType(configPath, balancerType)
	},
}

var balanceCmd = &cobra.Command{
	Use:   "balance",
	Short: "Force a balancing cycle",
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath, _ := cmd.Flags().GetString("config")
		force, _ := cmd.Flags().GetBool("force")
		balancerType, _ := cmd.Flags().GetString("balancer-type")
		return app.ForceBalanceWithBalancerType(configPath, force, balancerType)
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show cluster status",
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath, _ := cmd.Flags().GetString("config")
		return app.ShowStatus(configPath)
	},
}

var clusterCmd = &cobra.Command{
	Use:   "cluster",
	Short: "Show cluster information",
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath, _ := cmd.Flags().GetString("config")
		return app.ShowClusterInfo(configPath)
	},
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all VMs",
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath, _ := cmd.Flags().GetString("config")
		return app.ListVMs(configPath)
	},
}

var capacityCmd = &cobra.Command{
	Use:   "capacity",
	Short: "Show capacity planning information",
	Long: `Show detailed capacity planning information including:
- Host and VM profiles
- Predicted resource evolution for the next weeks
- Resource adaptation recommendations
- Buffer requirements based on workload patterns`,
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath, _ := cmd.Flags().GetString("config")
		detailed, _ := cmd.Flags().GetBool("detailed")
		forecast, _ := cmd.Flags().GetString("forecast")
		csvOutput, _ := cmd.Flags().GetString("csv")
		return app.ShowCapacityPlanning(configPath, detailed, forecast, csvOutput)
	},
}

var raftCmd = &cobra.Command{
	Use:   "raft",
	Short: "Show Raft cluster status",
	Long: `Show detailed Raft cluster status information including:
- Current node state (Leader/Follower/Candidate)
- Leader information
- Peer nodes and their status
- Cluster health and quorum status
- Auto-discovery information`,
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath, _ := cmd.Flags().GetString("config")
		return app.ShowRaftStatus(configPath)
	},
}

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install GoProxLB as a systemd service",
	Long: `Install GoProxLB as a systemd service for automatic startup and management.

This command will:
- Create a systemd service file
- Create the goproxlb user and group
- Enable and start the service (if --enable is used)
- Set up proper permissions

Examples:
  goproxlb install                    # Install with defaults (auto-detection)
  goproxlb install --config /etc/goproxlb/config.yaml
  goproxlb install --user root       # Install as root user
  goproxlb install --enable          # Enable and start service on boot`,
	RunE: func(cmd *cobra.Command, args []string) error {
		enableService, _ := cmd.Flags().GetBool("enable")
		return app.InstallService(serviceUser, serviceGroup, configPath, enableService)
	},
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "", "Configuration file path (optional - uses defaults with auto-detection)")

	// Command-specific flags
	listCmd.Flags().BoolVarP(&detailed, "detailed", "d", false, "Show detailed information")
	capacityCmd.Flags().BoolVarP(&detailed, "detailed", "d", false, "Show detailed information")
	capacityCmd.Flags().StringVarP(&forecast, "forecast", "f", "168h", "Forecast period (e.g., 168h for 7 days)")
	capacityCmd.Flags().StringVarP(&csvOutput, "csv", "", "", "Output to CSV file")
	balanceCmd.Flags().BoolVarP(&force, "force", "f", false, "Force balancing even if no improvement")
	balanceCmd.Flags().StringVarP(&balancerType, "balancer", "b", "", "Balancer type (threshold or advanced)")

	// Install command flags
	installCmd.Flags().StringVarP(&serviceUser, "user", "u", "goproxlb", "User to run the service as")
	installCmd.Flags().StringVarP(&serviceGroup, "group", "g", "goproxlb", "Group to run the service as")
	installCmd.Flags().BoolP("enable", "e", false, "Enable service to start on boot")

	// Add subcommands
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(clusterCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(balanceCmd)
	rootCmd.AddCommand(capacityCmd)
	rootCmd.AddCommand(raftCmd)
	rootCmd.AddCommand(installCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
