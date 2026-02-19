package scheduler

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// CronExpr represents a parsed cron expression (minute, hour, day-of-month, month, day-of-week).
type CronExpr struct {
	Minutes    []int
	Hours      []int
	DaysOfMonth []int
	Months     []int
	DaysOfWeek []int
}

// ParseCron parses a standard 5-field cron expression.
func ParseCron(expr string) (*CronExpr, error) {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return nil, fmt.Errorf("cron expression must have 5 fields, got %d", len(fields))
	}

	minutes, err := parseField(fields[0], 0, 59)
	if err != nil {
		return nil, fmt.Errorf("minute field: %w", err)
	}
	hours, err := parseField(fields[1], 0, 23)
	if err != nil {
		return nil, fmt.Errorf("hour field: %w", err)
	}
	doms, err := parseField(fields[2], 1, 31)
	if err != nil {
		return nil, fmt.Errorf("day-of-month field: %w", err)
	}
	months, err := parseField(fields[3], 1, 12)
	if err != nil {
		return nil, fmt.Errorf("month field: %w", err)
	}
	dows, err := parseField(fields[4], 0, 6)
	if err != nil {
		return nil, fmt.Errorf("day-of-week field: %w", err)
	}

	return &CronExpr{
		Minutes:     minutes,
		Hours:       hours,
		DaysOfMonth: doms,
		Months:      months,
		DaysOfWeek:  dows,
	}, nil
}

// Matches returns true if the given time matches the cron expression.
func (c *CronExpr) Matches(t time.Time) bool {
	return contains(c.Minutes, t.Minute()) &&
		contains(c.Hours, t.Hour()) &&
		contains(c.DaysOfMonth, t.Day()) &&
		contains(c.Months, int(t.Month())) &&
		contains(c.DaysOfWeek, int(t.Weekday()))
}

func contains(vals []int, v int) bool {
	for _, val := range vals {
		if val == v {
			return true
		}
	}
	return false
}

// parseField parses a single cron field (supports *, */n, n, n-m, n-m/s, comma-separated).
func parseField(field string, min, max int) ([]int, error) {
	var result []int

	for _, part := range strings.Split(field, ",") {
		vals, err := parsePart(part, min, max)
		if err != nil {
			return nil, err
		}
		result = append(result, vals...)
	}

	return result, nil
}

func parsePart(part string, min, max int) ([]int, error) {
	// Handle */n
	if strings.HasPrefix(part, "*/") {
		step, err := strconv.Atoi(part[2:])
		if err != nil || step <= 0 {
			return nil, fmt.Errorf("invalid step: %s", part)
		}
		var vals []int
		for i := min; i <= max; i += step {
			vals = append(vals, i)
		}
		return vals, nil
	}

	// Handle *
	if part == "*" {
		var vals []int
		for i := min; i <= max; i++ {
			vals = append(vals, i)
		}
		return vals, nil
	}

	// Handle n-m or n-m/s
	if strings.Contains(part, "-") {
		rangeParts := strings.SplitN(part, "/", 2)
		bounds := strings.SplitN(rangeParts[0], "-", 2)
		if len(bounds) != 2 {
			return nil, fmt.Errorf("invalid range: %s", part)
		}
		lo, err := strconv.Atoi(bounds[0])
		if err != nil {
			return nil, fmt.Errorf("invalid range start: %s", bounds[0])
		}
		hi, err := strconv.Atoi(bounds[1])
		if err != nil {
			return nil, fmt.Errorf("invalid range end: %s", bounds[1])
		}
		step := 1
		if len(rangeParts) == 2 {
			step, err = strconv.Atoi(rangeParts[1])
			if err != nil || step <= 0 {
				return nil, fmt.Errorf("invalid step: %s", rangeParts[1])
			}
		}
		var vals []int
		for i := lo; i <= hi; i += step {
			vals = append(vals, i)
		}
		return vals, nil
	}

	// Single value
	val, err := strconv.Atoi(part)
	if err != nil {
		return nil, fmt.Errorf("invalid value: %s", part)
	}
	if val < min || val > max {
		return nil, fmt.Errorf("value %d out of range %d-%d", val, min, max)
	}
	return []int{val}, nil
}
