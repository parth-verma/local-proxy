package db_service

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"
	_ "modernc.org/sqlite"
)

// DatabaseService manages all database operations (SQLite + time-series store)
// and exposes simple methods the rest of the app can use.
type DatabaseService struct {
	ctx     context.Context
	options application.ServiceOptions

	Db     *sql.DB
	dbPath string
}

// singleton instance for easy access from other services
var instance *DatabaseService

func Instance() *DatabaseService { return instance }

// NewDBService creates a new DatabaseService instance with dependency injection
func NewDBService(baseDir string) (*DatabaseService, error) {
	service := &DatabaseService{}

	// Create the database directory
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create data dir: %w", err)
	}

	// SQLite for blocked domains
	sqlitePath := filepath.Join(baseDir, "local-proxy.db")
	db, err := sql.Open("sqlite", sqlitePath+"?_journal_mode=WAL&_busy_timeout=500&_synchronous=NORMAL&_txlock=deferred")
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite db: %w", err)
	}
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		log.Printf("warning: failed to set busy_timeout: %v", err)
	}

	// Register regex function
	if err := service.registerRegexFunction(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to register regex function: %w", err)
	}

	createStmt := `CREATE TABLE IF NOT EXISTS blocked_domains (
		domain TEXT PRIMARY KEY,
		filter_type TEXT DEFAULT 'exact' CHECK(filter_type IN ('exact', 'glob', 'regex')),
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`
	if _, err := db.Exec(createStmt); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to create blocked_domains: %w", err)
	}

	service.Db = db
	service.dbPath = sqlitePath

	return service, nil
}

func (d *DatabaseService) ServiceName() string { return "db_service" }

func (d *DatabaseService) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	if instance != nil {
		log.Printf("DB Service already started")
		return nil
	}
	instance = d
	d.ctx = ctx
	d.options = options

	// Initialize database using the same logic as NewDBService
	if err := d.initDBs(); err != nil {
		log.Printf("DB Service init error: %v", err)
	}
	return nil
}

func (d *DatabaseService) ServiceShutdown() error {
	if d.Db != nil {
		if err := d.Db.Close(); err != nil {
			log.Printf("Warning: failed to close SQLite DB: %v", err)
		}
	}
	return nil
}

func (d *DatabaseService) initDBs() error {
	dataDir := application.Path(application.PathDataHome)
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return fmt.Errorf("failed to create data dir: %w", err)
	}

	// SQLite for blocked domains
	sqlitePath := filepath.Join(dataDir, "local-proxy", "local-proxy.db")
	db, err := sql.Open("sqlite", sqlitePath+"?_journal_mode=WAL&_busy_timeout=500&_synchronous=NORMAL&_txlock=deferred")
	if err != nil {
		return fmt.Errorf("failed to open sqlite db: %w", err)
	}
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		log.Printf("warning: failed to set busy_timeout: %v", err)
	}

	// Register regex function
	if err := d.registerRegexFunction(db); err != nil {
		_ = db.Close()
		return fmt.Errorf("failed to register regex function: %w", err)
	}
	createStmt := `CREATE TABLE IF NOT EXISTS blocked_domains (
		domain TEXT PRIMARY KEY,
		filter_type TEXT DEFAULT 'exact' CHECK(filter_type IN ('exact', 'glob', 'regex')),
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`
	if _, err := db.Exec(createStmt); err != nil {
		_ = db.Close()
		return fmt.Errorf("failed to create blocked_domains: %w", err)
	}

	d.Db = db
	d.dbPath = sqlitePath

	return nil
}

func regex(re, s string) (bool, error) {
	return regexp.MatchString(re, s)
}

// registerRegexFunction registers a custom regex function with SQLite
func (d *DatabaseService) registerRegexFunction(db *sql.DB) error {
	// For modernc.org/sqlite, we need to use a different approach
	// Since the driver doesn't support custom functions directly,
	// we'll implement regex matching in Go code in the IsDomainBlocked method
	return nil
}

// IsDomainBlocked checks exact, glob, or regex patterns case-insensitively.
func (d *DatabaseService) IsDomainBlocked(domain string) bool {
	if d == nil || d.Db == nil {
		return false
	}
	domain = strings.ToLower(strings.TrimSpace(domain))
	if domain == "" {
		return false
	}

	// Get all blocked patterns with their filter types
	rows, err := d.Db.Query(`SELECT domain, filter_type FROM blocked_domains`)
	if err != nil {
		log.Printf("DB error querying blocked domains: %v", err)
		return false
	}
	defer rows.Close()

	for rows.Next() {
		var pattern, filterType string
		if err := rows.Scan(&pattern, &filterType); err != nil {
			log.Printf("DB error scanning blocked domain: %v", err)
			continue
		}

		// Check based on filter type
		switch filterType {
		case "exact":
			if domain == strings.ToLower(pattern) {
				return true
			}
		case "glob":
			// Use SQLite GLOB for pattern matching
			if matched, _ := d.matchGlob(domain, strings.ToLower(pattern)); matched {
				return true
			}
		case "regex":
			// Use Go regex for pattern matching
			if matched, _ := d.matchRegex(domain, pattern); matched {
				return true
			}
		}
	}

	return false
}

