package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/kmala/timelapse/internal/api"
	"github.com/kmala/timelapse/internal/config"
	"github.com/kmala/timelapse/internal/manager"
	"github.com/kmala/timelapse/internal/persistence"
	"github.com/kmala/timelapse/internal/storage"
)

const banner = `
╔═══════════════════════════════════════════════════════════════╗
║                   TimeLapse Camera System                     ║
║                        v0.5.1 (Persistence)                   ║
╚═══════════════════════════════════════════════════════════════╝
`

func main() {
	// Parse command line flags
	configPath := flag.String("config", "configs/server.yaml", "Path to configuration file")
	apiOnly := flag.Bool("api-only", false, "Start API server only (no automatic capture)")
	flag.Parse()

	fmt.Println(banner)
	log.Printf("Starting TimeLapse Server...")
	log.Printf("Config file: %s", *configPath)

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	log.Printf("Configuration loaded successfully")
	log.Printf("  Server: %s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("  Storage: %s (%s)", cfg.Storage.Type, cfg.Storage.BasePath)
	log.Printf("  Cameras: %d configured", len(cfg.Cameras))
	log.Printf("  Log level: %s", cfg.Logging.Level)

	// Initialize storage backend
	var storageBackend storage.Backend
	switch cfg.Storage.Type {
	case "local":
		storageBackend = storage.NewLocalStorage(cfg.Storage.BasePath)
		log.Printf("Initialized local storage at: %s", cfg.Storage.BasePath)
	default:
		log.Fatalf("Unsupported storage type: %s", cfg.Storage.Type)
	}

	// Initialize camera persistence store
	// Store cameras.json in the data directory (parent of captures)
	dataDir := filepath.Dir(cfg.Storage.BasePath)
	camerasFilePath := filepath.Join(dataDir, "cameras.json")
	cameraStore := persistence.NewCameraStore(camerasFilePath)
	log.Printf("Camera persistence: %s", camerasFilePath)

	// Create camera manager with persistence
	mgr := manager.NewManagerWithPersistence(storageBackend, cameraStore)

	// Add cameras from config file (source: config)
	if !*apiOnly && len(cfg.Cameras) > 0 {
		log.Printf("\n╔════════════════════════════════════════════════════════╗")
		log.Printf("║ Config File Cameras                                    ║")
		log.Printf("╠════════════════════════════════════════════════════════╣")

		for i, camCfg := range cfg.Cameras {
			camera := camCfg.ToCamera()

			log.Printf("║ Camera %d: %-45s ║", i+1, camera.Name)
			log.Printf("║   UUID:     %s ║", camera.UUID)
			log.Printf("║   Type:     %-43s ║", camera.Type)
			log.Printf("║   URL:      %-43s ║", truncate(camera.ConnectionURL, 43))
			log.Printf("║   Interval: %-43s ║", camera.Schedule.Interval)
			log.Printf("║   Enabled:  %-43v ║", camera.Enabled)
			log.Printf("║   Source:   %-43s ║", "config")

			if err := mgr.AddCameraWithSource(camera, manager.SourceConfig); err != nil {
				log.Printf("║   ❌ Error: %-42s ║", truncate(err.Error(), 42))
			}

			if i < len(cfg.Cameras)-1 {
				log.Printf("╟────────────────────────────────────────────────────────╢")
			}
		}
		log.Printf("╚════════════════════════════════════════════════════════╝")
	}

	// Load persisted cameras (source: api)
	persistedCameras, err := cameraStore.Load()
	if err != nil {
		log.Printf("⚠ Warning: failed to load persisted cameras: %v", err)
	} else if len(persistedCameras) > 0 {
		log.Printf("\n╔════════════════════════════════════════════════════════╗")
		log.Printf("║ Persisted Cameras (API-added)                          ║")
		log.Printf("╠════════════════════════════════════════════════════════╣")

		loaded := 0
		for i, camera := range persistedCameras {
			// Skip if UUID already exists (config takes precedence)
			if _, err := mgr.GetCamera(camera.UUID); err == nil {
				log.Printf("║ Skipping %s (already in config) ║", truncate(camera.Name, 35))
				continue
			}

			log.Printf("║ Camera: %-47s ║", camera.Name)
			log.Printf("║   UUID:     %s ║", camera.UUID)
			log.Printf("║   Type:     %-43s ║", camera.Type)
			log.Printf("║   URL:      %-43s ║", truncate(camera.ConnectionURL, 43))
			log.Printf("║   Interval: %-43s ║", camera.Schedule.Interval)
			log.Printf("║   Enabled:  %-43v ║", camera.Enabled)
			log.Printf("║   Source:   %-43s ║", "api (persisted)")

			if err := mgr.AddCameraWithSource(camera, manager.SourceAPI); err != nil {
				log.Printf("║   ❌ Error: %-42s ║", truncate(err.Error(), 42))
			} else {
				loaded++
			}

			if i < len(persistedCameras)-1 {
				log.Printf("╟────────────────────────────────────────────────────────╢")
			}
		}
		log.Printf("╚════════════════════════════════════════════════════════╝")
		log.Printf("Loaded %d persisted camera(s)", loaded)
	}

	// Setup context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start camera manager (if we have cameras)
	if mgr.CameraCount() > 0 {
		if err := mgr.Start(ctx); err != nil {
			log.Fatalf("Failed to start camera manager: %v", err)
		}
	}

	// Create and start API server
	apiServer := api.NewServer(cfg.Server.Host, cfg.Server.Port, mgr, storageBackend)
	if err := apiServer.Start(); err != nil {
		log.Fatalf("Failed to start API server: %v", err)
	}

	log.Printf("\n✓ Server started successfully!")
	log.Printf("✓ API server: http://%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("✓ %d camera(s) configured", mgr.CameraCount())
	if *apiOnly {
		log.Printf("✓ Running in API-only mode (no automatic capture)")
	}
	log.Printf("✓ Press Ctrl+C to stop\n")

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Wait for shutdown signal
	sig := <-sigChan
	log.Printf("\n\n🛑 Shutdown signal received (%s)...", sig)

	// Cancel context to signal all goroutines
	cancel()

	// Stop API server
	if err := apiServer.Stop(ctx); err != nil {
		log.Printf("⚠ Error stopping API server: %v", err)
	}

	// Stop manager (waits for all workers to stop)
	mgr.Stop()

	// Print final statistics
	stats := mgr.GetStats()
	if len(stats) > 0 {
		log.Printf("\n📊 Final Statistics:")
		log.Printf("╔════════════════════════════════════════════════════════╗")
		for _, s := range stats {
			log.Printf("║ Camera: %-47s ║", s.CameraName)
			log.Printf("║   Total captures:      %-32d ║", s.TotalCaptures)
			log.Printf("║   Successful:          %-32d ║", s.SuccessfulCaptures)
			log.Printf("║   Failed:              %-32d ║", s.FailedCaptures)
			if s.LastCaptureTime != nil {
				log.Printf("║   Last capture:        %-32s ║", s.LastCaptureTime.Format("2006-01-02 15:04:05"))
			}
			if s.LastError != "" {
				log.Printf("║   Last error:          %-32s ║", truncate(s.LastError, 32))
			}
		}
		log.Printf("╚════════════════════════════════════════════════════════╝")
	}

	// Get storage stats
	storageStats, err := storageBackend.GetStats(context.Background())
	if err == nil {
		log.Printf("\n📦 Storage Statistics:")
		log.Printf("   Total images: %d", storageStats.TotalImages)
		log.Printf("   Total size: %d bytes", storageStats.TotalSize)
	}

	log.Printf("\n✓ Server stopped gracefully")
}

// truncate truncates a string to max length, adding "..." if needed
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
