package main

import (
	"context"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/slava-kov/monitoring-system/gen/telemetry"
)

type collectorClient struct {
	pb pb.CollectorServiceClient
}

func newCollectorClient(target string) (*collectorClient, error) {
	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &collectorClient{pb: pb.NewCollectorServiceClient(conn)}, nil
}

// metricBatch is a batch of metrics collected during one interval.
type metricBatch []metricPoint

type metricPoint struct {
	ServiceName string
	MetricName  string
	Value       float64
	Labels      map[string]string
	Timestamp   time.Time
}

func (c *collectorClient) sendBatch(ctx context.Context, batch metricBatch) error {
	pbMetrics := make([]*pb.MetricPoint, 0, len(batch))
	for _, m := range batch {
		pbMetrics = append(pbMetrics, &pb.MetricPoint{
			ServiceName: m.ServiceName,
			MetricName:  m.MetricName,
			Value:       m.Value,
			Labels:      m.Labels,
			Timestamp:   timestamppb.New(m.Timestamp),
		})
	}
	_, err := c.pb.SendMetrics(ctx, &pb.SendMetricsRequest{Metrics: pbMetrics})
	return err
}

var _hostname string

func hostname() string {
	if _hostname == "" {
		h, err := os.Hostname()
		if err != nil {
			h = "unknown"
		}
		_hostname = h
	}
	return _hostname
}
