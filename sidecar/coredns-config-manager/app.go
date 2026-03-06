package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

// App is the singleton application instance for coredns-config-manager
type App struct {
	dnsConfigManager *DNSConfigManager
	permissionChecker *PermissionCheckerImpl
	config           DNSConfigManagerConfig

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
			config: DefaultDNSConfigManagerConfig(),
		}
		klog.Info("CoreDNS Config Manager App initialized (singleton)")
	})
	return appInstance
}

// AppOption configures the App instance
type AppOption func(*App)

// WithConfig sets a custom DNSConfigManagerConfig
func WithConfig(config DNSConfigManagerConfig) AppOption {
	return func(a *App) {
		a.config = config
	}
}

// WithSyncInterval sets the synchronization interval
func WithSyncInterval(interval time.Duration) AppOption {
	return func(a *App) {
		a.config.SyncInterval = interval
	}
}

// Initialize initializes the singleton App with required dependencies
func (a *App) Initialize(clientset *kubernetes.Clientset, namespace string, opts ...AppOption) error {
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
	a.dnsConfigManager = NewDNSConfigManager(clientset, a.config)
	a.permissionChecker = NewPermissionChecker(clientset, namespace)

	// Initialize the DNS config manager components
	ctx := context.Background()
	if err := a.dnsConfigManager.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize DNS config manager: %w", err)
	}

	klog.Info("CoreDNS Config Manager App components initialized")
	return nil
}

// Run starts the application synchronization loop
func (a *App) Run(ctx context.Context) error {
	a.mu.Lock()
	if a.running {
		a.mu.Unlock()
		return fmt.Errorf("application is already running")
	}
	a.running = true
	a.mu.Unlock()

	klog.Infof("Starting DNS config syncer (interval: %v)", a.config.SyncInterval)

	ticker := time.NewTicker(a.config.SyncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			klog.Info("Context cancelled, stopping DNSConfigManager")
			return nil
		case <-ticker.C:
			// Check permissions
			if err := a.permissionChecker.Check(ctx); err != nil {
				klog.Errorf("Authorization failed: %v, retrying in %v", err, a.config.SyncInterval)
				continue
			}

			// Perform sync
			if err := a.dnsConfigManager.Sync(ctx); err != nil {
				klog.Errorf("Sync failed: %v", err)
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

// GetDNSConfigManager returns the DNS config manager instance
func (a *App) GetDNSConfigManager() *DNSConfigManager {
	return a.dnsConfigManager
}

// Shutdown gracefully stops the application
func (a *App) Shutdown() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.running {
		return fmt.Errorf("application is not running")
	}

	if err := a.dnsConfigManager.Shutdown(); err != nil {
		klog.Errorf("DNS config manager shutdown error: %v", err)
	}

	a.running = false
	klog.Info("CoreDNS Config Manager App shutdown complete")
	return nil
}

// Reset resets the singleton instance (primarily for testing)
func ResetApp() {
	appInstance = nil
	appOnce = sync.Once{}
}
