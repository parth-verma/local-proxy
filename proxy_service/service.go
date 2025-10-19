package proxy_service

import (
	"changeme/db_service"
	"context"
	"fmt"
	"github.com/elazarl/goproxy"
	"github.com/nakabonne/tstorage"
	"github.com/wailsapp/wails/v3/pkg/application"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const PROXY_PORT = 30002

// ---------------- Service Setup ----------------
// This is the main service struct. It can be named anything you like.
// Both the ServiceStartup() and ServiceShutdown() methods are called synchronously when the app starts and stops.
// Changing the name of this struct will change the name of the services class in the frontend
// Bound methods will exist inside frontend/bindings/github.com/user/proxy_service under the name of the struct
type ProxyService struct {
	ctx     context.Context
	options application.ServiceOptions
	tDB     tstorage.Storage
	tDBPath string
}

func (p *ProxyService) StartProxy() {
	proxy := goproxy.NewProxyHttpServer()
	proxy.Verbose = true

	// Intercept all requests (HTTP and CONNECT) to enforce domain blocking and record to tDB

	proxy.OnRequest().DoFunc(func(r *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
		start := time.Now()
		host := r.URL.Host
		if host == "" {
			host = r.Host
		}
		// Extract hostname and port
		hostOnly := host
		port := 0
		if i := strings.LastIndex(host, ":"); i != -1 {
			hostOnly = host[:i]
			if pnum, err := strconv.Atoi(host[i+1:]); err == nil {
				port = pnum
			}
		}
		if port == 0 {
			if r.Method == http.MethodConnect || strings.EqualFold(r.URL.Scheme, "https") {
				port = 443
			} else {
				port = 80
			}
		}
		method := r.Method
		path := r.URL.Path
		blocked := db_service.Instance().IsDomainBlocked(strings.ToLower(hostOnly))
		dur := time.Since(start).Nanoseconds()
		if blocked {
			p.logRequest(hostOnly, method, path, port, false, dur)
			return r, goproxy.NewResponse(r, goproxy.ContentTypeText, http.StatusForbidden, "Domain blocked")
		}
		p.logRequest(hostOnly, method, path, port, true, dur)
		return r, nil
	})
	// Start the proxy server asynchronously so we can proceed to set system proxy after it is ready.
	errCh := make(chan error, 1)
	go func() {
		if err := http.ListenAndServe(fmt.Sprintf(":%d", PROXY_PORT), proxy); err != nil {
			errCh <- err
		}
	}()

	// Wait until the port is accepting connections or an error occurs
	if err := waitForPort("127.0.0.1", PROXY_PORT, 5*time.Second); err != nil {
		select {
		case srvErr := <-errCh:
			log.Fatal("Proxy failed to start: ", srvErr)
		default:
			log.Fatal("Proxy did not become ready in time: ", err)
		}
	}

	// If running on macOS, configure system HTTP(S) proxy settings to use the local proxy.
	if runtime.GOOS == "darwin" {
		if err := setMacSystemProxy(PROXY_PORT); err != nil {
			log.Printf("Warning: failed to set macOS system proxy: %v", err)
		} else {
			log.Printf("macOS system HTTP(S) proxy set to 127.0.0.1:%d", PROXY_PORT)
		}
	}
}

// waitForPort attempts to connect to host:port until timeout
func waitForPort(host string, port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for %s:%d", host, port)
}

// setMacSystemProxy sets HTTP and HTTPS proxy for all available network services on macOS.
func setMacSystemProxy(port int) error {
	// Get list of network services
	out, err := exec.Command("networksetup", "-listallnetworkservices").CombinedOutput()
	if err != nil {
		return fmt.Errorf("listing network services failed: %w; output: %s", err, strings.TrimSpace(string(out)))
	}
	lines := strings.Split(string(out), "\n")
	services := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Skip header line commonly present on macOS
		if strings.HasPrefix(line, "An asterisk (") {
			continue
		}
		// Some disabled services may be prefixed with an asterisk
		line = strings.TrimPrefix(line, "*")
		line = strings.TrimSpace(line)
		if line != "" {
			services = append(services, line)
		}
	}
	if len(services) == 0 {
		return fmt.Errorf("no network services found")
	}

	host := "127.0.0.1"
	portStr := strconv.Itoa(port)
	var firstErr error
	for _, svc := range services {
		// Set HTTP proxy
		if out, err := exec.Command("networksetup", "-setwebproxy", svc, host, portStr).CombinedOutput(); err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("setwebproxy failed for %q: %w; output: %s", svc, err, strings.TrimSpace(string(out)))
			}
		}
		// Enable HTTP proxy
		if out, err := exec.Command("networksetup", "-setwebproxystate", svc, "on").CombinedOutput(); err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("setwebproxystate failed for %q: %w; output: %s", svc, err, strings.TrimSpace(string(out)))
			}
		}
		// Set HTTPS proxy
		if out, err := exec.Command("networksetup", "-setsecurewebproxy", svc, host, portStr).CombinedOutput(); err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("setsecurewebproxy failed for %q: %w; output: %s", svc, err, strings.TrimSpace(string(out)))
			}
		}
		// Enable HTTPS proxy
		if out, err := exec.Command("networksetup", "-setsecurewebproxystate", svc, "on").CombinedOutput(); err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("setsecurewebproxystate failed for %q: %w; output: %s", svc, err, strings.TrimSpace(string(out)))
			}
		}
	}
	return firstErr
}

