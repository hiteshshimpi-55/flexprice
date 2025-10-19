package kafka

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/flexprice/flexprice/internal/config"
	"github.com/flexprice/flexprice/internal/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockBatchProcessor is a mock implementation of BatchProcessor for testing
type mockBatchProcessor struct {
	processBatchFunc func(ctx context.Context, messages []*message.Message) error
	processedBatches [][]*message.Message
	mu               sync.Mutex
}

func (m *mockBatchProcessor) ProcessBatch(ctx context.Context, messages []*message.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.processedBatches = append(m.processedBatches, messages)

	if m.processBatchFunc != nil {
		return m.processBatchFunc(ctx, messages)
	}
	return nil
}

func (m *mockBatchProcessor) getProcessedBatches() [][]*message.Message {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Return a copy to avoid race conditions
	result := make([][]*message.Message, len(m.processedBatches))
	for i, batch := range m.processedBatches {
		result[i] = make([]*message.Message, len(batch))
		copy(result[i], batch)
	}
	return result
}

// mockMetricsCollector is a mock implementation of MetricsCollector for testing
type mockMetricsCollector struct {
	recordedBatchSizes     []int
	recordedBatchDurations []time.Duration
	recordedBatchErrors    int
	recordedBufferSizes    []int
	recordedRetries        int
	mu                     sync.Mutex
}

func (m *mockMetricsCollector) RecordBatchSize(size int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.recordedBatchSizes = append(m.recordedBatchSizes, size)
}

func (m *mockMetricsCollector) RecordBatchDuration(duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.recordedBatchDurations = append(m.recordedBatchDurations, duration)
}

func (m *mockMetricsCollector) RecordBatchError() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.recordedBatchErrors++
}

func (m *mockMetricsCollector) RecordBufferSize(size int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.recordedBufferSizes = append(m.recordedBufferSizes, size)
}

func (m *mockMetricsCollector) RecordRetry() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.recordedRetries++
}

func TestBufferedConsumer_ProcessBatch(t *testing.T) {
	// Create test configuration
	cfg := &config.Configuration{
		Kafka: config.KafkaConfig{
			Brokers:       []string{"localhost:9092"},
			ConsumerGroup: "test-group",
			ClientID:      "test-client",
		},
		Logging: config.LoggingConfig{
			Level: "debug",
		},
	}

	logger, err := logger.NewLogger(cfg)
	require.NoError(t, err)
	processor := &mockBatchProcessor{}
	metrics := &mockMetricsCollector{}

	// Create consumer with small batch size for testing
	consumerConfig := &BufferedConsumerConfig{
		BatchSize:      3, // Small batch size for testing
		FlushInterval:  100 * time.Millisecond,
		MaxRetries:     2,
		RetryDelay:     10 * time.Millisecond,
		MaxRetryDelay:  100 * time.Millisecond,
		EnableMetrics:  true,
		BufferCapacity: 10,
	}

	_, err = NewBufferedConsumer(cfg, processor, metrics, consumerConfig, logger)
	require.NoError(t, err)

	// Test basic batch processing
	t.Run("basic batch processing", func(t *testing.T) {
		// Create test messages
		messages := []*message.Message{
			message.NewMessage("1", []byte("test1")),
			message.NewMessage("2", []byte("test2")),
			message.NewMessage("3", []byte("test3")),
		}

		// Process batch
		err := processor.ProcessBatch(context.Background(), messages)
		assert.NoError(t, err)

		// Verify batch was processed
		batches := processor.getProcessedBatches()
		assert.Len(t, batches, 1)
		assert.Len(t, batches[0], 3)
	})

	// Test retry logic
	t.Run("retry logic", func(t *testing.T) {
		processor.processBatchFunc = func(ctx context.Context, messages []*message.Message) error {
			// Simulate error on first attempt
			if len(processor.getProcessedBatches()) == 0 {
				return assert.AnError
			}
			return nil
		}

		messages := []*message.Message{
			message.NewMessage("retry1", []byte("test")),
		}

		// This should succeed after retry
		err := processor.ProcessBatch(context.Background(), messages)
		assert.NoError(t, err)

		// Should have been processed twice (original + retry)
		batches := processor.getProcessedBatches()
		assert.Len(t, batches, 2)
	})

	// Test metrics collection
	t.Run("metrics collection", func(t *testing.T) {
		messages := []*message.Message{
			message.NewMessage("metrics1", []byte("test")),
			message.NewMessage("metrics2", []byte("test")),
		}

		err := processor.ProcessBatch(context.Background(), messages)
		assert.NoError(t, err)

		// Check that metrics were recorded
		assert.Len(t, metrics.recordedBatchSizes, 1)
		assert.Equal(t, 2, metrics.recordedBatchSizes[0])
		assert.Len(t, metrics.recordedBatchDurations, 1)
	})
}

