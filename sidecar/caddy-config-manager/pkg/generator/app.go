package generator

import (
	"context"
	"fmt"
	"sync"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

// App is the singleton application instance for caddy-config-manager
type App struct {
	configManager     *ConfigManager
	permissionChecker *PermissionChecker
	interval          time.Duration

	// Internal state
	running bool
	mu      sync.RWMutex
}

var (
	appInstance *App
	appOnce     sync.Once
)

// GetApp returns the singleton App instance
func GetApp() *App {
	appOnce.Do(func() {
		appInstance = &App{
			interval: 10 * time.Second,
		}
		klog.Info("Caddy Config Manager App initialized (singleton)")
	})
	return appInstance
}

// AppOption configures the App instance
type AppOption func(*App)

// WithInterval sets the sync interval
func WithInterval(interval time.Duration) AppOption {
	return func(a *App) {
		a.interval = interval
	}
}

// Initialize initializes the singleton App with required dependencies
func (a *App) Initialize(clientset kubernetes.Interface, namespace string, opts ...AppOption) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.running {
		return fmt.Errorf("application is already running")
	}

	// Apply options
	for _, opt := range opts {
		opt(a)
	}

	// Initialize components
	a.configManager = NewConfigManager(
		clientset,
		WithConfigPath("/config/Caddyfile"),
		WithConfigDir("/config"),
	)

	// Load cluster name
	if err := a.configManager.LoadClusterName("tailscale-cluster-name"); err != nil {
		klog.Warningf("Failed to load cluster name: %v", err)
	}

	a.permissionChecker = NewPermissionChecker(clientset, namespace)

	klog.Info("Caddy Config Manager App components initialized")
	return nil
}

// Run starts the application synchronization loop
// This is a blocking call
func (a *App) Run(ctx context.Context) error {
	a.mu.Lock()
	if a.running {
		a.mu.Unlock()
		return fmt.Errorf("application is already running")
	}
	a.running = true
	a.mu.Unlock()

	klog.Infof("Starting Caddy config syncer (interval: %v)", a.interval)

	ticker := time.NewTicker(a.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			klog.Info("Context cancelled, stopping syncer")
			return nil
		case <-ticker.C:
			// Check permissions
			if err := a.permissionChecker.CheckServicePermissions(ctx); err != nil {
				klog.Errorf("Permission check failed: %v, retrying in %v", err, a.interval)
				continue
			}

			// Sync configuration
			if err := a.configManager.Sync(ctx); err != nil {
				klog.Errorf("Configuration sync failed: %v", err)
			}
		}
	}
}

// IsRunning returns whether the application is currently running
func (a *App) IsRunning() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.running
}

// GetConfigManager returns the config manager instance
func (a *App) GetConfigManager() *ConfigManager {
	return a.configManager
}

// Shutdown gracefully stops the application
func (a *App) Shutdown() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.running {
		return fmt.Errorf("application is not running")
	}

	a.running = false
	klog.Info("Caddy Config Manager App shutdown complete")
	return nil
}

// Reset resets the singleton instance (primarily for testing)
func ResetApp() {
	appInstance = nil
	appOnce = sync.Once{}
}