// matchGlob performs glob pattern matching (similar to SQLite GLOB)
func (d *DatabaseService) matchGlob(text, pattern string) (bool, error) {
	// Convert glob pattern to regex
	regexPattern := strings.ReplaceAll(pattern, "*", ".*")
	regexPattern = strings.ReplaceAll(regexPattern, "?", ".")
	regexPattern = "^" + regexPattern + "$"

	matched, err := regexp.MatchString(regexPattern, text)
	return matched, err
}

// matchRegex performs regex pattern matching
func (d *DatabaseService) matchRegex(text, pattern string) (bool, error) {
	matched, err := regexp.MatchString(pattern, text)
	return matched, err
}

func (d *DatabaseService) BlockDomain(domain string) bool {
	return d.BlockDomainWithType(domain, "exact")
}

// BlockDomainWithType blocks a domain with a specific filter type
func (d *DatabaseService) BlockDomainWithType(domain string, filterType string) bool {
	if d == nil || d.Db == nil {
		return false
	}
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return false
	}

	// Validate filter type
	if filterType != "exact" && filterType != "glob" && filterType != "regex" {
		filterType = "exact"
	}

	// For exact and glob, convert to lowercase
	if filterType == "exact" || filterType == "glob" {
		domain = strings.ToLower(domain)
	}

	// Validate regex pattern if filter type is regex
	if filterType == "regex" {
		if _, err := regexp.Compile(domain); err != nil {
			log.Printf("Invalid regex pattern %q: %v", domain, err)
			return false
		}
	}

	createStmt := `INSERT OR IGNORE INTO blocked_domains (domain, filter_type) VALUES (?, ?)`

	if _, err := d.Db.Exec(createStmt, domain, filterType); err != nil {
		log.Printf("DB error adding domain %q with type %s: %v", domain, filterType, err)
		return false
	}
	return true
}

// BlockRegexPattern blocks a domain using regex pattern
func (d *DatabaseService) BlockRegexPattern(pattern string) bool {
	return d.BlockDomainWithType(pattern, "regex")
}

// BlockGlobPattern blocks a domain using glob pattern
func (d *DatabaseService) BlockGlobPattern(pattern string) bool {
	return d.BlockDomainWithType(pattern, "glob")
}

func (d *DatabaseService) UnblockDomain(domain string) bool {
	if d == nil || d.Db == nil {
		return false
	}
	domain = strings.ToLower(strings.TrimSpace(domain))
	if domain == "" {
		return false
	}
	deleteStmt := `DELETE FROM blocked_domains WHERE domain = ?`
	if _, err := d.Db.Exec(deleteStmt, domain); err != nil {
		log.Printf("DB error removing domain %q: %v", domain, err)
		return false
	}
	return true
}

func (d *DatabaseService) ListBlockedDomains(domain string) []string {
	if d == nil || d.Db == nil {
		return []string{}
	}

	listStmt := `SELECT domain FROM blocked_domains ORDER BY created_at DESC`

	rows, err := d.Db.Query(listStmt)
	if err != nil {
		log.Printf("DB error listing blocked domains: %v", err)
		return []string{}
	}
	defer rows.Close()
	var domains []string
	for rows.Next() {
		var domain string
		if err := rows.Scan(&domain); err != nil {
			log.Printf("DB error scanning blocked domains: %v", err)
			return domains
		}
		domains = append(domains, domain)
	}
	return domains
}

// BlockedDomainInfo represents a blocked domain with its filter type
type BlockedDomainInfo struct {
	Domain     string `json:"domain"`
	FilterType string `json:"filterType"`
	CreatedAt  string `json:"createdAt"`
}

// ListBlockedDomainsWithInfo returns blocked domains with their filter types
func (d *DatabaseService) ListBlockedDomainsWithInfo() []BlockedDomainInfo {
	if d == nil || d.Db == nil {
		return []BlockedDomainInfo{}
	}

	listStmt := `SELECT domain, filter_type, created_at FROM blocked_domains ORDER BY created_at DESC`

	rows, err := d.Db.Query(listStmt)
	if err != nil {
		log.Printf("DB error listing blocked domains with info: %v", err)
		return []BlockedDomainInfo{}
	}
	defer rows.Close()

	var domains []BlockedDomainInfo
	for rows.Next() {
		var domain, filterType, createdAt string
		if err := rows.Scan(&domain, &filterType, &createdAt); err != nil {
			log.Printf("DB error scanning blocked domains: %v", err)
			continue
		}
		domains = append(domains, BlockedDomainInfo{
			Domain:     domain,
			FilterType: filterType,
			CreatedAt:  createdAt,
		})
	}
	return domains
}