// ServiceName is the name of the service
func (p *ProxyService) ServiceName() string {
	return "proxy_service"
}

// ServiceStartup is called when the app is starting up. You can use this to
// initialise any resources you need. You can also access the application
// instance via the app property.
// OPTIONAL: This method is optional.
func (p *ProxyService) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	p.ctx = ctx
	p.options = options
	// Initialise the SQLite database in the user's PathDataHome-like directory
	if err := p.initDB(); err != nil {
		log.Printf("Failed to init DB: %v", err)
		// Do not fail startup; proxy can still run, but blocking won't work.
	}
	// Start the local HTTP proxy as soon as the application starts.
	go p.StartProxy()
	return nil
}

// ServiceShutdown is called when the app is shutting down via runtime.Quit() call
// You can use this to clean up any resources you have allocated
// OPTIONAL: This method is optional.
func (p *ProxyService) ServiceShutdown() error {
	// Close time-series DB if open
	if p.tDB != nil {
		if err := p.tDB.Close(); err != nil {
			log.Printf("Warning: failed to close tstorage: %v", err)
		}
	}
	// On macOS, revert the system proxy settings we previously applied.
	if runtime.GOOS == "darwin" {
		if err := unsetMacSystemProxy(); err != nil {
			log.Printf("Warning: failed to unset macOS system proxy: %v", err)
		} else {
			log.Printf("macOS system HTTP(S) proxy disabled")
		}
	}
	return nil
}

// initDB opens/creates the SQLite database in the user's data home directory and creates the table if needed.
func (p *ProxyService) initDB() error {
	dataDir := application.Path(application.PathDataHome)
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return fmt.Errorf("failed to create data dir: %w", err)
	}

	tDBPath := filepath.Join(dataDir, "logs.db")
	storage, err := tstorage.NewStorage(
		tstorage.WithDataPath(tDBPath),
	)

	if err != nil {
		return fmt.Errorf("failed to create tstorage: %w", err)
	}

	p.tDB = storage
	p.tDBPath = tDBPath
	return nil
}

// ---------------- Service Methods ----------------
// Service methods are just normal Go methods. You can add as many as you like.
// The only requirement is that they are exported (start with a capital letter).
// You can also return any type that is JSON serializable.
// See https://golang.org/pkg/encoding/json/#Marshal for more information.

// unsetMacSystemProxy disables HTTP and HTTPS proxy for all available network services on macOS.
func unsetMacSystemProxy() error {
	// Get list of network services
	out, err := exec.Command("networksetup", "-listallnetworkservices").CombinedOutput()
	if err != nil {
		return fmt.Errorf("listing network services failed: %w; output: %s", err, strings.TrimSpace(string(out)))
	}
	lines := strings.Split(string(out), "\n")
	services := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Skip header line commonly present on macOS
		if strings.HasPrefix(line, "An asterisk (") {
			continue
		}
		// Some disabled services may be prefixed with an asterisk
		line = strings.TrimPrefix(line, "*")
		line = strings.TrimSpace(line)
		if line != "" {
			services = append(services, line)
		}
	}
	if len(services) == 0 {
		return fmt.Errorf("no network services found")
	}

	var firstErr error
	for _, svc := range services {
		// Disable HTTP proxy
		if out, err := exec.Command("networksetup", "-setwebproxystate", svc, "off").CombinedOutput(); err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("setwebproxystate(off) failed for %q: %w; output: %s", svc, err, strings.TrimSpace(string(out)))
			}
		}
		// Disable HTTPS proxy
		if out, err := exec.Command("networksetup", "-setsecurewebproxystate", svc, "off").CombinedOutput(); err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("setsecurewebproxystate(off) failed for %q: %w; output: %s", svc, err, strings.TrimSpace(string(out)))
			}
		}
	}
	return firstErr
}

// logRequest writes a point to the time-series database capturing request metadata and decision.
func (p *ProxyService) logRequest(host, method, path string, port int, approved bool, duration int64) {
	if p == nil || p.tDB == nil {
		return
	}
	value := duration
	decision := "rejected"
	if approved {
		decision = "approved"
	}
	labels := []tstorage.Label{
		{Name: "host", Value: host},
		{Name: "method", Value: strings.ToUpper(method)},
		{Name: "path", Value: path},
		{Name: "port", Value: strconv.Itoa(port)},
		{Name: "decision", Value: decision},
	}
	pt := tstorage.Row{
		Metric: "proxy_requests",
		Labels: labels,
		DataPoint: tstorage.DataPoint{
			Timestamp: time.Now().UnixMilli(),
			Value:     float64(value),
		},
	}
	var rows []tstorage.Row
	rows = append(rows, pt)
	if err := p.tDB.InsertRows(rows); err != nil {
		log.Printf("warning: failed to write request log: %v", err)
	}
}

func (p *ProxyService) PauseProxy() error {
	return unsetMacSystemProxy()
}

func (p *ProxyService) ResumeProxy() error {
	return setMacSystemProxy(PROXY_PORT)
}
