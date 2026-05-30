package api

import (
	"github.com/gin-gonic/gin"
	"github.com/kmala/timelapse/internal/api/handlers"
	"github.com/kmala/timelapse/internal/api/middleware"
)

// SetupRouter configures all API routes
func (s *Server) SetupRouter() *gin.Engine {
	// Set Gin mode based on environment
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()

	// Global middleware
	router.Use(gin.Recovery())
	router.Use(middleware.CORS())
	router.Use(middleware.Logger())

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		// Discovery endpoints
		discovery := v1.Group("/discovery")
		{
			discoveryHandler := handlers.NewDiscoveryHandler()
			discovery.POST("/scan", discoveryHandler.Scan)
			discovery.GET("/results", discoveryHandler.GetResults)
			discovery.POST("/probe", discoveryHandler.Probe)
		}

		// Camera endpoints
		cameras := v1.Group("/cameras")
		{
			cameraHandler := handlers.NewCameraHandler(s.manager, s.storage)
			cameras.GET("", cameraHandler.List)
			cameras.POST("", cameraHandler.Create)
			cameras.GET("/:uuid", cameraHandler.Get)
			cameras.PUT("/:uuid", cameraHandler.Update)
			cameras.DELETE("/:uuid", cameraHandler.Delete)

			// Camera-specific profile endpoints
			cameras.GET("/:uuid/profiles", handlers.NewProfileHandler(s.manager).ListProfiles)
			cameras.PUT("/:uuid/profiles/:token", handlers.NewProfileHandler(s.manager).SelectProfile)

			// Camera-specific capture endpoints
			captureHandler := handlers.NewCaptureHandler(s.manager, s.storage)
			cameras.POST("/:uuid/start", captureHandler.StartCamera)
			cameras.POST("/:uuid/stop", captureHandler.StopCamera)
			cameras.POST("/:uuid/snapshot", captureHandler.TakeSnapshot)

			// Camera-specific image endpoints
			imageHandler := handlers.NewImageHandler(s.storage)
			cameras.GET("/:uuid/images", imageHandler.ListByCamera)

			// Camera-specific stats endpoints
			statsHandler := handlers.NewStatsHandler(s.manager, s.storage)
			cameras.GET("/:uuid/stats", statsHandler.GetCameraStats)
		}

		// Global image endpoints
		images := v1.Group("/images")
		{
			imageHandler := handlers.NewImageHandler(s.storage)
			images.GET("/:filename", imageHandler.Serve)
		}

		// Global stats endpoints
		stats := v1.Group("/stats")
		{
			statsHandler := handlers.NewStatsHandler(s.manager, s.storage)
			stats.GET("", statsHandler.GetGlobalStats)
			stats.GET("/storage", statsHandler.GetStorageStats)
		}
	}

	return router
}
