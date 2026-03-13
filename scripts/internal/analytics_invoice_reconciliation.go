package internal

import (
	"context"
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/flexprice/flexprice/internal/api/dto"
	"github.com/flexprice/flexprice/internal/cache"
	"github.com/flexprice/flexprice/internal/clickhouse"
	"github.com/flexprice/flexprice/internal/config"
	"github.com/flexprice/flexprice/internal/domain/customer"
	"github.com/flexprice/flexprice/internal/domain/invoice"
	domainSub "github.com/flexprice/flexprice/internal/domain/subscription"
	"github.com/flexprice/flexprice/internal/logger"
	"github.com/flexprice/flexprice/internal/postgres"
	chRepo "github.com/flexprice/flexprice/internal/repository/clickhouse"
	entRepo "github.com/flexprice/flexprice/internal/repository/ent"
	"github.com/flexprice/flexprice/internal/sentry"
	"github.com/flexprice/flexprice/internal/service"
	"github.com/flexprice/flexprice/internal/types"
	"github.com/shopspring/decimal"
)

// reconciliationRow represents a single row in the output CSV
type reconciliationRow struct {
	CustomerID       string
	SubscriptionID   string
	InvoiceID        string
	PeriodStart      string
	PeriodEnd        string
	AnalyticsAmount  string
	InvoiceSubtotal  string
	Diff             string
	InvoiceGenerated string
}

// reconciliationScript holds the dependencies for the reconciliation script
type reconciliationScript struct {
	log                         *logger.Logger
	subRepo                     domainSub.Repository
	invoiceRepo                 invoice.Repository
	customerRepo                customer.Repository
	featureUsageTrackingService service.FeatureUsageTrackingService
}

// RunAnalyticsInvoiceReconciliation is the entry point for the reconciliation script.
// It fetches all subscriptions for a tenant+environment, computes the 2 periods
// before the current period, fetches analytics costs (via GetDetailedUsageAnalytics)
// and invoices for those periods, compares them, and writes the results to a CSV file.
func RunAnalyticsInvoiceReconciliation() error {
	tenantID := os.Getenv("TENANT_ID")
	environmentID := os.Getenv("ENVIRONMENT_ID")
	workerCountStr := os.Getenv("WORKER_COUNT")

	if tenantID == "" || environmentID == "" {
		return fmt.Errorf("TENANT_ID and ENVIRONMENT_ID are required")
	}

	workerCount := 5
	if workerCountStr != "" {
		if n, err := strconv.Atoi(workerCountStr); err == nil && n > 0 {
			workerCount = n
		}
	}

	log.Printf("Starting analytics-invoice reconciliation for tenant=%s env=%s workers=%d\n",
		tenantID, environmentID, workerCount)

	script, err := newReconciliationScript()
	if err != nil {
		return fmt.Errorf("failed to initialize script: %w", err)
	}

	ctx := context.Background()
	ctx = context.WithValue(ctx, types.CtxTenantID, tenantID)
	ctx = context.WithValue(ctx, types.CtxEnvironmentID, environmentID)

	// Fetch all active + cancelled subscriptions (no pagination limit)
	subFilter := types.NewNoLimitSubscriptionFilter()
	subFilter.SubscriptionStatus = []types.SubscriptionStatus{
		types.SubscriptionStatusActive,
	}

	subs, err := script.subRepo.ListAll(ctx, subFilter)
	if err != nil {
		return fmt.Errorf("failed to list subscriptions: %w", err)
	}

	log.Printf("Found %d subscriptions to process\n", len(subs))

	// Batch-fetch all customers to build CustomerID -> ExternalID map
	customerIDs := make([]string, 0, len(subs))
	seen := make(map[string]bool)
	for _, sub := range subs {
		if !seen[sub.CustomerID] {
			customerIDs = append(customerIDs, sub.CustomerID)
			seen[sub.CustomerID] = true
		}
	}

	customerMap, err := script.buildCustomerMap(ctx, customerIDs)
	if err != nil {
		return fmt.Errorf("failed to fetch customers: %w", err)
	}
	log.Printf("Fetched %d unique customers\n", len(customerMap))

	// Process subscriptions concurrently
	var (
		mu        sync.Mutex
		allRows   []reconciliationRow
		wg        sync.WaitGroup
		semaphore = make(chan struct{}, workerCount)
	)

	for i, sub := range subs {
		if i%50 == 0 {
			log.Printf("Queued %d/%d subscriptions for processing\n", i, len(subs))
		}

		wg.Add(1)
		semaphore <- struct{}{} // acquire

		go func(sub *domainSub.Subscription) {
			defer wg.Done()
			defer func() { <-semaphore }() // release

			rows := script.processSubscription(ctx, sub, customerMap)
			if len(rows) > 0 {
				mu.Lock()
				allRows = append(allRows, rows...)
				mu.Unlock()
			}
		}(sub)
	}

	wg.Wait()

	log.Printf("Processed all %d subscriptions, total rows: %d\n", len(subs), len(allRows))

	// Write CSV
	outputFile := fmt.Sprintf("reconciliation_%s_%s.csv", tenantID, time.Now().Format("20060102_150405"))
	if err := writeReconciliationCSV(allRows, outputFile); err != nil {
		return fmt.Errorf("failed to write CSV: %w", err)
	}

	log.Printf("Reconciliation report generated: %s\n", outputFile)
	return nil
}

