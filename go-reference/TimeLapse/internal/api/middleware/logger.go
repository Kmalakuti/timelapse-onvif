package middleware

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"
)

// Logger returns a logging middleware that logs request details
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Start timer
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		// Process request
		c.Next()

		// Calculate latency
		latency := time.Since(start)

		// Get status code
		statusCode := c.Writer.Status()

		// Build log message
		if raw != "" {
			path = path + "?" + raw
		}

		// Color-code status
		statusIcon := "✓"
		if statusCode >= 400 && statusCode < 500 {
			statusIcon = "⚠"
		} else if statusCode >= 500 {
			statusIcon = "✗"
		}

		log.Printf("%s [%d] %s %s (%v)",
			statusIcon,
			statusCode,
			c.Request.Method,
			path,
			latency,
		)
	}
}
