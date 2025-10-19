package proxy_service

import (
	"changeme/db_service"
	"changeme/logging_service"
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/elazarl/goproxy"
	"github.com/wailsapp/wails/v3/pkg/application"
)

const PROXY_PORT = 30002

// ---------------- Service Setup ----------------
// This is the main service struct. It can be named anything you like.
// Both the ServiceStartup() and ServiceShutdown() methods are called synchronously when the app starts and stops.
// Changing the name of this struct will change the name of the services class in the frontend
// Bound methods will exist inside frontend/bindings/github.com/user/proxy_service under the name of the struct
type ProxyService struct {
	ctx      context.Context
	options  application.ServiceOptions
	isPaused bool
}

// singleton instance for easy access from other services
var instance *ProxyService

func Instance() *ProxyService { return instance }

func (p *ProxyService) StartProxy() {
	proxy := goproxy.NewProxyHttpServer()
	proxy.Verbose = true

	proxy.OnRequest().HandleConnectFunc(func(host string, ctx *goproxy.ProxyCtx) (*goproxy.ConnectAction, string) {
		start := time.Now()
		port := 443
		i := strings.LastIndex(host, ":")
		if i != -1 {
			if pnum, err := strconv.Atoi(host[i+1:]); err == nil {
				port = pnum
			}
		}

		modifiedHost := host[:i]

		blocked := db_service.Instance().IsDomainBlocked(strings.ToLower(modifiedHost))

		if blocked {
			log.Printf("CONNECT request for host: %s, port: %d, blocked: %v", modifiedHost, port, blocked)
			go logging_service.Instance().LogRequest(host, "CONNECT", "", port, false, time.Since(start).Nanoseconds())
			return goproxy.OkConnect, host
		}
		go logging_service.Instance().LogRequest(host, "CONNECT", "", port, true, time.Since(start).Nanoseconds())
		return goproxy.OkConnect, host
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
	if instance != nil {
		log.Printf("Proxy Service already started")
		return nil
	}
	instance = p
	p.ctx = ctx
	p.options = options
	p.isPaused = true
	// Start the local HTTP proxy as soon as the application starts.
	go p.StartProxy()
	return nil
}

// ServiceShutdown is called when the app is shutting down via runtime.Quit() call
// You can use this to clean up any resources you have allocated
// OPTIONAL: This method is optional.
func (p *ProxyService) ServiceShutdown() error {
	// On macOS, revert the system proxy settings we previously applied.
	if runtime.GOOS == "darwin" {
		if !p.isPaused {
			if err := unsetMacSystemProxy(); err != nil {
				log.Printf("Warning: failed to unset macOS system proxy: %v", err)
			} else {
				log.Printf("macOS system HTTP(S) proxy disabled")
			}
		}
	}
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

func (p *ProxyService) PauseProxy() error {
	if p.isPaused {
		return nil
	}
	err := unsetMacSystemProxy()
	if err != nil {
		return err
	}
	p.isPaused = true
	return nil
}

func (p *ProxyService) ResumeProxy() error {
	if !p.isPaused {
		return nil
	}
	if err := setMacSystemProxy(PROXY_PORT); err != nil {
		return err
	}
	p.isPaused = true
	return nil
}
