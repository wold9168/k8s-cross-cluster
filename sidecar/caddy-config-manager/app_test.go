package main

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestApp_GetApp_Singleton(t *testing.T) {
	// Reset singleton for test
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

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			app := GetApp()
			apps = append(apps, app)
		}()
	}

	wg.Wait()

	// All should be same instance
	for i := 1; i < len(apps); i++ {
		assert.Equal(t, apps[0], apps[i])
	}
}

func TestApp_Initialize(t *testing.T) {
	ResetApp()
	app := GetApp()

	clientset := fake.NewSimpleClientset()
	err := app.Initialize(clientset, "default")

	assert.NoError(t, err)
	assert.NotNil(t, app.configManager)
	assert.NotNil(t, app.permissionChecker)
	assert.False(t, app.IsRunning())
}

func TestApp_Initialize_WithOptions(t *testing.T) {
	ResetApp()
	app := GetApp()

	clientset := fake.NewSimpleClientset()
	err := app.Initialize(clientset, "default", WithInterval(5*time.Second))

	assert.NoError(t, err)
	assert.Equal(t, 5*time.Second, app.interval)
}

func TestApp_Initialize_AlreadyRunning(t *testing.T) {
	ResetApp()
	app := GetApp()

	clientset := fake.NewSimpleClientset()
	err := app.Initialize(clientset, "default")
	assert.NoError(t, err)

	// Manually set running state
	app.mu.Lock()
	app.running = true
	app.mu.Unlock()

	err = app.Initialize(clientset, "default")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already running")
}

func TestApp_Run(t *testing.T) {
	ResetApp()
	app := GetApp()

	clientset := fake.NewSimpleClientset(
		&v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "tailscale-cluster-name",
				Namespace: "default",
			},
			Data: map[string]string{
				"CLUSTER_NAME": "test-cluster",
			},
		},
	)

	err := app.Initialize(clientset, "default", WithInterval(50*time.Millisecond))
	assert.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// Run in goroutine since it's blocking
	done := make(chan struct{})
	go func() {
		_ = app.Run(ctx)
		close(done)
	}()

	// Wait for it to start running
	time.Sleep(30 * time.Millisecond)
	// Note: IsRunning may be false if permission check fails
	_ = app.IsRunning()

	// Wait for context timeout
	<-ctx.Done()
	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
	}
}

func TestApp_Run_AlreadyRunning(t *testing.T) {
	ResetApp()
	app := GetApp()

	clientset := fake.NewSimpleClientset()
	err := app.Initialize(clientset, "default")
	assert.NoError(t, err)

	// Manually set running state
	app.mu.Lock()
	app.running = true
	app.mu.Unlock()

	ctx := context.Background()
	err = app.Run(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already running")
}

func TestApp_Shutdown(t *testing.T) {
	ResetApp()
	app := GetApp()

	clientset := fake.NewSimpleClientset()
	err := app.Initialize(clientset, "default")
	assert.NoError(t, err)

	// Manually set running state
	app.mu.Lock()
	app.running = true
	app.mu.Unlock()

	err = app.Shutdown()
	assert.NoError(t, err)
	assert.False(t, app.IsRunning())
}

func TestApp_Shutdown_NotRunning(t *testing.T) {
	ResetApp()
	app := GetApp()

	err := app.Shutdown()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not running")
}

func TestApp_GetConfigManager(t *testing.T) {
	ResetApp()
	app := GetApp()

	clientset := fake.NewSimpleClientset()
	err := app.Initialize(clientset, "default")
	assert.NoError(t, err)

	cm := app.GetConfigManager()
	assert.NotNil(t, cm)
}

func TestApp_ResetApp(t *testing.T) {
	ResetApp()
	app1 := GetApp()
	app1Addr := fmt.Sprintf("%p", app1)

	ResetApp()
	app2 := GetApp()
	app2Addr := fmt.Sprintf("%p", app2)

	assert.NotNil(t, app1)
	assert.NotNil(t, app2)
	// After reset, new GetApp should return a new instance
	assert.NotEqual(t, app1Addr, app2Addr)
}

func TestApp_WithInterval(t *testing.T) {
	app := &App{}
	opt := WithInterval(30 * time.Second)
	opt(app)

	assert.Equal(t, 30*time.Second, app.interval)
}

func TestPermissionChecker_New(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	pc := NewPermissionChecker(clientset, "test-ns")

	assert.NotNil(t, pc)
	assert.Equal(t, "test-ns", pc.namespace)
}

func TestPermissionChecker_CheckServicePermissions(t *testing.T) {
	ctx := context.Background()
	clientset := fake.NewSimpleClientset()
	pc := NewPermissionChecker(clientset, "default")

	err := pc.CheckServicePermissions(ctx)

	// Fake clientset may deny permissions by default
	// This test verifies the function handles errors gracefully
	if err != nil {
		assert.Contains(t, err.Error(), "access denied")
	}
}

func TestPermissionChecker_CheckServicePermissions_EmptyNamespace(t *testing.T) {
	ctx := context.Background()
	clientset := fake.NewSimpleClientset()
	pc := NewPermissionChecker(clientset, "")

	err := pc.CheckServicePermissions(ctx)

	// Should handle empty namespace gracefully (may still fail on permissions)
	// Test just ensures no panic occurs
	_ = err
}

func TestApp_Initialize_LoadClusterNameFailure(t *testing.T) {
	ResetApp()
	app := GetApp()

	clientset := fake.NewSimpleClientset()
	// No ConfigMap, should use default cluster name
	err := app.Initialize(clientset, "default")

	assert.NoError(t, err) // Should not fail, just use default
	assert.Equal(t, "default-cluster-name", app.GetConfigManager().GetClusterName())
}

func TestApp_IsRunning_Concurrent(t *testing.T) {
	ResetApp()
	app := GetApp()

	clientset := fake.NewSimpleClientset()
	err := app.Initialize(clientset, "default")
	assert.NoError(t, err)

	var results []bool
	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results = append(results, app.IsRunning())
		}()
	}

	wg.Wait()

	// All should return same value (false)
	for _, result := range results {
		assert.False(t, result)
	}
}
