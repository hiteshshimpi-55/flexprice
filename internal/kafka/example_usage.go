package kafka

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/flexprice/flexprice/internal/config"
	"github.com/flexprice/flexprice/internal/logger"
)

// ExampleUsage demonstrates how to use the BufferedConsumer
func ExampleUsage() {
	// Load configuration
	cfg, err := config.NewConfig()
	if err != nil {
		panic(err)
	}

	// Create logger
	logger, err := logger.NewLogger(cfg)
	if err != nil {
		panic(err)
	}

	// Create metrics collector
	metrics := NewPrometheusMetricsCollector(logger)

	// Create batch processor
	processor := NewEventBatchProcessor(logger)

	// Configure buffered consumer
	consumerConfig := &BufferedConsumerConfig{
		BatchSize:      500,              // Process up to 500 messages per batch
		FlushInterval:  5 * time.Second,  // Flush every 5 seconds
		MaxRetries:     3,                // Retry failed batches up to 3 times
		RetryDelay:     1 * time.Second,  // Initial retry delay
		MaxRetryDelay:  30 * time.Second, // Maximum retry delay
		EnableMetrics:  true,             // Enable metrics collection
		BufferCapacity: 1000,             // Buffer up to 1000 messages
	}

	// Create buffered consumer
	consumer, err := NewBufferedConsumer(cfg, processor, metrics, consumerConfig, logger)
	if err != nil {
		logger.Fatalw("failed to create buffered consumer", "error", err)
	}

	// Start consuming from topic
	topic := "events"
	if err := consumer.Start(topic); err != nil {
		logger.Fatalw("failed to start consumer", "error", err)
	}

	// Set up graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start metrics reporting goroutine
	go reportMetrics(metrics, logger)

	// Wait for shutdown signal
	<-sigChan
	logger.Info("received shutdown signal")

	// Stop consumer gracefully
	if err := consumer.Stop(); err != nil {
		logger.Errorw("error stopping consumer", "error", err)
	}

	logger.Info("consumer stopped")
}

// reportMetrics periodically reports metrics
func reportMetrics(metrics *PrometheusMetricsCollector, logger *logger.Logger) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		snapshot := metrics.GetMetrics()
		logger.Infow("metrics snapshot",
			"total_batch_size", snapshot.TotalBatchSize,
			"total_batch_errors", snapshot.TotalBatchErrors,
			"total_retries", snapshot.TotalRetries,
			"current_buffer_size", snapshot.CurrentBufferSize,
			"batch_count", snapshot.BatchCount,
			"average_batch_size", snapshot.AverageBatchSize,
			"average_batch_duration", snapshot.AverageBatchDuration,
		)
	}
}

// ExampleWithCustomProcessor demonstrates using a custom processor
func ExampleWithCustomProcessor() {
	cfg, err := config.NewConfig()
	if err != nil {
		panic(err)
	}

	logger, err := logger.NewLogger(cfg)
	if err != nil {
		panic(err)
	}

	// Create custom processor
	customProcessor := &CustomBatchProcessor{
		logger: logger,
		// Add your dependencies
	}

	// Create metrics collector (or use NoOpMetricsCollector to disable metrics)
	metrics := NewNoOpMetricsCollector()

	// Use default configuration
	consumer, err := NewBufferedConsumer(cfg, customProcessor, metrics, nil, logger)
	if err != nil {
		logger.Fatalw("failed to create buffered consumer", "error", err)
	}

	// Start consuming
	if err := consumer.Start("custom-topic"); err != nil {
		logger.Fatalw("failed to start consumer", "error", err)
	}

	// Handle shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Wait for context cancellation
	<-ctx.Done()

	// Stop consumer
	consumer.Stop()
}

// CustomBatchProcessor is an example of a custom batch processor
type CustomBatchProcessor struct {
	logger *logger.Logger
	// Add your dependencies here
}