// buildCustomerMap fetches customers by IDs and returns a map of CustomerID -> ExternalID.
func (s *reconciliationScript) buildCustomerMap(ctx context.Context, customerIDs []string) (map[string]string, error) {
	result := make(map[string]string, len(customerIDs))

	// Fetch customers in batches of 100
	batchSize := 100
	for i := 0; i < len(customerIDs); i += batchSize {
		end := i + batchSize
		if end > len(customerIDs) {
			end = len(customerIDs)
		}
		batch := customerIDs[i:end]

		filter := types.NewNoLimitCustomerFilter()
		filter.CustomerIDs = batch

		customers, err := s.customerRepo.List(ctx, filter)
		if err != nil {
			return nil, fmt.Errorf("failed to list customers (batch %d-%d): %w", i, end, err)
		}

		for _, c := range customers {
			result[c.ID] = c.ExternalID
		}
	}

	return result, nil
}

// processSubscription computes the last 2 periods (before the current period) for a
// subscription and reconciles analytics vs invoice for each.
func (s *reconciliationScript) processSubscription(ctx context.Context, sub *domainSub.Subscription, customerMap map[string]string) []reconciliationRow {
	periods := s.computePreviousPeriods(sub, 2)
	if len(periods) == 0 {
		return nil
	}

	externalCustomerID, ok := customerMap[sub.CustomerID]
	if !ok || externalCustomerID == "" {
		s.log.Warnw("customer external ID not found, skipping subscription",
			"subscription_id", sub.ID,
			"customer_id", sub.CustomerID)
		return nil
	}

	var rows []reconciliationRow
	for _, period := range periods {
		row := s.reconcilePeriod(ctx, sub, period, externalCustomerID)
		if row != nil {
			rows = append(rows, *row)
		}
	}

	return rows
}

// period represents a billing period [Start, End)
type period struct {
	Start time.Time
	End   time.Time
}

// computePreviousPeriods computes N periods before the current period by walking
// backwards from CurrentPeriodStart using BillingAnchor and BillingPeriod.
func (s *reconciliationScript) computePreviousPeriods(sub *domainSub.Subscription, count int) []period {
	periods := make([]period, 0, count)

	// The end of the previous period is the start of the current period
	prevEnd := sub.CurrentPeriodStart

	for i := 0; i < count; i++ {
		// Calculate previous period start by going backwards one billing interval
		prevStart, err := types.PreviousBillingDate(prevEnd, 1, sub.BillingPeriod)
		if err != nil {
			s.log.Warnw("failed to compute previous billing date",
				"subscription_id", sub.ID,
				"prev_end", prevEnd,
				"billing_period", sub.BillingPeriod,
				"error", err)
			break
		}

		// Don't go before subscription start date
		if prevStart.Before(sub.StartDate) {
			if prevEnd.After(sub.StartDate) {
				prevStart = sub.StartDate
			} else {
				break
			}
		}

		periods = append(periods, period{Start: prevStart, End: prevEnd})
		prevEnd = prevStart
	}

	return periods
}

