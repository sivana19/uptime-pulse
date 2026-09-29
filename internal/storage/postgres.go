package storage

import (
	"database/sql"
	"time"

	"uptime-pulse/internal/monitor"
)

type Storage struct {
	db *sql.DB
}

func NewStorage(db *sql.DB) *Storage {
	return &Storage{db: db}
}

func (s *Storage) InitSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS monitors (
		id SERIAL PRIMARY KEY,
		url TEXT UNIQUE NOT NULL,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS scan_history (
		id SERIAL PRIMARY KEY,
		monitor_id INT REFERENCES monitors(id) ON DELETE CASCADE,
		status_code INT NOT NULL,
		response_time_ms BIGINT NOT NULL,
		is_up BOOLEAN NOT NULL,
		checked_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);`
	_, err := s.db.Exec(schema)
	return err
}

func (s *Storage) CreateMonitor(url string) (int64, error) {
	var id int64
	query := `INSERT INTO monitors (url) VALUES ($1) ON CONFLICT (url) DO UPDATE SET url=EXCLUDED.url RETURNING id;`
	err := s.db.QueryRow(query, url).Scan(&id)
	return id, err
}

func (s *Storage) GetAllMonitors() ([]monitor.Endpoint, error) {
	rows, err := s.db.Query(`SELECT id, url FROM monitors ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []monitor.Endpoint
	for rows.Next() {
		var ep monitor.Endpoint
		if err := rows.Scan(&ep.ID, &ep.URL); err != nil {
			return nil, err
		}
		list = append(list, ep)
	}
	return list, nil
}

func (s *Storage) SaveCheckResult(monitorID int64, res monitor.CheckResult) error {
	query := `INSERT INTO scan_history (monitor_id, status_code, response_time_ms, is_up, checked_at)
	          VALUES ($1, $2, $3, $4, $5);`
	_, err := s.db.Exec(query, monitorID, res.StatusCode, res.ResponseTime.Milliseconds(), res.IsUp, res.CheckedAt)
	return err
}

type MetricSummary struct {
	MonitorID        int64                 `json:"monitor_id"`
	UptimePercentage float64               `json:"uptime_percentage"`
	RecentScans      []monitor.CheckResult `json:"recent_scans"`
}

func (s *Storage) GetMetrics(monitorID int64) (*MetricSummary, error) {
	rows, err := s.db.Query(`
		SELECT status_code, response_time_ms, is_up, checked_at 
		FROM scan_history 
		WHERE monitor_id = $1 
		ORDER BY checked_at DESC LIMIT 50`, monitorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	total := 0
	upCount := 0
	var recent []monitor.CheckResult

	for rows.Next() {
		var sc int
		var ms int64
		var isUp bool
		var checkedAt time.Time

		if err := rows.Scan(&sc, &ms, &isUp, &checkedAt); err != nil {
			return nil, err
		}
		total++
		if isUp {
			upCount++
		}
		recent = append(recent, monitor.CheckResult{
			StatusCode:   sc,
			ResponseTime: time.Duration(ms) * time.Millisecond,
			IsUp:         isUp,
			CheckedAt:    checkedAt,
		})
	}

	uptimePct := 0.0
	if total > 0 {
		uptimePct = (float64(upCount) / float64(total)) * 100.0
	}

	return &MetricSummary{
		MonitorID:        monitorID,
		UptimePercentage: uptimePct,
		RecentScans:      recent,
	}, nil
}