// ProcessBatch implements the BatchProcessor interface
func (p *CustomBatchProcessor) ProcessBatch(ctx context.Context, messages []*message.Message) error {
	p.logger.Infow("processing custom batch",
		"batch_size", len(messages),
	)

	// Your custom processing logic here
	for i, msg := range messages {
		p.logger.Debugw("processing message",
			"index", i,
			"message_uuid", msg.UUID,
			"payload_size", len(msg.Payload),
		)

		// Process the message
		if err := p.processMessage(ctx, msg); err != nil {
			p.logger.Errorw("failed to process message",
				"error", err,
				"message_uuid", msg.UUID,
			)
			return err
		}
	}

	return nil
}

// processMessage processes a single message
func (p *CustomBatchProcessor) processMessage(ctx context.Context, msg *message.Message) error {
	// Your message processing logic here
	// For example:
	// 1. Unmarshal the payload
	// 2. Validate the data
	// 3. Transform the data
	// 4. Store in database
	// 5. Send notifications
	// 6. Update metrics

	return nil
}

// ExampleWithDeadLetterQueue demonstrates handling failed batches
func ExampleWithDeadLetterQueue() {
	cfg, err := config.NewConfig()
	if err != nil {
		panic(err)
	}

	logger, err := logger.NewLogger(cfg)
	if err != nil {
		panic(err)
	}

	// Create processor with DLQ support
	processor := &DLQBatchProcessor{
		logger: logger,
		// Add DLQ publisher
	}

	metrics := NewPrometheusMetricsCollector(logger)

	// Configure consumer with more aggressive retry settings
	consumerConfig := &BufferedConsumerConfig{
		BatchSize:      100,             // Smaller batches for faster processing
		FlushInterval:  2 * time.Second, // More frequent flushing
		MaxRetries:     5,               // More retries
		RetryDelay:     500 * time.Millisecond,
		MaxRetryDelay:  10 * time.Second,
		EnableMetrics:  true,
		BufferCapacity: 500,
	}

	consumer, err := NewBufferedConsumer(cfg, processor, metrics, consumerConfig, logger)
	if err != nil {
		logger.Fatalw("failed to create buffered consumer", "error", err)
	}

	// Start consuming
	if err := consumer.Start("events-with-dlq"); err != nil {
		logger.Fatalw("failed to start consumer", "error", err)
	}

	// Handle shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	consumer.Stop()
}

// DLQBatchProcessor demonstrates handling failed batches with DLQ
type DLQBatchProcessor struct {
	logger *logger.Logger
	// dlqPublisher Publisher // Add DLQ publisher
}

// ProcessBatch implements the BatchProcessor interface with DLQ support
func (p *DLQBatchProcessor) ProcessBatch(ctx context.Context, messages []*message.Message) error {
	p.logger.Infow("processing batch with DLQ support",
		"batch_size", len(messages),
	)

	// Try to process the batch
	if err := p.processBatchInternal(ctx, messages); err != nil {
		p.logger.Errorw("batch processing failed, sending to DLQ",
			"error", err,
			"batch_size", len(messages),
		)

		// Send failed batch to DLQ
		if err := p.sendToDLQ(ctx, messages, err); err != nil {
			p.logger.Errorw("failed to send batch to DLQ",
				"error", err,
			)
			return err
		}

		// Don't return error to prevent retries since we've sent to DLQ
		return nil
	}

	return nil
}

// processBatchInternal contains the actual batch processing logic
func (p *DLQBatchProcessor) processBatchInternal(ctx context.Context, messages []*message.Message) error {
	// Your batch processing logic here
	// Return error if processing fails
	return nil
}

// sendToDLQ sends failed batch to dead letter queue
func (p *DLQBatchProcessor) sendToDLQ(ctx context.Context, messages []*message.Message, originalError error) error {
	// TODO: Implement DLQ publishing logic
	// This could involve:
	// 1. Creating a DLQ message with the original batch and error
	// 2. Publishing to a DLQ topic
	// 3. Logging the DLQ event for monitoring

	p.logger.Infow("batch sent to DLQ",
		"batch_size", len(messages),
		"original_error", originalError,
	)

	return nil
}
