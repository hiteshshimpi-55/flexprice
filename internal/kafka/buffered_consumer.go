package kafka

import (
	"context"
	"sync"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/flexprice/flexprice/internal/config"
	"github.com/flexprice/flexprice/internal/logger"
)

// BufferedConsumerConfig holds configuration for the buffered consumer
type BufferedConsumerConfig struct {
	// BatchSize is the maximum number of messages to buffer before processing
	BatchSize int
	// FlushInterval is the maximum time to wait before flushing the buffer
	FlushInterval time.Duration
	// MaxRetries is the maximum number of retries for failed batches
	MaxRetries int
	// RetryDelay is the initial delay between retries
	RetryDelay time.Duration
	// MaxRetryDelay is the maximum delay between retries
	MaxRetryDelay time.Duration
	// EnableMetrics enables metrics collection
	EnableMetrics bool
	// BufferCapacity is the capacity of the internal buffer channel
	BufferCapacity int
}

// DefaultBufferedConsumerConfig returns a default configuration
func DefaultBufferedConsumerConfig() *BufferedConsumerConfig {
	return &BufferedConsumerConfig{
		BatchSize:      500,
		FlushInterval:  5 * time.Second,
		MaxRetries:     3,
		RetryDelay:     1 * time.Second,
		MaxRetryDelay:  30 * time.Second,
		EnableMetrics:  true,
		BufferCapacity: 1000,
	}
}

// BatchProcessor defines the interface for processing batches of messages
type BatchProcessor interface {
	// ProcessBatch processes a batch of messages
	// Returns an error if the entire batch should be retried
	ProcessBatch(ctx context.Context, messages []*message.Message) error
}

// MetricsCollector defines the interface for collecting metrics
type MetricsCollector interface {
	// RecordBatchSize records the size of a processed batch
	RecordBatchSize(size int)
	// RecordBatchDuration records the duration of batch processing
	RecordBatchDuration(duration time.Duration)
	// RecordBatchError records a batch processing error
	RecordBatchError()
	// RecordBufferSize records the current buffer size
	RecordBufferSize(size int)
	// RecordRetry records a retry attempt
	RecordRetry()
}

// BufferedConsumer handles buffered consumption of Kafka messages
type BufferedConsumer struct {
	subscriber   message.Subscriber
	processor    BatchProcessor
	metrics      MetricsCollector
	config       *BufferedConsumerConfig
	logger       *logger.Logger
	buffer       chan *message.Message
	shutdown     chan struct{}
	shutdownOnce sync.Once
	wg           sync.WaitGroup
	ctx          context.Context
	cancel       context.CancelFunc
}

// NewBufferedConsumer creates a new buffered consumer
func NewBufferedConsumer(
	cfg *config.Configuration,
	processor BatchProcessor,
	metrics MetricsCollector,
	consumerConfig *BufferedConsumerConfig,
	logger *logger.Logger,
) (*BufferedConsumer, error) {
	if consumerConfig == nil {
		consumerConfig = DefaultBufferedConsumerConfig()
	}

	// Create a regular consumer
	consumer, err := NewConsumer(cfg)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &BufferedConsumer{
		subscriber: consumer.(*Consumer).subscriber,
		processor:  processor,
		metrics:    metrics,
		config:     consumerConfig,
		logger:     logger,
		buffer:     make(chan *message.Message, consumerConfig.BufferCapacity),
		shutdown:   make(chan struct{}),
		ctx:        ctx,
		cancel:     cancel,
	}, nil
}

// Start begins consuming messages from the specified topic
func (bc *BufferedConsumer) Start(topic string) error {
	bc.logger.Infow("starting buffered consumer",
		"topic", topic,
		"batch_size", bc.config.BatchSize,
		"flush_interval", bc.config.FlushInterval,
	)

	// Subscribe to the topic
	messages, err := bc.subscriber.Subscribe(bc.ctx, topic)
	if err != nil {
		return err
	}

	// Start the message processing goroutine
	bc.wg.Add(1)
	go bc.consumeMessages(messages)

	// Start the batch processing goroutine
	bc.wg.Add(1)
	go bc.processBatches()

	return nil
}

// Stop gracefully stops the consumer
func (bc *BufferedConsumer) Stop() error {
	bc.shutdownOnce.Do(func() {
		bc.logger.Info("stopping buffered consumer")
		close(bc.shutdown)
		bc.cancel()
	})

	// Wait for all goroutines to finish
	bc.wg.Wait()

	// Close the subscriber
	if err := bc.subscriber.Close(); err != nil {
		bc.logger.Errorw("error closing subscriber", "error", err)
		return err
	}

	bc.logger.Info("buffered consumer stopped")
	return nil
}

