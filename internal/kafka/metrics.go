package kafka

import (
	"sync"
	"time"

	"github.com/flexprice/flexprice/internal/logger"
)

// PrometheusMetricsCollector implements MetricsCollector using Prometheus-style metrics
type PrometheusMetricsCollector struct {
	logger *logger.Logger

	// Counters
	batchSizeTotal  int64
	batchErrorTotal int64
	retryTotal      int64

	// Gauges
	bufferSizeCurrent int64

	// Histograms (simplified as averages for now)
	batchDurationTotal time.Duration
	batchCount         int64

	mu sync.RWMutex
}

// NewPrometheusMetricsCollector creates a new metrics collector
func NewPrometheusMetricsCollector(logger *logger.Logger) *PrometheusMetricsCollector {
	return &PrometheusMetricsCollector{
		logger: logger,
	}
}

// RecordBatchSize records the size of a processed batch
func (m *PrometheusMetricsCollector) RecordBatchSize(size int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.batchSizeTotal += int64(size)
	m.batchCount++

	m.logger.Debugw("metrics: batch size recorded",
		"size", size,
		"total_batch_size", m.batchSizeTotal,
		"batch_count", m.batchCount,
	)
}

// RecordBatchDuration records the duration of batch processing
func (m *PrometheusMetricsCollector) RecordBatchDuration(duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.batchDurationTotal += duration

	m.logger.Debugw("metrics: batch duration recorded",
		"duration", duration,
		"total_duration", m.batchDurationTotal,
		"batch_count", m.batchCount,
	)
}

// RecordBatchError records a batch processing error
func (m *PrometheusMetricsCollector) RecordBatchError() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.batchErrorTotal++

	m.logger.Debugw("metrics: batch error recorded",
		"error_count", m.batchErrorTotal,
	)
}

// RecordBufferSize records the current buffer size
func (m *PrometheusMetricsCollector) RecordBufferSize(size int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.bufferSizeCurrent = int64(size)

	// Only log buffer size changes every 10 messages to avoid spam
	if size%10 == 0 {
		m.logger.Debugw("metrics: buffer size recorded",
			"size", size,
		)
	}
}

// RecordRetry records a retry attempt
func (m *PrometheusMetricsCollector) RecordRetry() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.retryTotal++

	m.logger.Debugw("metrics: retry recorded",
		"retry_count", m.retryTotal,
	)
}

// GetMetrics returns current metrics snapshot
func (m *PrometheusMetricsCollector) GetMetrics() MetricsSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	avgBatchSize := float64(0)
	avgBatchDuration := time.Duration(0)

	if m.batchCount > 0 {
		avgBatchSize = float64(m.batchSizeTotal) / float64(m.batchCount)
		avgBatchDuration = m.batchDurationTotal / time.Duration(m.batchCount)
	}

	return MetricsSnapshot{
		TotalBatchSize:       m.batchSizeTotal,
		TotalBatchErrors:     m.batchErrorTotal,
		TotalRetries:         m.retryTotal,
		CurrentBufferSize:    m.bufferSizeCurrent,
		BatchCount:           m.batchCount,
		AverageBatchSize:     avgBatchSize,
		AverageBatchDuration: avgBatchDuration,
	}
}

// MetricsSnapshot represents a snapshot of current metrics
type MetricsSnapshot struct {
	TotalBatchSize       int64         `json:"total_batch_size"`
	TotalBatchErrors     int64         `json:"total_batch_errors"`
	TotalRetries         int64         `json:"total_retries"`
	CurrentBufferSize    int64         `json:"current_buffer_size"`
	BatchCount           int64         `json:"batch_count"`
	AverageBatchSize     float64       `json:"average_batch_size"`
	AverageBatchDuration time.Duration `json:"average_batch_duration"`
}

// NoOpMetricsCollector is a no-op implementation for when metrics are disabled
type NoOpMetricsCollector struct{}

// NewNoOpMetricsCollector creates a no-op metrics collector
func NewNoOpMetricsCollector() *NoOpMetricsCollector {
	return &NoOpMetricsCollector{}
}

func (m *NoOpMetricsCollector) RecordBatchSize(size int)                   {}
func (m *NoOpMetricsCollector) RecordBatchDuration(duration time.Duration) {}
func (m *NoOpMetricsCollector) RecordBatchError()                          {}
func (m *NoOpMetricsCollector) RecordBufferSize(size int)                  {}
func (m *NoOpMetricsCollector) RecordRetry()                               {}
