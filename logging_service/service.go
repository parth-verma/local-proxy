package logging_service

import (
	"changeme/db_service"
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	_ "modernc.org/sqlite"
)

// LogRequest represents a request to be logged
type LogRequest struct {
	Host     string
	Method   string
	Path     string
	Port     int
	Approved bool
	Duration int64
}

// LoggingService manages SQLite database operations for request logging
type LoggingService struct {
	DbService *db_service.DatabaseService
	ctx       context.Context
	options   application.ServiceOptions

	// Channel-based logging
	logChannel chan LogRequest
	stopChan   chan struct{}
	wg         sync.WaitGroup
}

// singleton instance for easy access from other services
var instance *LoggingService

func Instance() *LoggingService { return instance }

func (l *LoggingService) ServiceName() string {
	return "logging_service"
}

// ServiceStartup is called when the app is starting up
func (l *LoggingService) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	if instance != nil {
		log.Printf("Logging Service already started")
		return nil
	}
	instance = l
	l.ctx = ctx
	l.options = options

	// Initialize channels
	l.logChannel = make(chan LogRequest, 1000) // Buffer for 1000 pending requests
	l.stopChan = make(chan struct{})

	if err := l.initDB(); err != nil {
		log.Printf("Logging Service init error: %v", err)
		return err
	}

	// Start the consumer goroutine
	l.startConsumer()

	return nil
}

// ServiceShutdown is called when the app is shutting down
func (l *LoggingService) ServiceShutdown() error {
	// Signal the consumer to stop
	close(l.stopChan)

	// Wait for the consumer to finish processing
	l.wg.Wait()

	// Close the log channel
	close(l.logChannel)

	return nil
}

// initDB opens/creates the SQLite database in the user's data home directory
func (l *LoggingService) initDB() error {
	db := l.DbService.Db
	if db == nil {
		return fmt.Errorf("database not initialized")
	}

	// Create the requests table if it doesn't exist
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS requests (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp INTEGER NOT NULL,
		host TEXT NOT NULL,
		method TEXT NOT NULL,
		path TEXT NOT NULL,
		port INTEGER NOT NULL,
		decision TEXT NOT NULL,
		duration REAL NOT NULL
	);
	
	CREATE INDEX IF NOT EXISTS idx_requests_timestamp ON requests(timestamp);
	CREATE INDEX IF NOT EXISTS idx_requests_decision ON requests(decision);
	CREATE INDEX IF NOT EXISTS idx_requests_host ON requests(host);
	`

	if _, err := db.Exec(createTableSQL); err != nil {
		db.Close()
		return fmt.Errorf("failed to create table: %w", err)
	}

	return nil
}

// startConsumer starts the background goroutine that processes log requests
func (l *LoggingService) startConsumer() {
	l.wg.Add(1)
	go func() {
		defer l.wg.Done()

		for {
			select {
			case logReq := <-l.logChannel:
				l.processLogRequest(logReq)
			case <-l.stopChan:
				// Process any remaining requests in the channel
				for {
					select {
					case logReq := <-l.logChannel:
						l.processLogRequest(logReq)
					default:
						return
					}
				}
			}
		}
	}()
}

// processLogRequest processes a single log request and writes it to the database
func (l *LoggingService) processLogRequest(logReq LogRequest) {
	if l.DbService.Db == nil {
		log.Printf("database not available for logging")
		return
	}

	decision := "rejected"
	if logReq.Approved {
		decision = "approved"
	}

	timestamp := time.Now().UnixMilli()

	_, err := l.DbService.Db.Exec(`
		INSERT INTO requests (timestamp, host, method, path, port, decision, duration)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, timestamp, logReq.Host, strings.ToUpper(logReq.Method), logReq.Path, logReq.Port, decision, float64(logReq.Duration))

	if err != nil {
		log.Printf("warning: failed to write request log: %v", err)
	}
}

// LogRequest sends a request to be logged via the channel (non-blocking)
func (l *LoggingService) LogRequest(host, method, path string, port int, approved bool, duration int64) {
	if l == nil || l.logChannel == nil {
		log.Printf("logging service not ready")
		return
	}

	// Create log request struct
	logReq := LogRequest{
		Host:     host,
		Method:   method,
		Path:     path,
		Port:     port,
		Approved: approved,
		Duration: duration,
	}

	// Send to channel (non-blocking with buffer)
	select {
	case l.logChannel <- logReq:
		// Successfully queued for processing
	default:
		// Channel is full, log a warning but don't block
		log.Printf("warning: log channel full, dropping request for %s", host)
	}
}

// DashboardData represents aggregated data for the dashboard
type DashboardData struct {
	TimeRange     string           `json:"timeRange"`
	TotalRequests int64            `json:"totalRequests"`
	ApprovedCount int64            `json:"approvedCount"`
	RejectedCount int64            `json:"rejectedCount"`
	Connections   []ConnectionData `json:"connections"`
	Requests      []RequestDetail  `json:"requests"`
}

