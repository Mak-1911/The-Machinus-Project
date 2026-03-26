package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/machinus/cloud-agent/internal/config"
	"github.com/machinus/cloud-agent/internal/daemon"
	"github.com/spf13/cobra"
)

const (
	machinusDir = "~/.machinus"
	pidFile     = "~/.machinus/pid/agent.pid"
	logDir      = "~/.machinus/logs"
)

// rootCmd is the root command for machinus
var rootCmd = &cobra.Command{
	Use:   "machinus",
	Short: "Machinus AI Agent",
	Long:  `Autonomous AI agent for task automation.`,
}

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Manage the Machinus Daemon",
	Long:  `Manage the Machinus daemon for 24/7 autonomous operations.`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

var daemonStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the daemon",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := config.Load()
		if cfg == nil {
			fmt.Printf("Error loading config\n")
			os.Exit(1)
		}

		d, err := daemon.New(daemon.DaemonConfig{
			PidFile: pidFile,
			LogDir:  logDir,
			WorkDir: ".",
			Config:  cfg,
		})
		if err != nil {
			fmt.Printf("Error creating Daemon: %v\n", err)
			os.Exit(1)
		}

		if err := d.Start(); err != nil {
			fmt.Printf("Error starting daemon: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("Daemon Started Successfully")
		fmt.Printf("PID file: %s\n", expandPath(pidFile))
		fmt.Printf("Log file: %s\n", joinPath(expandPath(logDir), "agent.log"))

		d.Run()
	},
}

var daemonStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the daemon",
	Run: func(cmd *cobra.Command, args []string) {
		pidManager := daemon.NewPIDManager(expandPath(pidFile))

		running, pid := pidManager.IsRunning()
		if !running {
			fmt.Println("Daemon is not running")
			os.Exit(0)
		}
		fmt.Printf("Stopping Daemon (PID: %d)...\n", pid)

		process, err := os.FindProcess(pid)
		if err != nil {
			fmt.Printf("Error finding process: %v\n", err)
			os.Exit(1)
		}

		if err := process.Signal(os.Interrupt); err != nil {
			fmt.Printf("Error sending signal: %v\n", err)
			os.Exit(1)
		}

		if err := pidManager.Remove(); err != nil {
			fmt.Printf("Warning: Could not remove PID file: %v\n", err)
		}
		fmt.Println("Daemon Stopped")
	},
}

var daemonStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check Daemon Status",
	Run: func(cmd *cobra.Command, args []string) {
		pidManager := daemon.NewPIDManager(expandPath(pidFile))
		running, pid := pidManager.IsRunning()
		if !running {
			fmt.Println("Daemon is not running")
			os.Exit(0)
		}
		fmt.Printf("Daemon is running (PID: %d)\n", pid)
		fmt.Printf("Log file: %s\n", joinPath(expandPath(logDir), "agent.log"))
	},
}

var daemonRestartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart the Daemon",
	Run: func(cmd *cobra.Command, args []string) {
		pidManager := daemon.NewPIDManager(expandPath(pidFile))

		running, pid := pidManager.IsRunning()
		if running {
			fmt.Println("Stopping Daemon...")
			process, err := os.FindProcess(pid)
			if err != nil {
				fmt.Printf("Error finding process: %v\n", err)
			} else {
				process.Signal(os.Interrupt)
			}
			fmt.Println("Waiting for daemon to stop...")
		}
		fmt.Println("Starting Daemon...")
		fmt.Println("Please run 'machinus daemon start' to start the daemon")
	},
}

var daemonLogsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Show Daemon Logs",
	Run: func(cmd *cobra.Command, args []string) {
		logPath := joinPath(expandPath(logDir), "agent.log")
		data, err := os.ReadFile(logPath)
		if err != nil {
			fmt.Printf("Error reading log file: %v\n", err)
			os.Exit(1)
		}
		fmt.Print(string(data))
	},
}

func expandPath(path string) string {
	if len(path) > 0 && path[0] == '~' {
		home, _ := os.UserHomeDir()
		return joinPath(home, path[1:])
	}
	return path
}

func joinPath(base, name string) string {
	if len(base) > 0 && base[len(base)-1] == filepath.Separator {
		return base + name
	}
	return base + string(filepath.Separator) + name
}

func init() {
	rootCmd.AddCommand(daemonCmd)
	daemonCmd.AddCommand(daemonStartCmd)
	daemonCmd.AddCommand(daemonStopCmd)
	daemonCmd.AddCommand(daemonRestartCmd)
	daemonCmd.AddCommand(daemonStatusCmd)
	daemonCmd.AddCommand(daemonLogsCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

