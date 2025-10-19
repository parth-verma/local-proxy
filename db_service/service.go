package db_service

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
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

func (d *DatabaseService) ServiceName() string { return "db_service" }

func (d *DatabaseService) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	if instance != nil {
		log.Printf("DB Service already started")
		return nil
	}
	instance = d
	d.ctx = ctx
	d.options = options
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
	createStmt := `CREATE TABLE IF NOT EXISTS blocked_domains (
		domain TEXT PRIMARY KEY,
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

// IsDomainBlocked checks exact or glob patterns (SQLite GLOB) case-insensitively.
func (d *DatabaseService) IsDomainBlocked(domain string) bool {
	if d == nil || d.Db == nil {
		return false
	}
	domain = strings.ToLower(strings.TrimSpace(domain))
	if domain == "" {
		return false
	}
	var exists int
	err := d.Db.QueryRow(
		`SELECT 1 FROM blocked_domains
	      WHERE lower(domain) = ?
	         OR ? GLOB lower(domain)
	      LIMIT 1`,
		domain, domain,
	).Scan(&exists)
	if err == sql.ErrNoRows {
		return false
	}
	if err != nil {
		log.Printf("DB error checking domain %q: %v", domain, err)
		return false
	}
	return true
}

func (d *DatabaseService) BlockDomain(domain string) bool {
	if d == nil || d.Db == nil {
		return false
	}
	domain = strings.ToLower(strings.TrimSpace(domain))
	if domain == "" {
		return false
	}

	createStmt := `INSERT OR IGNORE INTO blocked_domains (domain) VALUES (?)`

	if _, err := d.Db.Exec(createStmt, domain); err != nil {
		log.Printf("DB error adding domain %q: %v", domain, err)
		return false
	}
	return true
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

	listStmt := `SELECT domain FROM blocked_domains`

	rows, err := d.Db.Query(listStmt, domain)
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