// ConnectionData represents connection count over time
type ConnectionData struct {
	Timestamp int64 `json:"timestamp"`
	Count     int64 `json:"count"`
	Approved  int64 `json:"approved"`
	Rejected  int64 `json:"rejected"`
}

// RequestDetail represents a detailed request entry
type RequestDetail struct {
	Timestamp int64   `json:"timestamp"`
	Host      string  `json:"host"`
	Method    string  `json:"method"`
	Path      string  `json:"path"`
	Port      int     `json:"port"`
	Decision  string  `json:"decision"`
	Duration  float64 `json:"duration"`
}

// GetDashboardData retrieves dashboard data for the specified time range
func (l *LoggingService) GetDashboardData(timeRange string) (*DashboardData, error) {
	if l == nil || l.DbService.Db == nil {
		return nil, fmt.Errorf("logging service not ready")
	}

	// Calculate time boundaries based on range
	now := time.Now()
	var startTime time.Time

	switch timeRange {
	case "1h":
		startTime = now.Add(-1 * time.Hour)
	case "6h":
		startTime = now.Add(-6 * time.Hour)
	case "24h":
		startTime = now.Add(-24 * time.Hour)
	case "7d":
		startTime = now.Add(-7 * 24 * time.Hour)
	case "30d":
		startTime = now.Add(-30 * 24 * time.Hour)
	default:
		startTime = now.Add(-24 * time.Hour) // Default to 24h
	}

	startTimestamp := startTime.UnixMilli()
	endTimestamp := now.UnixMilli()

	// Query all requests in the time range
	rows, err := l.DbService.Db.Query(`
		SELECT timestamp, host, method, path, port, decision, duration
		FROM requests
		WHERE timestamp >= ? AND timestamp <= ?
		ORDER BY timestamp DESC
	`, startTimestamp, endTimestamp)

	if err != nil {
		return nil, fmt.Errorf("failed to query data: %w", err)
	}
	defer rows.Close()

	// Process data
	var connections []ConnectionData
	var requests []RequestDetail
	var totalRequests, approvedCount, rejectedCount int64

	// Group by time intervals for connection chart
	intervalMinutes := getIntervalMinutes(timeRange)
	timeGroups := make(map[int64]*ConnectionData)

	for rows.Next() {
		var timestamp int64
		var host, method, path, decision string
		var port int
		var duration float64

		if err := rows.Scan(&timestamp, &host, &method, &path, &port, &decision, &duration); err != nil {
			log.Printf("warning: failed to scan row: %v", err)
			continue
		}

		// Calculate which time interval this point belongs to
		intervalTime := (timestamp / (int64(intervalMinutes) * 60 * 1000)) * (int64(intervalMinutes) * 60 * 1000)

		if timeGroups[intervalTime] == nil {
			timeGroups[intervalTime] = &ConnectionData{
				Timestamp: intervalTime,
				Count:     0,
				Approved:  0,
				Rejected:  0,
			}
		}

		// Update connection data
		timeGroups[intervalTime].Count++
		if decision == "approved" {
			timeGroups[intervalTime].Approved++
			approvedCount++
		} else {
			timeGroups[intervalTime].Rejected++
			rejectedCount++
		}
		totalRequests++

		// Add to requests detail
		requests = append(requests, RequestDetail{
			Timestamp: timestamp,
			Host:      host,
			Method:    method,
			Path:      path,
			Port:      port,
			Decision:  decision,
			Duration:  duration,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	// Convert time groups to slice and sort by timestamp
	for _, conn := range timeGroups {
		connections = append(connections, *conn)
	}

	// Sort connections by timestamp
	for i := 0; i < len(connections)-1; i++ {
		for j := i + 1; j < len(connections); j++ {
			if connections[i].Timestamp > connections[j].Timestamp {
				connections[i], connections[j] = connections[j], connections[i]
			}
		}
	}

	return &DashboardData{
		TimeRange:     timeRange,
		TotalRequests: totalRequests,
		ApprovedCount: approvedCount,
		RejectedCount: rejectedCount,
		Connections:   connections,
		Requests:      requests,
	}, nil
}

// getIntervalMinutes returns the appropriate interval in minutes for the given time range
func getIntervalMinutes(timeRange string) int {
	switch timeRange {
	case "1h":
		return 5 // 5-minute intervals for 1 hour
	case "6h":
		return 30 // 30-minute intervals for 6 hours
	case "24h":
		return 60 // 1-hour intervals for 24 hours
	case "7d":
		return 360 // 6-hour intervals for 7 days
	case "30d":
		return 1440 // 1-day intervals for 30 days
	default:
		return 60 // Default to 1-hour intervals
	}
}
