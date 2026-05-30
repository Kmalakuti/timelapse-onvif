package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kmala/timelapse/internal/manager"
	"github.com/kmala/timelapse/internal/storage"
)

// Server represents the HTTP API server
type Server struct {
	router     *gin.Engine
	httpServer *http.Server
	manager    *manager.Manager
	storage    storage.Backend
	host       string
	port       int
}

// NewServer creates a new API server
func NewServer(host string, port int, mgr *manager.Manager, storageBackend storage.Backend) *Server {
	s := &Server{
		manager: mgr,
		storage: storageBackend,
		host:    host,
		port:    port,
	}

	// Setup router
	s.router = s.SetupRouter()

	// Create HTTP server
	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", host, port),
		Handler:      s.router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s
}

// Start starts the HTTP server in a goroutine
func (s *Server) Start() error {
	log.Printf("🌐 Starting API server on http://%s:%d", s.host, s.port)

	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("❌ API server error: %v", err)
		}
	}()

	return nil
}

// Stop gracefully shuts down the HTTP server
func (s *Server) Stop(ctx context.Context) error {
	log.Printf("🛑 Stopping API server...")

	// Create a deadline context if not provided
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
	}

	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("server shutdown failed: %w", err)
	}

	log.Printf("✓ API server stopped")
	return nil
}

// Router returns the Gin router for testing
func (s *Server) Router() *gin.Engine {
	return s.router
}

// Address returns the server address
func (s *Server) Address() string {
	return fmt.Sprintf("http://%s:%d", s.host, s.port)
}
