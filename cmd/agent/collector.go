package main

import (
	"context"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
	"go.uber.org/zap"
)

type systemCollector struct {
	serviceName string
	client      *collectorClient
	log         *zap.Logger
}

func newSystemCollector(serviceName string, client *collectorClient, log *zap.Logger) *systemCollector {
	return &systemCollector{serviceName: serviceName, client: client, log: log}
}

// Run starts one goroutine per metric group and blocks until ctx is cancelled.
// Each goroutine streams at its natural measurement rate — no artificial ticker.
func (s *systemCollector) Run(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Add(4)
	go func() { defer wg.Done(); s.streamCPU(ctx) }()
	go func() { defer wg.Done(); s.streamMemory(ctx) }()
	go func() { defer wg.Done(); s.streamDisk(ctx) }()
	go func() { defer wg.Done(); s.streamNetwork(ctx) }()
	wg.Wait()
}

// streamCPU measures CPU by blocking for 500 ms per sample — that IS the interval.
// No sleep needed: the measurement window itself throttles the loop.
func (s *systemCollector) streamCPU(ctx context.Context) {
	base := map[string]string{"host": hostname()}
	counts, _ := cpu.CountsWithContext(ctx, false)

	for {
		if ctx.Err() != nil {
			return
		}
		// cpu.Percent with non-zero interval blocks for that duration
		percents, err := cpu.PercentWithContext(ctx, 500*time.Millisecond, false)
		if err != nil || len(percents) == 0 {
			continue
		}
		now := time.Now().UTC()
		s.send(ctx, metricBatch{
			s.pt("cpu_usage_percent", percents[0], base, now),
			s.pt("cpu_count", float64(counts), base, now),
		})
	}
}

// streamMemory polls every 1 s — memory doesn't need sub-second granularity.
func (s *systemCollector) streamMemory(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(1 * time.Second):
		}
		v, err := mem.VirtualMemoryWithContext(ctx)
		if err != nil {
			continue
		}
		now := time.Now().UTC()
		base := map[string]string{"host": hostname()}
		s.send(ctx, metricBatch{
			s.pt("mem_total_bytes", float64(v.Total), base, now),
			s.pt("mem_used_bytes", float64(v.Used), base, now),
			s.pt("mem_available_bytes", float64(v.Available), base, now),
			s.pt("mem_usage_percent", v.UsedPercent, base, now),
		})
	}
}

// streamDisk polls every 2 s — disk usage changes slowly.
func (s *systemCollector) streamDisk(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
		partitions, err := disk.PartitionsWithContext(ctx, false)
		if err != nil {
			continue
		}
		now := time.Now().UTC()
		var batch metricBatch
		for _, p := range partitions {
			usage, err := disk.UsageWithContext(ctx, p.Mountpoint)
			if err != nil {
				continue
			}
			labels := map[string]string{
				"host": hostname(), "mountpoint": p.Mountpoint, "device": p.Device,
			}
			batch = append(batch,
				s.pt("disk_total_bytes", float64(usage.Total), labels, now),
				s.pt("disk_used_bytes", float64(usage.Used), labels, now),
				s.pt("disk_free_bytes", float64(usage.Free), labels, now),
				s.pt("disk_usage_percent", usage.UsedPercent, labels, now),
			)
		}
		if len(batch) > 0 {
			s.send(ctx, batch)
		}
	}
}

// streamNetwork calculates per-second rates by taking two snapshots 500 ms apart.
// The delta divided by elapsed gives bytes/s with high resolution.
func (s *systemCollector) streamNetwork(ctx context.Context) {
	prev, prevTime := netSnapshot(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(500 * time.Millisecond):
		}

		cur, curTime := netSnapshot(ctx)
		if cur == nil {
			continue
		}
		elapsed := curTime.Sub(prevTime).Seconds()
		if elapsed <= 0 {
			prev, prevTime = cur, curTime
			continue
		}

		now := time.Now().UTC()
		var batch metricBatch
		for name, c := range cur {
			p, ok := prev[name]
			if !ok {
				continue
			}
			labels := map[string]string{"host": hostname(), "interface": name}
			batch = append(batch,
				s.pt("net_bytes_sent_total", float64(c.BytesSent), labels, now),
				s.pt("net_bytes_recv_total", float64(c.BytesRecv), labels, now),
				s.pt("net_bytes_sent_per_sec", float64(c.BytesSent-p.BytesSent)/elapsed, labels, now),
				s.pt("net_bytes_recv_per_sec", float64(c.BytesRecv-p.BytesRecv)/elapsed, labels, now),
			)
		}
		if len(batch) > 0 {
			s.send(ctx, batch)
		}
		prev, prevTime = cur, curTime
	}
}

func netSnapshot(ctx context.Context) (map[string]net.IOCountersStat, time.Time) {
	stats, err := net.IOCountersWithContext(ctx, true)
	if err != nil {
		return nil, time.Time{}
	}
	m := make(map[string]net.IOCountersStat, len(stats))
	for _, s := range stats {
		if s.Name != "lo" {
			m[s.Name] = s
		}
	}
	return m, time.Now().UTC()
}

func (s *systemCollector) pt(name string, value float64, labels map[string]string, t time.Time) metricPoint {
	return metricPoint{ServiceName: s.serviceName, MetricName: name, Value: value, Labels: labels, Timestamp: t}
}

func (s *systemCollector) send(ctx context.Context, batch metricBatch) {
	if err := s.client.sendBatch(ctx, batch); err != nil {
		s.log.Warn("send batch", zap.String("metric", batch[0].MetricName), zap.Error(err))
	}
}
