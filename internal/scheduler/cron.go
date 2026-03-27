package scheduler

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// CronExpressions -> Represents a parsed cron expression.
type CronExpression struct {
	Minute  []bool
	Hour 	[]bool
	Day 	[]bool
	Month 	[]bool
	Weekday []bool
}

// Parse(): parses a cron expressions: "minute hour day month weekday"
// Examples ->
// 		"0 9 * * *"		- Every day at 9:00 AM
// 		"*/30 * * * *"	- Every 30 Minutes
// 		"0 2 * * 0"		- At 2:00 AM Every Sunday
// 		"0 0 1 * *" 	- At midnight on the 1st of every month
func Parse(cronExpression string) (*CronExpression, error) {
	fields := strings.Fields(cronExpression)
	if len(fields) != 5 { 
		return nil, fmt.Errorf("invalid cron format: expected 5 fields got %d", len(fields))
	}
	expression := &CronExpression{}
	var err error
	expression.Minute, err = parseField(fields[0], 0, 59)
	if err != nil {
		return nil, fmt.Errorf("minute: %w", err)
	}
	expression.Hour, err = parseField(fields[1], 0, 23)
	if err != nil {
		return nil, fmt.Errorf("hour: %w", err)
	}
	expression.Day, err = parseField(fields[2], 1, 31)
	if err != nil {
		return nil, fmt.Errorf("day: %w", err)
	}
	expression.Month, err = parseField(fields[3], 1, 12)
	if err != nil {
		return nil, fmt.Errorf("month: %w", err)
	}
	expression.Weekday, err = parseField(fields[4], 0, 6)
	if err != nil {
		return nil, fmt.Errorf("weekday: %w", err)
	}
	return expression, nil
}

// parseField(): parses a single cron field.
//     -> Supports wildcard(*), ranges(1-5), lists(1,3,5), steps (*/5, 1-10/2)
func parseField(field string, min, max int) ([]bool, error) {
	result := make([]bool, max-min+1)

	// Handle wildcard "*"
	if field == "*" {
		for i := range result {
			result[i] = true
		}
		return result, nil
	}

	// Handle step: "*/5" or "1-10/2"
	if strings.Contains(field, "/") {
		parts := strings.Split(field, "/")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid step format: %s", field)
		}
		step, err := strconv.Atoi(parts[1])
		if err != nil || step <= 0 {
			return nil, fmt.Errorf("invalid step value: %s", parts[1])
		}
		// Get base range before step
		var base string
		if strings.HasPrefix(parts[0], "*/") {
			base = fmt.Sprintf("%d-%d", min, max)
		} else {
			base = parts[0]
		}
		baseResult, err := parseField(base, min, max)
		if err != nil {
			return nil, err
		}

		// Apply Step
		count := 0
		for i, enabled := range baseResult {
			if enabled {
				if count%step == 0 {
					result[i] = true
				}
				count++
			}
		}
		return result, nil
	}
	// Handle range: "1-5"
	if strings.Contains(field, "-") {
		parts := strings.Split(field, "-")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid range format: %s", field)
		}
		start, err := strconv.Atoi(parts[0])
		if err != nil {
			return nil, fmt.Errorf("inavlid range start: %s", parts[0])
		}
		end, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil, fmt.Errorf("invalid range end: %s", parts[1])
		}
		if start < min || start > max || end < min || end > max {
			return nil, fmt.Errorf("range out of bounds: %s", field)
		}
		for i := start; i<= end; i ++ {
			if i >= min && i <= max {
				result[i-min] = true
			}
		}
		return result, nil
	}

	// Handle List: "1,3,5"
	if strings.Contains(field, ",") {
		parts := strings.Split(field, ",")
		for _, part := range parts {
			val, err := strconv.Atoi(strings.TrimSpace(part))
			if err != nil {
				return nil, fmt.Errorf("invalid value in list: %s", part)
			}
			if val < min || val > max {
				return nil, fmt.Errorf("value %d out of range [%d, %d]", val, min, max)
			}
			result[val-min] = true
		}
		return result, nil
	}

	// Handle Single Numbers: "5"
	val, err := strconv.Atoi(field)
	if err != nil {
		return nil, fmt.Errorf("invalid value: %s", field)
	}
	if val < min || val > max {
		return nil, fmt.Errorf("value %d out of range [%d, %d]", val, min, max)
	}
	result[val-min] = true
	return result, nil
}


// GetNextRun calculates the next run time after the given time .
func (c *CronExpression) GetNextRun(after time.Time) time.Time {
	// Start from the next minute
	t := after.Add(1 * time.Minute)
	t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), 0,0, after.Location())

	// Try to find the next valid time (with safety limit of ~4 years)
	maxIterations := 4 * 365 * 24 * 60
	for i := 0; i < maxIterations; i++ {
		if c.matches(t) {
			return t
		}
		t = t.Add(1 * time.Minute)
	}
	// Should never happen with valid cron expressions
	return time.Time{}
}

// matches(): checks if the given time matches the cron expression
func (c *CronExpression) matches(t time.Time) bool {
	return c.Minute[t.Minute()] &&
	c.Hour[t.Hour()] &&
	c.Day[t.Day() - 1 ] &&
	c.Month[int(t.Month()) - 1] &&
	c.Weekday[int(t.Weekday())]
}

// IsValid(): checks if a cron expression string is valid
func IsValid(cronExpression string) bool {
	_, err := Parse(cronExpression)
	return err == nil
}