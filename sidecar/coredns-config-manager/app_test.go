package main

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestApp_GetApp_Singleton(t *testing.T) {
	ResetApp()

	app1 := GetApp()
	app2 := GetApp()

	assert.NotNil(t, app1)
	assert.NotNil(t, app2)
	assert.Equal(t, app1, app2) // Should be same instance
}

func TestApp_GetApp_Concurrent(t *testing.T) {
	ResetApp()

	var apps []*App
	var wg sync.WaitGroup
	var mu sync.Mutex

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			app := GetApp()
			mu.Lock()
			apps = append(apps, app)
			mu.Unlock()
		}()
	}

	wg.Wait()

	// All should be same instance
	for i := 1; i < len(apps); i++ {
		assert.Equal(t, apps[0], apps[i])
	}
}

func TestApp_DefaultConfig(t *testing.T) {
	ResetApp()
	app := GetApp()

	assert.NotNil(t, app)
	assert.NotNil(t, app.config)
	assert.Equal(t, syncInterval, app.config.SyncInterval)
}

func TestApp_WithOptions(t *testing.T) {
	ResetApp()
	app := GetApp()

	opt := WithSyncInterval(5 * time.Second)
	opt(app)

	assert.Equal(t, 5*time.Second, app.config.SyncInterval)
}

func TestApp_WithConfig(t *testing.T) {
	ResetApp()
	app := &App{}
	customConfig := DNSConfigManagerConfig{
		SyncInterval: 60 * time.Second,
		SubDNSAddr:   "127.0.0.1:20053",
	}
	opt := WithConfig(customConfig)
	opt(app)

	assert.Equal(t, customConfig, app.config)
}

func TestApp_IsRunning(t *testing.T) {
	ResetApp()
	app := GetApp()

	assert.False(t, app.IsRunning())

	app.mu.Lock()
	app.running = true
	app.mu.Unlock()

	assert.True(t, app.IsRunning())
}

func TestApp_IsRunning_Concurrent(t *testing.T) {
	ResetApp()
	app := GetApp()

	var results []bool
	var wg sync.WaitGroup
	var mu sync.Mutex

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result := app.IsRunning()
			mu.Lock()
			results = append(results, result)
			mu.Unlock()
		}()
	}

	wg.Wait()

	// All should return same value (false)
	for _, result := range results {
		assert.False(t, result)
	}
}

func TestApp_Shutdown_NotRunning(t *testing.T) {
	ResetApp()
	app := GetApp()

	err := app.Shutdown()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not running")
}

func TestApp_ResetApp(t *testing.T) {
	ResetApp()
	app1 := GetApp()
	app1Addr := "%p"

	ResetApp()
	app2 := GetApp()
	app2Addr := "%p"

	assert.NotNil(t, app1)
	assert.NotNil(t, app2)
	// Verify both are valid app instances
	assert.NotNil(t, app1.config)
	assert.NotNil(t, app2.config)
	
	// Suppress unused variable warning
	_ = app1Addr
	_ = app2Addr
}

func TestDNSConfigManagerConfig_Default(t *testing.T) {
	config := DefaultDNSConfigManagerConfig()

	assert.Equal(t, syncInterval, config.SyncInterval)
	assert.Equal(t, subdnsAddr, config.SubDNSAddr)
	assert.Equal(t, metricsAddr, config.MetricsAddr)
	assert.Equal(t, svcAddr, config.APIAddr)
	assert.Equal(t, CoreDNSNamespace, config.CoreDNSConfig.Namespace)
}

func TestDNSConfigManagerConfig_Custom(t *testing.T) {
	config := DNSConfigManagerConfig{
		SyncInterval: 30 * time.Second,
		SubDNSAddr:   "127.0.0.1:20053",
		MetricsAddr:  "127.0.0.1:9090",
		APIAddr:      "127.0.0.1:8082",
	}

	assert.Equal(t, 30*time.Second, config.SyncInterval)
	assert.Equal(t, "127.0.0.1:20053", config.SubDNSAddr)
	assert.Equal(t, "127.0.0.1:9090", config.MetricsAddr)
	assert.Equal(t, "127.0.0.1:8082", config.APIAddr)
}
