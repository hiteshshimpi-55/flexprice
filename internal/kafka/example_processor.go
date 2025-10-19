package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/flexprice/flexprice/internal/domain/events"
	"github.com/flexprice/flexprice/internal/logger"
	"github.com/flexprice/flexprice/internal/types"
)

// EventBatchProcessor processes batches of events
type EventBatchProcessor struct {
	logger *logger.Logger
	// Add your dependencies here (e.g., repositories, services)
	// eventRepo    repository.EventRepository
	// subscriptionRepo repository.SubscriptionRepository
	// priceRepo    repository.PriceRepository
}

// NewEventBatchProcessor creates a new event batch processor
func NewEventBatchProcessor(logger *logger.Logger) *EventBatchProcessor {
	return &EventBatchProcessor{
		logger: logger,
	}
}

// ProcessBatch processes a batch of event messages
func (p *EventBatchProcessor) ProcessBatch(ctx context.Context, messages []*message.Message) error {
	if len(messages) == 0 {
		return nil
	}

	p.logger.Infow("processing event batch",
		"batch_size", len(messages),
	)

	// Extract events from messages
	eventList := make([]*events.Event, 0, len(messages))
	tenantIDs := make(map[string]bool)
	environmentIDs := make(map[string]bool)

	for _, msg := range messages {
		var event events.Event
		if err := json.Unmarshal(msg.Payload, &event); err != nil {
			p.logger.Errorw("failed to unmarshal event in batch",
				"error", err,
				"message_uuid", msg.UUID,
			)
			return fmt.Errorf("failed to unmarshal event: %w", err)
		}

		eventList = append(eventList, &event)
		tenantIDs[event.TenantID] = true
		environmentIDs[event.EnvironmentID] = true
	}

	// Bulk fetch dependent data (subscriptions, prices, etc.)
	// This is where you would implement bulk fetching to reduce DB load
	if err := p.bulkFetchDependentData(ctx, tenantIDs, environmentIDs); err != nil {
		p.logger.Errorw("failed to fetch dependent data",
			"error", err,
		)
		return fmt.Errorf("failed to fetch dependent data: %w", err)
	}

	// Process events in bulk
	if err := p.processEventsBulk(ctx, eventList); err != nil {
		p.logger.Errorw("failed to process events in bulk",
			"error", err,
		)
		return fmt.Errorf("failed to process events: %w", err)
	}

	p.logger.Infow("successfully processed event batch",
		"batch_size", len(eventList),
	)

	return nil
}

// bulkFetchDependentData fetches all dependent data in bulk to reduce DB queries
func (p *EventBatchProcessor) bulkFetchDependentData(ctx context.Context, tenantIDs, environmentIDs map[string]bool) error {
	// Convert maps to slices
	tenantIDList := make([]string, 0, len(tenantIDs))
	for tenantID := range tenantIDs {
		tenantIDList = append(tenantIDList, tenantID)
	}

	environmentIDList := make([]string, 0, len(environmentIDs))
	for envID := range environmentIDs {
		environmentIDList = append(environmentIDList, envID)
	}

	p.logger.Debugw("bulk fetching dependent data",
		"tenant_count", len(tenantIDList),
		"environment_count", len(environmentIDList),
	)

	// TODO: Implement bulk fetching of:
	// - Subscriptions for all tenants
	// - Prices for all environments
	// - Customer data
	// - Any other dependent data needed for processing

	// Example implementation:
	// subscriptions, err := p.subscriptionRepo.GetByTenantIDs(ctx, tenantIDList)
	// if err != nil {
	//     return err
	// }
	//
	// prices, err := p.priceRepo.GetByEnvironmentIDs(ctx, environmentIDList)
	// if err != nil {
	//     return err
	// }

	// Cache the fetched data for use in processEventsBulk
	// p.cacheSubscriptions(subscriptions)
	// p.cachePrices(prices)

	return nil
}

// processEventsBulk processes all events in the batch
func (p *EventBatchProcessor) processEventsBulk(ctx context.Context, eventList []*events.Event) error {
	// Group events by tenant for efficient processing
	eventsByTenant := make(map[string][]*events.Event)
	for _, event := range eventList {
		eventsByTenant[event.TenantID] = append(eventsByTenant[event.TenantID], event)
	}

	// Process each tenant's events
	for tenantID, tenantEvents := range eventsByTenant {
		if err := p.processTenantEvents(ctx, tenantID, tenantEvents); err != nil {
			p.logger.Errorw("failed to process tenant events",
				"error", err,
				"tenant_id", tenantID,
				"event_count", len(tenantEvents),
			)
			return fmt.Errorf("failed to process events for tenant %s: %w", tenantID, err)
		}
	}

	return nil
}

// processTenantEvents processes all events for a specific tenant
func (p *EventBatchProcessor) processTenantEvents(ctx context.Context, tenantID string, tenantEvents []*events.Event) error {
	// Add tenant context
	ctx = context.WithValue(ctx, types.CtxTenantID, tenantID)

	p.logger.Debugw("processing tenant events",
		"tenant_id", tenantID,
		"event_count", len(tenantEvents),
	)

	// TODO: Implement tenant-specific event processing logic
	// This could include:
	// - Validating events against tenant's subscription
	// - Applying tenant-specific pricing rules
	// - Updating tenant metrics
	// - Triggering tenant-specific workflows

	// Example: Insert events into database in bulk
	// if err := p.eventRepo.BulkInsertEvents(ctx, events); err != nil {
	//     return err
	// }

	// Example: Update tenant metrics
	// if err := p.updateTenantMetrics(ctx, tenantID, events); err != nil {
	//     return err
	// }

	return nil
}

// ExampleBatchProcessor is a simple example processor for demonstration
type ExampleBatchProcessor struct {
	logger *logger.Logger
}

// NewExampleBatchProcessor creates a new example batch processor
func NewExampleBatchProcessor(logger *logger.Logger) *ExampleBatchProcessor {
	return &ExampleBatchProcessor{
		logger: logger,
	}
}

// ProcessBatch processes a batch of messages (simple example)
func (p *ExampleBatchProcessor) ProcessBatch(ctx context.Context, messages []*message.Message) error {
	p.logger.Infow("processing example batch",
		"batch_size", len(messages),
	)

	// Simulate some processing time
	time.Sleep(100 * time.Millisecond)

	// Simulate occasional errors for testing retry logic
	if len(messages) > 0 && len(messages)%10 == 0 {
		p.logger.Warnw("simulating batch processing error for testing",
			"batch_size", len(messages),
		)
		return fmt.Errorf("simulated processing error")
	}

	p.logger.Debugw("example batch processed successfully",
		"batch_size", len(messages),
	)

	return nil
}