// reconcilePeriod fetches analytics cost and invoice for a single period and returns a CSV row.
func (s *reconciliationScript) reconcilePeriod(ctx context.Context, sub *domainSub.Subscription, p period, externalCustomerID string) *reconciliationRow {
	row := reconciliationRow{
		CustomerID:     sub.CustomerID,
		SubscriptionID: sub.ID,
		PeriodStart:    p.Start.UTC().Format(time.RFC3339),
		PeriodEnd:      p.End.UTC().Format(time.RFC3339),
	}

	// 1. Get analytics cost via featureUsageTrackingService.GetDetailedUsageAnalytics
	analyticsAmount := decimal.Zero
	analyticsResp, err := s.featureUsageTrackingService.GetDetailedUsageAnalytics(ctx, &dto.GetUsageAnalyticsRequest{
		ExternalCustomerID: externalCustomerID,
		StartTime:          p.Start,
		EndTime:            p.End,
	})
	if err != nil {
		s.log.Warnw("failed to get detailed usage analytics",
			"subscription_id", sub.ID,
			"customer_id", sub.CustomerID,
			"external_customer_id", externalCustomerID,
			"period_start", p.Start,
			"period_end", p.End,
			"error", err)
	} else if analyticsResp != nil {
		analyticsAmount = analyticsResp.TotalCost
	}
	row.AnalyticsAmount = analyticsAmount.StringFixed(2)

	// 2. Find matching invoice for this subscription + period
	inv := s.findInvoiceForPeriod(ctx, sub.ID, p)
	if inv != nil {
		row.InvoiceID = inv.ID
		row.InvoiceSubtotal = inv.Subtotal.StringFixed(2)
		row.InvoiceGenerated = "true"

		diff := analyticsAmount.Sub(inv.Subtotal)
		row.Diff = diff.StringFixed(2)
	} else if analyticsAmount.GreaterThan(decimal.Zero) {
		row.InvoiceID = ""
		row.InvoiceSubtotal = ""
		row.InvoiceGenerated = "false"
		row.Diff = analyticsAmount.StringFixed(2)
	} else {
		return nil
	}

	return &row
}

// findInvoiceForPeriod queries for a subscription invoice matching the given period.
func (s *reconciliationScript) findInvoiceForPeriod(ctx context.Context, subscriptionID string, p period) *invoice.Invoice {
	filter := types.NewNoLimitInvoiceFilter()
	filter.SubscriptionID = subscriptionID
	filter.InvoiceType = types.InvoiceTypeSubscription
	filter.InvoiceStatus = []types.InvoiceStatus{
		types.InvoiceStatusDraft,
		types.InvoiceStatusFinalized,
	}

	// Match period start within a small tolerance window (±1 minute)
	tolerance := 1 * time.Minute
	periodStartGTE := p.Start.Add(-tolerance)
	periodStartLTE := p.Start.Add(tolerance)
	filter.PeriodStartGTE = &periodStartGTE
	filter.PeriodStartLTE = &periodStartLTE

	filter.SkipLineItems = true

	invoices, err := s.invoiceRepo.List(ctx, filter)
	if err != nil {
		s.log.Warnw("failed to list invoices for period",
			"subscription_id", subscriptionID,
			"period_start", p.Start,
			"period_end", p.End,
			"error", err)
		return nil
	}

	if len(invoices) == 0 {
		return nil
	}

	return invoices[0]
}

