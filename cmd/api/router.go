package main

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"github.com/slava-kov/monitoring-system/internal/metrics"
	"github.com/slava-kov/monitoring-system/internal/storage"
)

func newServer(addr string, db *storage.DB, m *metrics.ServiceMetrics, tracer trace.Tracer, log *zap.Logger) *http.Server {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	r.Use(func(c *gin.Context) {
		start := time.Now()
		c.Next()
		status := strconv.Itoa(c.Writer.Status())
		m.RequestsTotal.WithLabelValues(c.Request.Method, c.FullPath(), status).Inc()
		m.RequestDuration.WithLabelValues(c.Request.Method, c.FullPath()).Observe(time.Since(start).Seconds())
	})

	h := &apiHandler{db: db, tracer: tracer, log: log}

	v1 := r.Group("/api/v1")
	{
		v1.GET("/metrics", h.getMetrics)
		v1.GET("/logs", h.getLogs)
		v1.GET("/traces/:trace_id", h.getTrace)
	}

	r.GET("/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })
	r.GET("/metrics", gin.WrapH(m.Handler()))

	return &http.Server{Addr: addr, Handler: r}
}
