# Buffered Kafka Consumer

A high-performance, production-ready Kafka consumer implementation that processes messages in batches to optimize database operations and reduce system load.

## Features

- **Batch Processing**: Groups messages into configurable batches for efficient processing
- **Automatic Flushing**: Processes batches when size threshold is reached or time interval expires
- **Reliable Offset Management**: Uses Kafka consumer groups for automatic offset management
- **Retry Logic**: Built-in exponential backoff retry mechanism for failed batches
- **Graceful Shutdown**: Flushes remaining messages before stopping
- **Metrics Collection**: Comprehensive metrics for monitoring and observability
- **Dead Letter Queue Support**: Easy integration with DLQ for failed messages
- **Thread-Safe**: Safe for concurrent use

## Quick Start

```go
package main

import (
    "context"
    "log"
    "os"
    "os/signal"
    "syscall"

    "github.com/flexprice/flexprice/internal/kafka"
    "github.com/flexprice/flexprice/internal/config"
    "github.com/flexprice/flexprice/internal/logger"
)

func main() {
    // Load configuration
    cfg, err := config.NewConfig()
    if err != nil {
        log.Fatal(err)
    }

    // Create logger
    logger := logger.NewLogger(cfg.Logging.Level)

    // Create batch processor
    processor := kafka.NewEventBatchProcessor(logger)

    // Create metrics collector
    metrics := kafka.NewPrometheusMetricsCollector(logger)

    // Configure consumer
    consumerConfig := &kafka.BufferedConsumerConfig{
        BatchSize:      500,              // Process up to 500 messages per batch
        FlushInterval:  5 * time.Second,  // Flush every 5 seconds
        MaxRetries:     3,                // Retry failed batches up to 3 times
        RetryDelay:     1 * time.Second,  // Initial retry delay
        MaxRetryDelay:  30 * time.Second, // Maximum retry delay
        EnableMetrics:  true,             // Enable metrics collection
        BufferCapacity: 1000,             // Buffer up to 1000 messages
    }

    // Create buffered consumer
    consumer, err := kafka.NewBufferedConsumer(cfg, processor, metrics, consumerConfig, logger)
    if err != nil {
        log.Fatal(err)
    }

    // Start consuming
    if err := consumer.Start("events"); err != nil {
        log.Fatal(err)
    }

    // Handle graceful shutdown
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    <-sigChan

    // Stop consumer
    consumer.Stop()
}
```

## Configuration

### BufferedConsumerConfig

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `BatchSize` | `int` | `500` | Maximum number of messages to buffer before processing |
| `FlushInterval` | `time.Duration` | `5s` | Maximum time to wait before flushing the buffer |
| `MaxRetries` | `int` | `3` | Maximum number of retries for failed batches |
| `RetryDelay` | `time.Duration` | `1s` | Initial delay between retries |
| `MaxRetryDelay` | `time.Duration` | `30s` | Maximum delay between retries |
| `EnableMetrics` | `bool` | `true` | Enable metrics collection |
| `BufferCapacity` | `int` | `1000` | Capacity of the internal buffer channel |

## Batch Processing

### Implementing BatchProcessor

```go
type MyBatchProcessor struct {
    logger *logger.Logger
    // Add your dependencies
}

func (p *MyBatchProcessor) ProcessBatch(ctx context.Context, messages []*message.Message) error {
    // Extract data from messages
    events := make([]*Event, 0, len(messages))
    for _, msg := range messages {
        var event Event
        if err := json.Unmarshal(msg.Payload, &event); err != nil {
            return err
        }
        events = append(events, &event)
    }

    // Bulk fetch dependent data
    if err := p.bulkFetchDependentData(ctx, events); err != nil {
        return err
    }

    // Process events in bulk
    if err := p.processEventsBulk(ctx, events); err != nil {
        return err
    }

    return nil
}
```

### Bulk Data Fetching

The key to efficient batch processing is bulk fetching dependent data:

```go
func (p *MyBatchProcessor) bulkFetchDependentData(ctx context.Context, events []*Event) error {
    // Collect unique IDs
    tenantIDs := make(map[string]bool)
    for _, event := range events {
        tenantIDs[event.TenantID] = true
    }

    // Bulk fetch subscriptions for all tenants
    tenantIDList := make([]string, 0, len(tenantIDs))
    for tenantID := range tenantIDs {
        tenantIDList = append(tenantIDList, tenantID)
    }

    subscriptions, err := p.subscriptionRepo.GetByTenantIDs(ctx, tenantIDList)
    if err != nil {
        return err
    }

    // Cache for use in processEventsBulk
    p.cacheSubscriptions(subscriptions)
    return nil
}
```

## Metrics

### PrometheusMetricsCollector

The `PrometheusMetricsCollector` provides comprehensive metrics:

