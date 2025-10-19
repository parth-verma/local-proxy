package db_service

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestNewDBService(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Test successful creation
	service, err := NewDBService(tempDir)
	if err != nil {
		t.Fatalf("NewDBService failed: %v", err)
	}

	if service == nil {
		t.Fatal("NewDBService returned nil service")
	}

	if service.Db == nil {
		t.Fatal("Database connection is nil")
	}

	// Test that database file was created
	dbPath := filepath.Join(tempDir, "local-proxy.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatal("Database file was not created")
	}

	// Clean up
	service.ServiceShutdown()
}

func TestNewDBService_InvalidDirectory(t *testing.T) {
	// Test with invalid directory (read-only)
	invalidDir := "/invalid/path/that/does/not/exist"

	_, err := NewDBService(invalidDir)
	if err == nil {
		t.Fatal("Expected error for invalid directory, got nil")
	}
}

func TestDatabaseService_BlockDomain(t *testing.T) {
	service := setupTestService(t)
	defer service.ServiceShutdown()

	// Test blocking a domain
	domain := "example.com"
	if !service.BlockDomain(domain) {
		t.Fatal("Failed to block domain")
	}

	// Verify domain is blocked
	if !service.IsDomainBlocked(domain) {
		t.Fatal("Domain should be blocked")
	}

	// Test case insensitive blocking
	if !service.IsDomainBlocked("EXAMPLE.COM") {
		t.Fatal("Domain should be blocked (case insensitive)")
	}
}

func TestDatabaseService_BlockDomainWithType(t *testing.T) {
	service := setupTestService(t)
	defer service.ServiceShutdown()

	tests := []struct {
		domain     string
		filterType string
		shouldPass bool
	}{
		{"exact.com", "exact", true},
		{"glob.*.com", "glob", true},
		{"regex.*\\.com$", "regex", true},
		{"invalid-regex[", "regex", false}, // Invalid regex
		{"", "exact", false},               // Empty domain
		{"test.com", "invalid", true},      // Invalid filter type should default to exact
	}

	for _, tt := range tests {
		t.Run(tt.domain+"_"+tt.filterType, func(t *testing.T) {
			result := service.BlockDomainWithType(tt.domain, tt.filterType)
			if result != tt.shouldPass {
				t.Errorf("BlockDomainWithType(%q, %q) = %v, want %v", tt.domain, tt.filterType, result, tt.shouldPass)
			}
		})
	}
}

func TestDatabaseService_IsDomainBlocked_Exact(t *testing.T) {
	service := setupTestService(t)
	defer service.ServiceShutdown()

	// Block exact domain
	domain := "example.com"
	service.BlockDomainWithType(domain, "exact")

	// Test exact matches
	if !service.IsDomainBlocked("example.com") {
		t.Fatal("Should block exact match")
	}
	if !service.IsDomainBlocked("EXAMPLE.COM") {
		t.Fatal("Should block case-insensitive match")
	}

	// Test non-matches
	if service.IsDomainBlocked("sub.example.com") {
		t.Fatal("Should not block subdomain")
	}
	if service.IsDomainBlocked("example.org") {
		t.Fatal("Should not block different domain")
	}
}

func TestDatabaseService_IsDomainBlocked_Glob(t *testing.T) {
	service := setupTestService(t)
	defer service.ServiceShutdown()

	// Block glob pattern
	pattern := "*.example.com"
	service.BlockDomainWithType(pattern, "glob")

	// Test glob matches
	if !service.IsDomainBlocked("sub.example.com") {
		t.Fatal("Should block subdomain with glob pattern")
	}
	if !service.IsDomainBlocked("api.example.com") {
		t.Fatal("Should block api subdomain with glob pattern")
	}

	// Test non-matches
	if service.IsDomainBlocked("example.com") {
		t.Fatal("Should not block exact domain with glob pattern")
	}
	if service.IsDomainBlocked("other.com") {
		t.Fatal("Should not block different domain")
	}
}

func TestDatabaseService_IsDomainBlocked_Regex(t *testing.T) {
	service := setupTestService(t)
	defer service.ServiceShutdown()

	// Block regex pattern
	pattern := `.*\.example\.com$`
	service.BlockDomainWithType(pattern, "regex")

	// Test regex matches
	if !service.IsDomainBlocked("sub.example.com") {
		t.Fatal("Should block subdomain with regex pattern")
	}
	if !service.IsDomainBlocked("api.example.com") {
		t.Fatal("Should block api subdomain with regex pattern")
	}

	// Test non-matches
	if service.IsDomainBlocked("example.com") {
		t.Fatal("Should not block exact domain with regex pattern")
	}
	if service.IsDomainBlocked("other.com") {
		t.Fatal("Should not block different domain")
	}
}

