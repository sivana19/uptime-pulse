package monitor

import (
	"fmt"
	"time"
)

// Custom error for failed target checks
type TargetCheckError struct {
	URL        string
	StatusCode int
	Err        error
}

func (e *TargetCheckError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("target check failed for [%s]: %v", e.URL, e.Err)
	}
	return fmt.Sprintf("target check failed for [%s]: received status %d", e.URL, e.StatusCode)
}

type CheckResult struct {
	URL          string        `json:"url"`
	StatusCode   int           `json:"status_code"`
	ResponseTime time.Duration `json:"response_time"`
	IsUp         bool          `json:"is_up"`
	CheckedAt    time.Time     `json:"checked_at"`
	ErrorMsg     string        `json:"error_msg,omitempty"`
}

type Endpoint struct {
	ID             int64         `json:"id"`
	URL            string        `json:"url"`
	TotalChecks    int           `json:"total_checks"`
	SuccessfulHits int           `json:"successful_hits"`
	LastLatency    time.Duration `json:"last_latency"`
}

// RecordCheck uses a pointer receiver to mutate the Endpoint state in-place
func (e *Endpoint) RecordCheck(result CheckResult) {
	e.TotalChecks++
	e.LastLatency = result.ResponseTime
	if result.IsUp {
		e.SuccessfulHits++
	}
}

// UptimePercentage calculates the availability ratio
func (e *Endpoint) UptimePercentage() float64 {
	if e.TotalChecks == 0 {
		return 0.0
	}
	return (float64(e.SuccessfulHits) / float64(e.TotalChecks)) * 100.0
}
