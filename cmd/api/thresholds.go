package main

import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
)

// ThresholdConfig holds warn thresholds for system metrics (percent values).
type ThresholdConfig struct {
	CPUWarn  float64 `json:"cpu_warn"`
	MemWarn  float64 `json:"mem_warn"`
	DiskWarn float64 `json:"disk_warn"`
}

type thresholdStore struct {
	mu  sync.RWMutex
	cfg ThresholdConfig
}

func newThresholdStore() *thresholdStore {
	return &thresholdStore{cfg: ThresholdConfig{CPUWarn: 85, MemWarn: 90, DiskWarn: 90}}
}

func (s *thresholdStore) Get() ThresholdConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg
}

func (s *thresholdStore) handleGet(c *gin.Context) {
	c.JSON(http.StatusOK, s.Get())
}

func (s *thresholdStore) handleSet(c *gin.Context) {
	var cfg ThresholdConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if cfg.CPUWarn < 1 || cfg.CPUWarn > 100 ||
		cfg.MemWarn < 1 || cfg.MemWarn > 100 ||
		cfg.DiskWarn < 1 || cfg.DiskWarn > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "thresholds must be between 1 and 100"})
		return
	}
	s.mu.Lock()
	s.cfg = cfg
	s.mu.Unlock()
	c.JSON(http.StatusOK, s.Get())
}