// consumeMessages handles incoming messages and adds them to the buffer
func (bc *BufferedConsumer) consumeMessages(messages <-chan *message.Message) {
	defer bc.wg.Done()

	for {
		select {
		case <-bc.ctx.Done():
			bc.logger.Info("message consumption stopped")
			return
		case msg, ok := <-messages:
			if !ok {
				bc.logger.Info("message channel closed")
				return
			}

			select {
			case bc.buffer <- msg:
				// Message added to buffer successfully
				if bc.metrics != nil {
					bc.metrics.RecordBufferSize(len(bc.buffer))
				}
			case <-bc.ctx.Done():
				bc.logger.Info("context cancelled while adding message to buffer")
				return
			}
		}
	}
}

// processBatches processes buffered messages in batches
func (bc *BufferedConsumer) processBatches() {
	defer bc.wg.Done()

	ticker := time.NewTicker(bc.config.FlushInterval)
	defer ticker.Stop()

	batch := make([]*message.Message, 0, bc.config.BatchSize)

	for {
		select {
		case <-bc.ctx.Done():
			// Process remaining messages in buffer
			if len(batch) > 0 {
				bc.processBatch(batch)
			}
			bc.logger.Info("batch processing stopped")
			return

		case <-bc.shutdown:
			// Process remaining messages in buffer
			if len(batch) > 0 {
				bc.processBatch(batch)
			}
			bc.logger.Info("batch processing stopped due to shutdown")
			return

		case msg := <-bc.buffer:
			batch = append(batch, msg)
			if bc.metrics != nil {
				bc.metrics.RecordBufferSize(len(bc.buffer))
			}

			// Process batch if it reaches the size threshold
			if len(batch) >= bc.config.BatchSize {
				bc.processBatch(batch)
				batch = batch[:0] // Reset slice but keep capacity
			}

		case <-ticker.C:
			// Process batch if there are any messages
			if len(batch) > 0 {
				bc.processBatch(batch)
				batch = batch[:0] // Reset slice but keep capacity
			}
		}
	}
}

// processBatch processes a batch of messages with retry logic
func (bc *BufferedConsumer) processBatch(batch []*message.Message) {
	if len(batch) == 0 {
		return
	}

	startTime := time.Now()
	retryCount := 0

	for {
		err := bc.processor.ProcessBatch(bc.ctx, batch)
		if err == nil {
			// Success - acknowledge all messages
			bc.acknowledgeMessages(batch)

			// Record metrics
			if bc.metrics != nil {
				bc.metrics.RecordBatchSize(len(batch))
				bc.metrics.RecordBatchDuration(time.Since(startTime))
			}

			bc.logger.Debugw("batch processed successfully",
				"batch_size", len(batch),
				"duration", time.Since(startTime),
			)
			return
		}

		// Record error metrics
		if bc.metrics != nil {
			bc.metrics.RecordBatchError()
		}

		retryCount++
		if retryCount > bc.config.MaxRetries {
			bc.logger.Errorw("batch processing failed after max retries",
				"batch_size", len(batch),
				"retry_count", retryCount,
				"error", err,
			)
			// TODO: Send to dead letter queue or handle failure
			bc.nacknowledgeMessages(batch)
			return
		}

		// Calculate retry delay with exponential backoff
		delay := bc.calculateRetryDelay(retryCount)
		bc.logger.Warnw("batch processing failed, retrying",
			"batch_size", len(batch),
			"retry_count", retryCount,
			"delay", delay,
			"error", err,
		)

		if bc.metrics != nil {
			bc.metrics.RecordRetry()
		}

		select {
		case <-time.After(delay):
			// Continue to retry
		case <-bc.ctx.Done():
			bc.logger.Info("context cancelled during retry")
			return
		}
	}
}

// calculateRetryDelay calculates the delay for the next retry using exponential backoff
func (bc *BufferedConsumer) calculateRetryDelay(retryCount int) time.Duration {
	delay := bc.config.RetryDelay * time.Duration(1<<uint(retryCount-1))
	if delay > bc.config.MaxRetryDelay {
		delay = bc.config.MaxRetryDelay
	}
	return delay
}

// acknowledgeMessages acknowledges all messages in the batch
func (bc *BufferedConsumer) acknowledgeMessages(messages []*message.Message) {
	for _, msg := range messages {
		msg.Ack()
	}
}

// nacknowledgeMessages nacknowledges all messages in the batch
func (bc *BufferedConsumer) nacknowledgeMessages(messages []*message.Message) {
	for _, msg := range messages {
		msg.Nack()
	}
}