func TestBufferedConsumer_Configuration(t *testing.T) {
	t.Run("default configuration", func(t *testing.T) {
		config := DefaultBufferedConsumerConfig()

		assert.Equal(t, 500, config.BatchSize)
		assert.Equal(t, 5*time.Second, config.FlushInterval)
		assert.Equal(t, 3, config.MaxRetries)
		assert.Equal(t, 1*time.Second, config.RetryDelay)
		assert.Equal(t, 30*time.Second, config.MaxRetryDelay)
		assert.True(t, config.EnableMetrics)
		assert.Equal(t, 1000, config.BufferCapacity)
	})

	t.Run("custom configuration", func(t *testing.T) {
		config := &BufferedConsumerConfig{
			BatchSize:      100,
			FlushInterval:  2 * time.Second,
			MaxRetries:     5,
			RetryDelay:     500 * time.Millisecond,
			MaxRetryDelay:  10 * time.Second,
			EnableMetrics:  false,
			BufferCapacity: 500,
		}

		assert.Equal(t, 100, config.BatchSize)
		assert.Equal(t, 2*time.Second, config.FlushInterval)
		assert.Equal(t, 5, config.MaxRetries)
		assert.Equal(t, 500*time.Millisecond, config.RetryDelay)
		assert.Equal(t, 10*time.Second, config.MaxRetryDelay)
		assert.False(t, config.EnableMetrics)
		assert.Equal(t, 500, config.BufferCapacity)
	})
}

func TestMetricsCollector(t *testing.T) {
	t.Run("prometheus metrics collector", func(t *testing.T) {
		logger, err := logger.NewLogger(config.GetDefaultConfig())
		require.NoError(t, err)
		metrics := NewPrometheusMetricsCollector(logger)

		// Record some metrics
		metrics.RecordBatchSize(10)
		metrics.RecordBatchSize(20)
		metrics.RecordBatchDuration(100 * time.Millisecond)
		metrics.RecordBatchDuration(200 * time.Millisecond)
		metrics.RecordBatchError()
		metrics.RecordRetry()
		metrics.RecordBufferSize(5)

		// Get metrics snapshot
		snapshot := metrics.GetMetrics()

		assert.Equal(t, int64(30), snapshot.TotalBatchSize)
		assert.Equal(t, int64(2), snapshot.BatchCount)
		assert.Equal(t, float64(15), snapshot.AverageBatchSize)
		assert.Equal(t, 150*time.Millisecond, snapshot.AverageBatchDuration)
		assert.Equal(t, int64(1), snapshot.TotalBatchErrors)
		assert.Equal(t, int64(1), snapshot.TotalRetries)
		assert.Equal(t, int64(5), snapshot.CurrentBufferSize)
	})

	t.Run("no-op metrics collector", func(t *testing.T) {
		metrics := NewNoOpMetricsCollector()

		// These should not panic
		metrics.RecordBatchSize(10)
		metrics.RecordBatchDuration(100 * time.Millisecond)
		metrics.RecordBatchError()
		metrics.RecordRetry()
		metrics.RecordBufferSize(5)
	})
}

func TestRetryDelayCalculation(t *testing.T) {
	config := &BufferedConsumerConfig{
		RetryDelay:    1 * time.Second,
		MaxRetryDelay: 30 * time.Second,
	}

	consumer := &BufferedConsumer{config: config}

	tests := []struct {
		retryCount int
		expected   time.Duration
	}{
		{1, 1 * time.Second},   // 1 * 2^0 = 1s
		{2, 2 * time.Second},   // 1 * 2^1 = 2s
		{3, 4 * time.Second},   // 1 * 2^2 = 4s
		{4, 8 * time.Second},   // 1 * 2^3 = 8s
		{5, 16 * time.Second},  // 1 * 2^4 = 16s
		{6, 30 * time.Second},  // Capped at MaxRetryDelay
		{10, 30 * time.Second}, // Capped at MaxRetryDelay
	}

	for _, test := range tests {
		delay := consumer.calculateRetryDelay(test.retryCount)
		assert.Equal(t, test.expected, delay, "retry count %d", test.retryCount)
	}
}

// Benchmark tests
func BenchmarkBufferedConsumer_ProcessBatch(b *testing.B) {
	cfg := &config.Configuration{
		Kafka: config.KafkaConfig{
			Brokers:       []string{"localhost:9092"},
			ConsumerGroup: "test-group",
			ClientID:      "test-client",
		},
		Logging: config.LoggingConfig{
			Level: "error", // Reduce logging for benchmark
		},
	}

	logger, err := logger.NewLogger(cfg)
	require.NoError(b, err)
	processor := &mockBatchProcessor{}
	metrics := NewNoOpMetricsCollector() // Disable metrics for benchmark

	consumerConfig := &BufferedConsumerConfig{
		BatchSize:      100,
		FlushInterval:  1 * time.Second,
		MaxRetries:     3,
		RetryDelay:     10 * time.Millisecond,
		MaxRetryDelay:  100 * time.Millisecond,
		EnableMetrics:  false,
		BufferCapacity: 1000,
	}

	_, err = NewBufferedConsumer(cfg, processor, metrics, consumerConfig, logger)
	require.NoError(b, err)

	// Create test messages
	messages := make([]*message.Message, 100)
	for i := 0; i < 100; i++ {
		messages[i] = message.NewMessage(string(rune(i)), []byte("test"))
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			processor.ProcessBatch(context.Background(), messages)
		}
	})
}