func TestDatabaseService_UnblockDomain(t *testing.T) {
	service := setupTestService(t)
	defer service.ServiceShutdown()

	// Block a domain
	domain := "example.com"
	service.BlockDomain(domain)

	// Verify it's blocked
	if !service.IsDomainBlocked(domain) {
		t.Fatal("Domain should be blocked")
	}

	// Unblock the domain
	if !service.UnblockDomain(domain) {
		t.Fatal("Failed to unblock domain")
	}

	// Verify it's unblocked
	if service.IsDomainBlocked(domain) {
		t.Fatal("Domain should be unblocked")
	}

	// Test case insensitive unblocking
	service.BlockDomain("test.com")
	if !service.UnblockDomain("TEST.COM") {
		t.Fatal("Failed to unblock domain (case insensitive)")
	}
	if service.IsDomainBlocked("test.com") {
		t.Fatal("Domain should be unblocked (case insensitive)")
	}
}

func TestDatabaseService_ListBlockedDomains(t *testing.T) {
	service := setupTestService(t)
	defer service.ServiceShutdown()

	// Initially should be empty
	domains := service.ListBlockedDomains("")
	if len(domains) != 0 {
		t.Fatal("Should start with empty blocked domains list")
	}

	// Block some domains
	domainsToBlock := []string{"example.com", "test.org", "api.example.com"}
	for _, domain := range domainsToBlock {
		service.BlockDomain(domain)
	}

	// List blocked domains
	blockedDomains := service.ListBlockedDomains("")
	if len(blockedDomains) != len(domainsToBlock) {
		t.Fatalf("Expected %d blocked domains, got %d", len(domainsToBlock), len(blockedDomains))
	}

	// Verify all domains are in the list
	for _, expectedDomain := range domainsToBlock {
		found := false
		for _, blockedDomain := range blockedDomains {
			if blockedDomain == expectedDomain {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("Domain %s not found in blocked domains list", expectedDomain)
		}
	}
}

func TestDatabaseService_ListBlockedDomainsWithInfo(t *testing.T) {
	service := setupTestService(t)
	defer service.ServiceShutdown()

	// Block domains with different filter types
	service.BlockDomainWithType("exact.com", "exact")
	service.BlockDomainWithType("*.glob.com", "glob")
	service.BlockDomainWithType(".*\\.regex\\.com$", "regex")

	// Get domains with info
	domainsInfo := service.ListBlockedDomainsWithInfo()
	if len(domainsInfo) != 3 {
		t.Fatalf("Expected 3 blocked domains, got %d", len(domainsInfo))
	}

	// Verify filter types
	filterTypes := make(map[string]string)
	for _, info := range domainsInfo {
		filterTypes[info.Domain] = info.FilterType
	}

	expectedTypes := map[string]string{
		"exact.com":         "exact",
		"*.glob.com":        "glob",
		".*\\.regex\\.com$": "regex",
	}

	for domain, expectedType := range expectedTypes {
		if actualType, exists := filterTypes[domain]; !exists {
			t.Fatalf("Domain %s not found in blocked domains", domain)
		} else if actualType != expectedType {
			t.Fatalf("Domain %s has filter type %s, expected %s", domain, actualType, expectedType)
		}
	}
}

func TestDatabaseService_EdgeCases(t *testing.T) {
	service := setupTestService(t)
	defer service.ServiceShutdown()

	// Test with nil service
	var nilService *DatabaseService
	if nilService.IsDomainBlocked("test.com") {
		t.Fatal("Nil service should not block domains")
	}
	if nilService.BlockDomain("test.com") {
		t.Fatal("Nil service should not block domains")
	}
	if nilService.UnblockDomain("test.com") {
		t.Fatal("Nil service should not unblock domains")
	}
	if len(nilService.ListBlockedDomains("")) != 0 {
		t.Fatal("Nil service should return empty domain list")
	}

	// Test with empty domain
	if service.BlockDomain("") {
		t.Fatal("Should not block empty domain")
	}
	if service.UnblockDomain("") {
		t.Fatal("Should not unblock empty domain")
	}
	if service.IsDomainBlocked("") {
		t.Fatal("Should not block empty domain")
	}

	// Test with whitespace-only domain
	if service.BlockDomain("   ") {
		t.Fatal("Should not block whitespace-only domain")
	}
}

func TestDatabaseService_ConcurrentAccess(t *testing.T) {
	service := setupTestService(t)
	defer service.ServiceShutdown()

	// Test concurrent blocking with retry mechanism
	done := make(chan bool, 10)
	results := make(chan string, 10)

	for i := 0; i < 10; i++ {
		go func(i int) {
			domain := fmt.Sprintf("test%d.com", i)
			// Retry on database lock
			for retries := 0; retries < 3; retries++ {
				if service.BlockDomain(domain) {
					results <- domain
					break
				}
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
	close(results)

	// Collect successfully blocked domains
	blockedDomains := make(map[string]bool)
	for domain := range results {
		blockedDomains[domain] = true
	}

	// Verify that at least some domains were blocked (allowing for some failures due to locking)
	if len(blockedDomains) == 0 {
		t.Fatal("No domains were successfully blocked")
	}

	// Verify blocked domains are actually blocked
	for domain := range blockedDomains {
		if !service.IsDomainBlocked(domain) {
			t.Fatalf("Domain %s should be blocked", domain)
		}
	}
}

// Helper function to set up a test service
func setupTestService(t *testing.T) *DatabaseService {
	tempDir := t.TempDir()
	service, err := NewDBService(tempDir)
	if err != nil {
		t.Fatalf("Failed to create test service: %v", err)
	}
	return service
}
