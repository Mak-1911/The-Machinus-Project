package scheduler

import "time"

type ScheduledTask struct {
	ID 			string 			`yaml:"id" json:"id"`
	Name 		string 			`yaml:"name" json:"name"`
	Cron 		string 			`yaml:"cron" json:"cron"`
	Task 		string 			`yaml:"task" json:"task"`
	Enabled 	bool 			`yaml:"enabled" json:"enabled"`
	NextRun 	time.Time 		`yaml:"next_run" json:"next_run,omitempty"`
	LastRun   	time.Time `yaml:"last_run,omitempty" json:"last_run,omitempty"`
	CreatedAt 	time.Time 		`yaml:"created_at" json:"created_at"`
}

// ScheduleConfig: Holds the scheduler configuration (for YAML file)
type ScheduleConfig struct {
	Schedule []*ScheduledTask 	`yaml:"schedule" json:"schedule"`
}