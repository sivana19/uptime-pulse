package monitor

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestUptimePercentage(t *testing.T) {
	tests := []struct {
		name           string
		totalChecks    int
		successfulHits int
		expectedUptime float64
	}{
		{"No checks executed", 0, 0, 0.0},
		{"100% availability", 10, 10, 100.0},
		{"50% availability", 4, 2, 50.0},
		{"Zero availability", 5, 0, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ep := Endpoint{
				TotalChecks:    tt.totalChecks,
				SuccessfulHits: tt.successfulHits,
			}
			got := ep.UptimePercentage()
			if got != tt.expectedUptime {
				t.Errorf("expected %.2f%%, got %.2f%%", tt.expectedUptime, got)
			}
		})
	}
}

func TestScannerCheckSingleTarget(t *testing.T) {
	// Spin up local mock HTTP test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ok":
			w.WriteHeader(http.StatusOK)
		case "/fail":
			w.WriteHeader(http.StatusInternalServerError)
		case "/timeout":
			time.Sleep(50 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	scanner := NewScanner(25 * time.Millisecond) // Short timeout for testing

	tests := []struct {
		name           string
		path           string
		expectedStatus int
		expectUp       bool
	}{
		{"200 OK Response", "/ok", http.StatusOK, true},
		{"500 Server Error", "/fail", http.StatusInternalServerError, false},
		{"404 Not Found", "/unknown", http.StatusNotFound, false},
		{"Timeout Failure", "/timeout", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			targetURL := server.URL + tt.path
			result := scanner.CheckSingleTarget(targetURL)

			if result.IsUp != tt.expectUp {
				t.Errorf("expected IsUp=%v, got %v", tt.expectUp, result.IsUp)
			}
			if result.StatusCode != tt.expectedStatus {
				t.Errorf("expected StatusCode=%d, got %d", tt.expectedStatus, result.StatusCode)
			}
		})
	}
}
