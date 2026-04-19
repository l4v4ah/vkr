package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/zap"
)

// fakePublisher satisfies the handler's publisher interface without a NATS server.
type fakePublisher struct{ subjects []string }

func (f *fakePublisher) Publish(_ context.Context, subject string, _ any) error {
	f.subjects = append(f.subjects, subject)
	return nil
}

func setupTestRouter(p publisher) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &handler{
		nc:     p,
		tracer: noop.NewTracerProvider().Tracer("test"),
		log:    zap.NewNop(),
	}
	r.POST("/api/v1/metrics", h.receiveMetrics)
	r.POST("/api/v1/logs", h.receiveLogs)
	r.POST("/api/v1/traces", h.receiveSpans)
	return r
}

func TestReceiveMetrics_ValidPayload(t *testing.T) {
	pub := &fakePublisher{}
	r := setupTestRouter(pub)

	body, _ := json.Marshal(metricRequest{
		ServiceName: "payment-service",
		MetricName:  "request_duration_seconds",
		Value:       0.123,
		Timestamp:   time.Now(),
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/metrics", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusAccepted, w.Code)
	assert.Len(t, pub.subjects, 1)
	assert.Equal(t, "telemetry.metrics", pub.subjects[0])
}

func TestReceiveMetrics_MissingRequiredFields(t *testing.T) {
	pub := &fakePublisher{}
	r := setupTestRouter(pub)

	body, _ := json.Marshal(map[string]any{"value": 1.0})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/metrics", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Empty(t, pub.subjects)
}

func TestReceiveLogs_ValidPayload(t *testing.T) {
	pub := &fakePublisher{}
	r := setupTestRouter(pub)

	body, _ := json.Marshal(logRequest{
		ServiceName: "auth-service",
		Level:       "error",
		Message:     "token expired",
		TraceID:     "trace-abc",
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/logs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusAccepted, w.Code)
	assert.Equal(t, "telemetry.logs", pub.subjects[0])
}

func TestReceiveSpans_ValidPayload(t *testing.T) {
	pub := &fakePublisher{}
	r := setupTestRouter(pub)

	now := time.Now()
	body, _ := json.Marshal(spanRequest{
		TraceID:       "trace-xyz",
		SpanID:        "span-001",
		ServiceName:   "order-service",
		OperationName: "create_order",
		StartTime:     now,
		EndTime:       now.Add(50 * time.Millisecond),
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/traces", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusAccepted, w.Code)
	assert.Equal(t, "telemetry.spans", pub.subjects[0])
}