- `batch_size_total`: Total number of messages processed
- `batch_duration_seconds`: Duration of batch processing
- `batch_errors_total`: Total number of batch processing errors
- `retries_total`: Total number of retry attempts
- `buffer_size_current`: Current buffer size

### Custom Metrics

```go
type CustomMetricsCollector struct {
    // Your metrics implementation
}

func (m *CustomMetricsCollector) RecordBatchSize(size int) {
    // Record batch size metric
}

func (m *CustomMetricsCollector) RecordBatchDuration(duration time.Duration) {
    // Record batch duration metric
}

// ... implement other methods
```

## Error Handling

### Retry Logic

The consumer automatically retries failed batches with exponential backoff:

```go
consumerConfig := &BufferedConsumerConfig{
    MaxRetries:     5,                // Retry up to 5 times
    RetryDelay:     1 * time.Second,  // Start with 1 second delay
    MaxRetryDelay:  30 * time.Second, // Cap at 30 seconds
}
```

### Dead Letter Queue

For messages that fail after all retries, implement DLQ handling:

```go
type DLQBatchProcessor struct {
    dlqPublisher Publisher
}

func (p *DLQBatchProcessor) ProcessBatch(ctx context.Context, messages []*message.Message) error {
    if err := p.processBatchInternal(ctx, messages); err != nil {
        // Send to DLQ instead of returning error
        if err := p.sendToDLQ(ctx, messages, err); err != nil {
            return err
        }
        return nil // Don't retry if sent to DLQ
    }
    return nil
}
```

## Graceful Shutdown

The consumer supports graceful shutdown:

```go
// Handle shutdown signals
sigChan := make(chan os.Signal, 1)
signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

// Wait for signal
<-sigChan

// Stop consumer (flushes remaining messages)
consumer.Stop()
```

## Performance Considerations

### Batch Size Tuning

- **Small batches (50-100)**: Lower latency, higher overhead
- **Large batches (500-1000)**: Higher throughput, higher memory usage
- **Very large batches (1000+)**: Risk of timeouts and memory issues

### Buffer Capacity

Set buffer capacity to handle traffic spikes:

```go
consumerConfig := &BufferedConsumerConfig{
    BufferCapacity: 2000, // Handle 2x normal batch size
}
```

### Memory Usage

Monitor memory usage, especially with large batches:

```go
// Use smaller batches if memory is constrained
consumerConfig := &BufferedConsumerConfig{
    BatchSize:      100,  // Smaller batches
    BufferCapacity: 500,  // Smaller buffer
}
```

## Testing

### Unit Tests

```go
func TestBufferedConsumer(t *testing.T) {
    processor := &mockBatchProcessor{}
    metrics := &mockMetricsCollector{}
    
    consumer, err := NewBufferedConsumer(cfg, processor, metrics, nil, logger)
    require.NoError(t, err)
    
    // Test batch processing
    messages := []*message.Message{
        message.NewMessage("1", []byte("test1")),
        message.NewMessage("2", []byte("test2")),
    }
    
    err = processor.ProcessBatch(context.Background(), messages)
    assert.NoError(t, err)
}
```

### Integration Tests

```go
func TestBufferedConsumerIntegration(t *testing.T) {
    // Set up test Kafka cluster
    // Create consumer with test configuration
    // Send test messages
    // Verify processing
}
```

## Monitoring

### Health Checks

```go
func (c *BufferedConsumer) HealthCheck() error {
    // Check if consumer is running
    // Check buffer size
    // Check error rates
    return nil
}
```

### Metrics Dashboard

Create dashboards to monitor:

- Batch processing rate
- Average batch size
- Error rates
- Buffer utilization
- Processing latency

## Best Practices

1. **Batch Size**: Start with 500 messages, adjust based on your workload
2. **Flush Interval**: Set to 5-10 seconds for most use cases
3. **Retry Logic**: Use exponential backoff with reasonable limits
4. **Metrics**: Always enable metrics in production
5. **Monitoring**: Set up alerts for error rates and buffer utilization
6. **Testing**: Test with realistic data volumes and failure scenarios
7. **Memory**: Monitor memory usage, especially with large batches
8. **Graceful Shutdown**: Always implement proper shutdown handling

## Troubleshooting

### Common Issues

1. **High Memory Usage**: Reduce batch size or buffer capacity
2. **Slow Processing**: Increase batch size or optimize batch processing logic
3. **Message Loss**: Check retry configuration and DLQ setup
4. **High Error Rates**: Review batch processing logic and dependencies

### Debug Logging

Enable debug logging to troubleshoot issues:

```go
cfg := &config.Configuration{
    Logging: config.LoggingConfig{
        Level: "debug",
    },
}
```

## Examples

See the `example_usage.go` file for complete examples including:

- Basic usage
- Custom processors
- Dead letter queue handling
- Metrics collection
- Graceful shutdown
