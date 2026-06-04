package app

import (
	"context"
	"net/http"
	"time"
	"vpnpanel/internal/broker"
	"vpnpanel/internal/db"

	"github.com/gin-gonic/gin"
)

const healthTimeout = 500 * time.Millisecond

func (s *Server) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"service": "corvin-ui",
	})
}

func (s *Server) Ready(c *gin.Context) {
	checks := gin.H{
		"status":   "ok",
		"db":       "ok",
		"rabbitmq": "skipped",
		"minio":    "skipped",
	}

	overall := "ok"

	if err := pingDB(); err != nil {
		checks["db"] = "error"
		overall = "error"
	}

	if s.Config.RabbitMQ.URL != "" || broker.GlobalProducer != nil {
		if broker.IsReady() {
			checks["rabbitmq"] = "ok"
		} else {
			checks["rabbitmq"] = "error"
			if overall == "ok" {
				overall = "degraded"
			}
		}
	}

	if s.StorageRepo != nil {
		ctx, cancel := context.WithTimeout(c.Request.Context(), healthTimeout)
		defer cancel()

		if err := s.StorageRepo.Ping(ctx); err != nil {
			checks["minio"] = "error"
			if overall == "ok" {
				overall = "degraded"
			}
		} else {
			checks["minio"] = "ok"
		}
	}

	checks["status"] = overall

	statusCode := http.StatusOK
	if overall == "error" {
		statusCode = http.StatusServiceUnavailable
	}

	c.JSON(statusCode, checks)
}

func pingDB() error {
	if db.DB == nil {
		return errDependencyNotConfigured
	}

	sqlDB, err := db.DB.DB()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), healthTimeout)
	defer cancel()

	return sqlDB.PingContext(ctx)
}

type dependencyError string

func (e dependencyError) Error() string {
	return string(e)
}

const errDependencyNotConfigured dependencyError = "dependency is not configured"