// writeReconciliationCSV writes the reconciliation rows to a CSV file.
func writeReconciliationCSV(rows []reconciliationRow, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create CSV file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	header := []string{
		"customer_id",
		"subscription_id",
		"invoice_id",
		"period_start",
		"period_end",
		"analytics_amount",
		"invoice_subtotal",
		"diff",
		"invoice_generated",
	}
	if err := writer.Write(header); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	for _, row := range rows {
		record := []string{
			row.CustomerID,
			row.SubscriptionID,
			row.InvoiceID,
			row.PeriodStart,
			row.PeriodEnd,
			row.AnalyticsAmount,
			row.InvoiceSubtotal,
			row.Diff,
			row.InvoiceGenerated,
		}
		if err := writer.Write(record); err != nil {
			return fmt.Errorf("failed to write CSV record: %w", err)
		}
	}

	return nil
}

// newReconciliationScript initialises all dependencies needed by the reconciliation script.
func newReconciliationScript() (*reconciliationScript, error) {
	cfg, err := config.NewConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	l, err := logger.NewLogger(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create logger: %w", err)
	}

	sentryService := sentry.NewSentryService(cfg, l)

	chStore, err := clickhouse.NewClickHouseStore(cfg, sentryService)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to clickhouse: %w", err)
	}

	entClient, err := postgres.NewEntClients(cfg, l)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to postgres: %w", err)
	}
	client := postgres.NewClient(entClient, l, sentryService)
	cacheClient := cache.NewInMemoryCache()

	// Create repositories
	customerRepo := entRepo.NewCustomerRepository(client, l, cacheClient)
	subscriptionRepo := entRepo.NewSubscriptionRepository(client, l, cacheClient)
	subscriptionLineItemRepo := entRepo.NewSubscriptionLineItemRepository(client, l, cacheClient)
	subscriptionPhaseRepo := entRepo.NewSubscriptionPhaseRepository(client, l, cacheClient)
	planRepo := entRepo.NewPlanRepository(client, l, cacheClient)
	priceRepo := entRepo.NewPriceRepository(client, l, cacheClient)
	meterRepo := entRepo.NewMeterRepository(client, l, cacheClient)
	featureRepo := entRepo.NewFeatureRepository(client, l, cacheClient)
	entitlementRepo := entRepo.NewEntitlementRepository(client, l, cacheClient)
	addonRepo := entRepo.NewAddonRepository(client, l, cacheClient)
	addonAssociationRepo := entRepo.NewAddonAssociationRepository(client, l, cacheClient)
	invoiceRepo := entRepo.NewInvoiceRepository(client, l, cacheClient)
	settingsRepo := entRepo.NewSettingsRepository(client, l, cacheClient)
	eventRepo := chRepo.NewEventRepository(chStore, l)
	processedEventRepo := chRepo.NewProcessedEventRepository(chStore, l)
	featureUsageRepo := chRepo.NewFeatureUsageRepository(chStore, l)

	serviceParams := service.ServiceParams{
		Logger:                   l,
		Config:                   cfg,
		DB:                       client,
		CustomerRepo:             customerRepo,
		SubRepo:                  subscriptionRepo,
		SubscriptionLineItemRepo: subscriptionLineItemRepo,
		SubscriptionPhaseRepo:    subscriptionPhaseRepo,
		PlanRepo:                 planRepo,
		PriceRepo:                priceRepo,
		MeterRepo:                meterRepo,
		FeatureRepo:              featureRepo,
		EntitlementRepo:          entitlementRepo,
		AddonRepo:                addonRepo,
		AddonAssociationRepo:     addonAssociationRepo,
		InvoiceRepo:              invoiceRepo,
		SettingsRepo:             settingsRepo,
		EventRepo:                eventRepo,
		ProcessedEventRepo:       processedEventRepo,
		FeatureUsageRepo:         featureUsageRepo,
	}

	featureUsageTrackingService := service.NewFeatureUsageTrackingService(serviceParams, eventRepo, featureUsageRepo)

	return &reconciliationScript{
		log:                         l,
		subRepo:                     subscriptionRepo,
		invoiceRepo:                 invoiceRepo,
		customerRepo:                customerRepo,
		featureUsageTrackingService: featureUsageTrackingService,
	}, nil
}
