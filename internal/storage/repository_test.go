package storage_test

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/slava-kov/monitoring-system/internal/storage"
)

func migrationsDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "migrations")
}

func startPostgres(ctx context.Context, t *testing.T) string {
	t.Helper()

	req := testcontainers.ContainerRequest{
		Image:        "postgres:16-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_DB":       "testdb",
			"POSTGRES_USER":     "test",
			"POSTGRES_PASSWORD": "test",
		},
		WaitingFor: wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).
			WithStartupTimeout(30 * time.Second),
	}

	pgc, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = pgc.Terminate(ctx) })

	host, err := pgc.Host(ctx)
	require.NoError(t, err)
	port, err := pgc.MappedPort(ctx, "5432")
	require.NoError(t, err)

	return fmt.Sprintf("postgres://test:test@%s:%s/testdb?sslmode=disable", host, port.Port())
}

func TestMetricRepository(t *testing.T) {
	ctx := context.Background()
	dsn := startPostgres(ctx, t)

	db, err := storage.Connect(ctx, dsn, migrationsDir())
	require.NoError(t, err)
	defer db.Close()

	now := time.Now().UTC().Truncate(time.Second)
	mp := storage.MetricPoint{
		ServiceName: "payment-service",
		MetricName:  "request_duration_seconds",
		Value:       0.123,
		Labels:      map[string]string{"method": "POST", "status": "200"},
		Timestamp:   now,
	}

	require.NoError(t, db.InsertMetric(ctx, mp))

	results, err := db.QueryMetrics(ctx, "payment-service", now.Add(-time.Minute), now.Add(time.Minute))
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, mp.ServiceName, results[0].ServiceName)
	assert.Equal(t, mp.MetricName, results[0].MetricName)
	assert.InDelta(t, mp.Value, results[0].Value, 1e-9)
	assert.Equal(t, mp.Labels, results[0].Labels)
}

func TestLogRepository(t *testing.T) {
	ctx := context.Background()
	dsn := startPostgres(ctx, t)

	db, err := storage.Connect(ctx, dsn, migrationsDir())
	require.NoError(t, err)
	defer db.Close()

	le := storage.LogEntry{
		ServiceName: "auth-service",
		Level:       "error",
		Message:     "invalid token",
		TraceID:     "trace-abc",
		Fields:      map[string]string{"user_id": "42"},
		Timestamp:   time.Now().UTC(),
	}
	require.NoError(t, db.InsertLog(ctx, le))

	results, err := db.QueryLogs(ctx, "auth-service", "error", 10)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, le.Message, results[0].Message)
	assert.Equal(t, le.TraceID, results[0].TraceID)
}

func TestSpanRepository(t *testing.T) {
	ctx := context.Background()
	dsn := startPostgres(ctx, t)

	db, err := storage.Connect(ctx, dsn, migrationsDir())
	require.NoError(t, err)
	defer db.Close()

	now := time.Now().UTC()
	s := storage.TraceSpan{
		TraceID:       "trace-xyz",
		SpanID:        "span-001",
		ServiceName:   "order-service",
		OperationName: "create_order",
		StartTime:     now,
		EndTime:       now.Add(50 * time.Millisecond),
		Status:        "ok",
		Attributes:    map[string]string{"db.system": "postgresql"},
	}
	require.NoError(t, db.InsertSpan(ctx, s))

	spans, err := db.QuerySpansByTrace(ctx, "trace-xyz")
	require.NoError(t, err)
	require.Len(t, spans, 1)
	assert.Equal(t, s.OperationName, spans[0].OperationName)
	assert.Equal(t, s.Attributes, spans[0].Attributes)
}
