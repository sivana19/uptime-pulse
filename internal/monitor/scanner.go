package monitor

import (
	"net/http"
	"sync"
	"time"
)

type Scanner struct {
	client *http.Client
}

func NewScanner(timeout time.Duration) *Scanner {
	return &Scanner{
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// CheckSingleTarget pings a single URL and builds a CheckResult
func (s *Scanner) CheckSingleTarget(targetURL string) CheckResult {
	start := time.Now()
	resp, err := s.client.Get(targetURL)
	latency := time.Since(start)

	if err != nil {
		targetErr := &TargetCheckError{URL: targetURL, Err: err}
		return CheckResult{
			URL:          targetURL,
			StatusCode:   0,
			ResponseTime: latency,
			IsUp:         false,
			CheckedAt:    time.Now().UTC(),
			ErrorMsg:     targetErr.Error(),
		}
	}
	defer resp.Body.Close()

	isUp := resp.StatusCode >= 200 && resp.StatusCode < 400
	var errorMsg string
	if !isUp {
		targetErr := &TargetCheckError{URL: targetURL, StatusCode: resp.StatusCode}
		errorMsg = targetErr.Error()
	}

	return CheckResult{
		URL:          targetURL,
		StatusCode:   resp.StatusCode,
		ResponseTime: latency,
		IsUp:         isUp,
		CheckedAt:    time.Now().UTC(),
		ErrorMsg:     errorMsg,
	}
}

// ScanAllConcurrent fires concurrent requests and collects results through a buffered channel
func (s *Scanner) ScanAllConcurrent(urls []string) []CheckResult {
	var wg sync.WaitGroup
	resultChan := make(chan CheckResult, len(urls))

	for _, target := range urls {
		wg.Add(1)
		go func(u string) {
			defer wg.Done()
			resultChan <- s.CheckSingleTarget(u)
		}(target)
	}

	// Wait for all goroutines to finish in a separate thread, then close the channel
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	var results []CheckResult
	for res := range resultChan {
		results = append(results, res)
	}

	return results
}
