package main

import "time"

// Server configuration constants
const (
	syncInterval    = 10 * time.Second
	syncIntervalKey = "SYNC_INTERVAL"

	metricsPort = ":8082"
	metricsIp   = "0.0.0.0"
	metricsAddr = metricsIp + metricsPort

	configPath = "/config/Caddyfile"
	configDir  = "/config"

	clusterNameConfigMap = "tailscale-cluster-name"

	// Caddy admin API configuration
	caddyAdminPort     = "2019"
	caddyAdminPortKey = "CADDY_ADMIN_PORT"

	// API server configuration
	apiPort = ":8081"
	apiIp   = "0.0.0.0"
	apiAddr = apiIp + apiPort
)
