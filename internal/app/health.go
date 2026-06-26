package app

import (
	"context"
	"net/http"
	"time"
	"vpnpanel/internal/broker"
	"vpnpanel/internal/db"
	"vpnpanel/internal/logger"

	"github.com/gin-gonic/gin"
)

const healthTimeout = 500 * time.Millisecond

func (s *Server) Health(c *gin.Context) {
	requestID, _ := c.Get("request_id")
	logger.Info("health check started", "component", "http_api", "operation", "health", "request_id", requestID)
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"service": "corvin-ui",
	})
	logger.Info("health check finished", "component", "http_api", "operation", "health", "request_id", requestID, "status_code", http.StatusOK, "reason", "success")
}

func (s *Server) Ready(c *gin.Context) {
	requestID, _ := c.Get("request_id")
	logger.Info("readiness check started", "component", "http_api", "operation", "ready", "request_id", requestID)
	checks := gin.H{
		"status":   "ok",
		"db":       "ok",
		"rabbitmq": "skipped",
		"minio":    "skipped",
	}

	overall := "ok"

	logger.Info("readiness db ping started", "component", "http_api", "operation", "ready", "external_system", "database", "request_id", requestID)
	if err := pingDB(); err != nil {
		logger.Error("readiness db ping failed", err, "component", "http_api", "operation", "ready", "external_system", "database", "request_id", requestID)
		checks["db"] = "error"
		overall = "error"
	} else {
		logger.Info("readiness db ping succeeded", "component", "http_api", "operation", "ready", "external_system", "database", "request_id", requestID)
	}

	if s.Config.RabbitMQ.URL != "" || broker.GlobalProducer != nil {
		logger.Info("readiness rabbitmq check started", "component", "http_api", "operation", "ready", "external_system", "rabbitmq", "request_id", requestID)
		if broker.IsReady() {
			checks["rabbitmq"] = "ok"
			logger.Info("readiness rabbitmq check succeeded", "component", "http_api", "operation", "ready", "external_system", "rabbitmq", "request_id", requestID)
		} else {
			logger.Warn("readiness rabbitmq check failed", "component", "http_api", "operation", "ready", "external_system", "rabbitmq", "request_id", requestID, "reason", "not_ready")
			checks["rabbitmq"] = "error"
			if overall == "ok" {
				overall = "degraded"
			}
		}
	}

	if s.StorageRepo != nil {
		logger.Info("readiness minio ping started", "component", "http_api", "operation", "ready", "external_system", "minio", "request_id", requestID)
		ctx, cancel := context.WithTimeout(c.Request.Context(), healthTimeout)
		defer cancel()

		if err := s.StorageRepo.Ping(ctx); err != nil {
			logger.Error("readiness minio ping failed", err, "component", "http_api", "operation", "ready", "external_system", "minio", "request_id", requestID)
			checks["minio"] = "error"
			if overall == "ok" {
				overall = "degraded"
			}
		} else {
			checks["minio"] = "ok"
			logger.Info("readiness minio ping succeeded", "component", "http_api", "operation", "ready", "external_system", "minio", "request_id", requestID)
		}
	}

	checks["status"] = overall

	statusCode := http.StatusOK
	if overall == "error" {
		statusCode = http.StatusServiceUnavailable
	}

	c.JSON(statusCode, checks)
	logger.Info("readiness check finished", "component", "http_api", "operation", "ready", "request_id", requestID, "status_code", statusCode, "reason", overall)
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
