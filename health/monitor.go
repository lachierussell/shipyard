package health

import (
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/shipyard/shipyard/config"
	"github.com/shipyard/shipyard/service"
)

// Monitor manages backend health monitoring and auto-restart
type Monitor struct {
	cfg           *config.Config
	svcMgr        *service.Manager
	serviceStatus map[string]*ServiceStatus
	mu            sync.RWMutex
	ticker        *time.Ticker
	done          chan bool
}

// ServiceStatus tracks the health status of a service
type ServiceStatus struct {
	LastCheck           time.Time
	ConsecutiveFailures int
	Healthy             bool
}

// NewMonitor creates a new health monitor
func NewMonitor(cfg *config.Config) *Monitor {
	return &Monitor{
		cfg:           cfg,
		svcMgr:        service.NewManager(cfg),
		serviceStatus: make(map[string]*ServiceStatus),
		done:          make(chan bool),
	}
}

// Start begins monitoring services
func (m *Monitor) Start() {
	interval := m.cfg.Health.PollInterval
	if interval == 0 {
		interval = 15 * time.Second
	}

	m.ticker = time.NewTicker(interval)

	go func() {
		for {
			select {
			case <-m.ticker.C:
				m.checkAllServices()
			case <-m.done:
				return
			}
		}
	}()
}

// Stop stops the health monitor
func (m *Monitor) Stop() {
	if m.ticker != nil {
		m.ticker.Stop()
	}
	m.done <- true
}

// checkAllServices polls all backend services
func (m *Monitor) checkAllServices() {
	for siteName, site := range m.cfg.Site {
		if site.Backend == nil {
			continue
		}

		healthy := m.checkService(siteName, &site)

		m.mu.Lock()
		if status, ok := m.serviceStatus[siteName]; ok {
			if !healthy {
				status.ConsecutiveFailures++
				if status.ConsecutiveFailures >= m.cfg.Health.FailureThreshold {
					slog.Warn("health check threshold reached, restarting service",
						"site", siteName,
						"failures", status.ConsecutiveFailures,
					)
					m.svcMgr.Restart(siteName)
					status.ConsecutiveFailures = 0
				}
			} else {
				status.ConsecutiveFailures = 0
			}
			status.Healthy = healthy
			status.LastCheck = time.Now()
		} else {
			m.serviceStatus[siteName] = &ServiceStatus{
				LastCheck:           time.Now(),
				ConsecutiveFailures: 0,
				Healthy:             healthy,
			}
		}
		m.mu.Unlock()
	}
}

// checkService performs a health check on a single service
func (m *Monitor) checkService(siteName string, site *config.SiteConfig) bool {
	if site.Backend == nil {
		return true
	}

	// Make HTTP request to health endpoint
	healthURL := fmt.Sprintf("http://%s:%d%s",
		site.Backend.JailIP,
		site.Backend.ListenPort,
		m.cfg.Health.HealthPath,
	)

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Get(healthURL)
	if err != nil {
		slog.Debug("health check failed", "site", siteName, "url", healthURL, "error", err)
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// GetStatus returns the current status of all services
func (m *Monitor) GetStatus() map[string]*ServiceStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Make a copy
	result := make(map[string]*ServiceStatus)
	for k, v := range m.serviceStatus {
		status := *v
		result[k] = &status
	}

	return result
}

// GetServiceStatus returns the status of a specific service
func (m *Monitor) GetServiceStatus(siteName string) *ServiceStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if status, ok := m.serviceStatus[siteName]; ok {
		s := *status
		return &s
	}

	return nil
}
