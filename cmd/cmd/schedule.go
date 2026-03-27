package main

import (
	"fmt"
	"os"

	"github.com/machinus/cloud-agent/internal/scheduler"
	"github.com/spf13/cobra"
)

const (
	scheduleFile = "~/.machinus/schedule.yml"
)

var scheduleCmd = &cobra.Command{
	Use: "schedule",
	Short: "Manage Scheduled Tasks.",
	Long: `Manage scheduled tasks for autonomous execution`,
	Run: func(cmd *cobra.Command, args []string){
		cmd.Help()
	},
}

var scheduleAddCmd = &cobra.Command{
	Use: "add",
	Short: "Add a new scheduled task.",
	Run: func(cmd *cobra.Command, args []string){
		name, _ := cmd.Flags().GetString("name")
		cronExpression, _ := cmd.Flags().GetString("cron")
		task, _ := cmd.Flags().GetString("task")

		if name == "" || cronExpression == "" || task == "" {
            fmt.Println("Error: --name, --cron, and --task are required")
            os.Exit(1)
        }

		// Create scheduler
		sched := scheduler.New(scheduler.Config{
			ScheduleFile: scheduleFile,
			Agent:       nil,
		})

		// Load existing tasks (creates directory if needed)
		if err := sched.Load(); err != nil {
			fmt.Printf("Error loading schedule: %v\n", err)
			os.Exit(1)
		}

		newTask := &scheduler.ScheduledTask{
			Name:    name,
			Cron:    cronExpression,
			Task:    task,
			Enabled: true,
		}
		if err := sched.Add(newTask); err != nil {
			fmt.Printf("Error Adding Task: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Task '%s' added successfully\n", name)        
        fmt.Printf("Next run: %s\n", newTask.NextRun.Format("2006-01-02 15:04:05"))
	},
}

var scheduleListCmd = &cobra.Command{
	Use: "list",
	Short: "List all scheduled tasks",
	Run: func(cmd *cobra.Command, args []string){
		schedule := scheduler.New(scheduler.Config{
			ScheduleFile: scheduleFile,
			Agent: 		  nil, 
		})
		if err := schedule.Load(); err != nil {
			fmt.Printf("Error loading schedule: %v\n", err)
			os.Exit(1)
		}
		tasks := schedule.List()
		if len(tasks) == 0 {
			fmt.Printf("No Scheduled Tasks")
			return
		}
		fmt.Println("\nScheduled Tasks:")
        fmt.Println("─────────────────────────────────────────────────────────────")
        for _, t := range tasks {
            status := "✓ Enabled"
            if !t.Enabled {
                status = "✗ Disabled"
            }

            fmt.Printf("\n  Name:      %s\n", t.Name)
            fmt.Printf("  Schedule:  %s\n", t.Cron)
            fmt.Printf("  Task:      %s\n", t.Task)
            fmt.Printf("  Status:    %s\n", status)
            fmt.Printf("  Next Run:  %s\n", t.NextRun.Format("2006-01-02 15:04:05"))
            if !t.LastRun.IsZero() {
                fmt.Printf("  Last Run:  %s\n", t.LastRun.Format("2006-01-02 15:04:05"))
            }
        }
        fmt.Println()
	},
}

var scheduleRemoveCmd = &cobra.Command{
	Use: "remove <id>",
	Short: "Remove a scheduled task",
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		schedule := scheduler.New(scheduler.Config{
			ScheduleFile: scheduleFile,
			Agent: 		  nil,
		})
		if err := schedule.Remove(id); err != nil {
			fmt.Printf("Error removing task: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Task '%s' removed\n", id)
	},
}

var scheduleEnableCmd = &cobra.Command{
	Use: "enable <id>",
	Short: "Enable a scheduled task",
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		schedule := scheduler.New(scheduler.Config{
			ScheduleFile: 	scheduleFile,
			Agent: 			nil,
		})
		if err := schedule.Enable(id); err != nil {
			fmt.Printf("Error enabling task: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Task '%s' enabled\n", id)
	},
}

var scheduleDisableCmd = &cobra.Command{
	Use: "disable <id>",
	Short: "Disable a scheduled task",
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		schedule := scheduler.New(scheduler.Config{
			ScheduleFile: scheduleFile,
			Agent: 		   nil, 
		})
		if err := schedule.Disable(id); err != nil {
			fmt.Printf("Error disabling task: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Task '%s' disabled\n", id)
	},
}

func init(){
	// Add Flags
	scheduleAddCmd.Flags().String("name", "", "Task name")
	scheduleAddCmd.Flags().String("cron", "", "Cron expression (e.g., '0 0 * * *')")
	scheduleAddCmd.Flags().String("task", "", "Task description")

	// Register Commands
	rootCmd.AddCommand(scheduleCmd)
	scheduleCmd.AddCommand(scheduleAddCmd)
	scheduleCmd.AddCommand(scheduleListCmd)
	scheduleCmd.AddCommand(scheduleRemoveCmd)
	scheduleCmd.AddCommand(scheduleEnableCmd)
	scheduleCmd.AddCommand(scheduleDisableCmd)
}