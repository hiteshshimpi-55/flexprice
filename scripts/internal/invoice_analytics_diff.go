package internal

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/flexprice/flexprice/internal/api/dto"
	"github.com/flexprice/flexprice/internal/cache"
	"github.com/flexprice/flexprice/internal/clickhouse"
	"github.com/flexprice/flexprice/internal/config"
	"github.com/flexprice/flexprice/internal/domain/customer"
	"github.com/flexprice/flexprice/internal/domain/invoice"
	"github.com/flexprice/flexprice/internal/logger"
	"github.com/flexprice/flexprice/internal/postgres"
	chRepo "github.com/flexprice/flexprice/internal/repository/clickhouse"
	entRepo "github.com/flexprice/flexprice/internal/repository/ent"
	"github.com/flexprice/flexprice/internal/sentry"
	"github.com/flexprice/flexprice/internal/service"
	"github.com/flexprice/flexprice/internal/types"
	"github.com/shopspring/decimal"
)

// LineItemDiff represents a discrepancy at the line item level
type LineItemDiff struct {
	MeterID         string          `json:"meter_id,omitempty"`
	MeterName       string          `json:"meter_name,omitempty"`
	PriceID         string          `json:"price_id,omitempty"`
	InvoiceAmount   decimal.Decimal `json:"invoice_amount"`
	AnalyticsAmount decimal.Decimal `json:"analytics_amount"`
	Difference      decimal.Decimal `json:"difference"`
	InvoiceQuantity decimal.Decimal `json:"invoice_quantity"`
	AnalyticsUsage  decimal.Decimal `json:"analytics_usage"`
	QuantityDiff    decimal.Decimal `json:"quantity_diff"`
}

// InvoiceAnalyticsDiff represents a discrepancy between invoice and analytics totals
type InvoiceAnalyticsDiff struct {
	CustomerID         string          `json:"customer_id"`
	ExternalCustomerID string          `json:"external_customer_id"`
	InvoiceID          string          `json:"invoice_id"`
	InvoiceNumber      string          `json:"invoice_number"`
	PeriodStart        time.Time       `json:"period_start"`
	PeriodEnd          time.Time       `json:"period_end"`
	InvoiceTotal       decimal.Decimal `json:"invoice_total"`
	AnalyticsTotal     decimal.Decimal `json:"analytics_total"`
	Difference         decimal.Decimal `json:"difference"`
	Currency           string          `json:"currency"`
	LineItemDiffs      []LineItemDiff  `json:"line_item_diffs,omitempty"`
}

type invoiceAnalyticsDiffScript struct {
	log                 *logger.Logger
	customerRepo        customer.Repository
	invoiceRepo         invoice.Repository
	featureUsageService service.FeatureUsageTrackingService
}

// RunInvoiceAnalyticsDiff compares invoice totals with analytics API totals and reports discrepancies
func RunInvoiceAnalyticsDiff() error {
	// Get environment variables for the script
	tenantID := os.Getenv("TENANT_ID")
	environmentID := os.Getenv("ENVIRONMENT_ID")
	externalCustomerID := os.Getenv("EXTERNAL_CUSTOMER_ID")
	outputFormat := os.Getenv("OUTPUT_FORMAT") // "csv" or "json", defaults to "csv"

	if tenantID == "" || environmentID == "" {
		return fmt.Errorf("TENANT_ID and ENVIRONMENT_ID are required")
	}

	if outputFormat == "" {
		outputFormat = "json"
	}

	log.Printf("Starting invoice-analytics diff for tenant: %s, environment: %s\n", tenantID, environmentID)
	if externalCustomerID != "" {
		log.Printf("Filtering for customer: %s\n", externalCustomerID)
	}
	log.Printf("Output format: %s\n", outputFormat)

	// Initialize script
	script, err := newInvoiceAnalyticsDiffScript()
	if err != nil {
		return fmt.Errorf("failed to initialize script: %w", err)
	}

	ctx := context.Background()
	ctx = context.WithValue(ctx, types.CtxTenantID, tenantID)
	ctx = context.WithValue(ctx, types.CtxEnvironmentID, environmentID)

	// Get customers to process
	customers, err := script.getCustomersToProcess(ctx, externalCustomerID)
	if err != nil {
		return fmt.Errorf("failed to get customers: %w", err)
	}

	log.Printf("Found %d customers to process\n", len(customers))

	// Process each customer and collect discrepancies
	var diffs []InvoiceAnalyticsDiff

	for i, cust := range customers {
		if i%10 == 0 {
			log.Printf("Processing customer %d/%d: %s (external: %s)\n", i+1, len(customers), cust.ID, cust.ExternalID)
		}

		customerDiffs, err := script.processCustomer(ctx, cust)
		if err != nil {
			log.Printf("Warning: Failed to process customer %s: %v\n", cust.ID, err)
			continue
		}

		diffs = append(diffs, customerDiffs...)
	}

	// Generate output based on format
	timestamp := time.Now().Format("20060102_150405")
	if outputFormat == "json" {
		outputFile := fmt.Sprintf("invoice_analytics_diff_%s_%s.json", tenantID, timestamp)
		if err := generateInvoiceAnalyticsDiffJSON(diffs, outputFile); err != nil {
			return fmt.Errorf("failed to generate JSON report: %w", err)
		}
		log.Printf("Invoice-analytics diff report generated successfully: %s\n", outputFile)
	} else {
		outputFile := fmt.Sprintf("invoice_analytics_diff_%s_%s.csv", tenantID, timestamp)
		if err := generateInvoiceAnalyticsDiffCSV(diffs, outputFile); err != nil {
			return fmt.Errorf("failed to generate CSV report: %w", err)
		}
		log.Printf("Invoice-analytics diff report generated successfully: %s\n", outputFile)
	}

	log.Printf("Total discrepancies found: %d\n", len(diffs))

	return nil
}

func (s *invoiceAnalyticsDiffScript) getCustomersToProcess(ctx context.Context, externalCustomerID string) ([]*customer.Customer, error) {
	customerFilter := &types.CustomerFilter{
		QueryFilter: types.NewNoLimitQueryFilter(),
	}

	if externalCustomerID != "" {
		customerFilter.ExternalID = externalCustomerID
	}

	return s.customerRepo.ListAll(ctx, customerFilter)
}

func (s *invoiceAnalyticsDiffScript) processCustomer(ctx context.Context, cust *customer.Customer) ([]InvoiceAnalyticsDiff, error) {
	var diffs []InvoiceAnalyticsDiff

	// Get all finalized invoices for this customer
	invoiceFilter := &types.InvoiceFilter{
		QueryFilter:   types.NewNoLimitQueryFilter(),
		CustomerID:    cust.ID,
		InvoiceStatus: []types.InvoiceStatus{types.InvoiceStatusFinalized},
	}

	invoices, err := s.invoiceRepo.List(ctx, invoiceFilter)
	if err != nil {
		return nil, fmt.Errorf("failed to list invoices: %w", err)
	}

	for _, inv := range invoices {
		// Skip invoices without period dates
		if inv.PeriodStart == nil || inv.PeriodEnd == nil {
			continue
		}

		// Call analytics API for this customer and time range
		analyticsReq := &dto.GetUsageAnalyticsRequest{
			ExternalCustomerID: cust.ExternalID,
			StartTime:          *inv.PeriodStart,
			EndTime:            *inv.PeriodEnd,
		}

		analyticsResp, err := s.featureUsageService.GetDetailedUsageAnalytics(ctx, analyticsReq)
		if err != nil {
			log.Printf("Warning: Failed to get analytics for invoice %s: %v\n", inv.ID, err)
			continue
		}

		// Compare invoice total with analytics total
		difference := inv.Total.Sub(analyticsResp.TotalCost)

		// Compare line items
		lineItemDiffs := s.compareLineItems(inv, analyticsResp)

		// Only record if difference is non-negative (invoice >= analytics)
		// Skip cases where analytics has more than invoice (difference < 0)
		if difference.GreaterThanOrEqual(decimal.Zero) && (!difference.IsZero() || len(lineItemDiffs) > 0) {
			invoiceNumber := ""
			if inv.InvoiceNumber != nil {
				invoiceNumber = *inv.InvoiceNumber
			}

			diffs = append(diffs, InvoiceAnalyticsDiff{
				CustomerID:         cust.ID,
				ExternalCustomerID: cust.ExternalID,
				InvoiceID:          inv.ID,
				InvoiceNumber:      invoiceNumber,
				PeriodStart:        *inv.PeriodStart,
				PeriodEnd:          *inv.PeriodEnd,
				InvoiceTotal:       inv.Total,
				AnalyticsTotal:     analyticsResp.TotalCost,
				Difference:         difference,
				Currency:           inv.Currency,
				LineItemDiffs:      lineItemDiffs,
			})
		}
	}

	return diffs, nil
}

func (s *invoiceAnalyticsDiffScript) compareLineItems(inv *invoice.Invoice, analyticsResp *dto.GetUsageAnalyticsResponse) []LineItemDiff {
	var diffs []LineItemDiff

	// Create a map of analytics data by meter_id for quick lookup
	analyticsMap := make(map[string]*dto.UsageAnalyticItem)
	for i := range analyticsResp.Items {
		item := &analyticsResp.Items[i]
		if item.PriceID != "" {
			analyticsMap[item.PriceID] = item
		}
	}

	// Compare each invoice line item with corresponding analytics data
	for _, lineItem := range inv.LineItems {
		if lineItem.PriceID == nil {
			continue
		}

		priceID := *lineItem.PriceID
		analytic, exists := analyticsMap[priceID]

		var analyticsCost, analyticsUsage decimal.Decimal
		var meterID, meterName string

		if exists {
			analyticsCost = analytic.TotalCost
			analyticsUsage = analytic.TotalUsage
			meterID = analytic.MeterID
			meterName = analytic.EventName
		} else {
			analyticsCost = decimal.Zero
			analyticsUsage = decimal.Zero
			// Use invoice line item data when analytics doesn't have this price
			if lineItem.MeterID != nil {
				meterID = *lineItem.MeterID
			}
			if lineItem.DisplayName != nil {
				meterName = *lineItem.DisplayName
			}
		}

		amountDiff := lineItem.Amount.Sub(analyticsCost)
		quantityDiff := lineItem.Quantity.Sub(analyticsUsage)

		// Only add to diffs if there's a discrepancy
		if !amountDiff.IsZero() || !quantityDiff.IsZero() {
			diffs = append(diffs, LineItemDiff{
				MeterID:         meterID,
				MeterName:       meterName,
				PriceID:         priceID,
				InvoiceAmount:   lineItem.Amount,
				AnalyticsAmount: analyticsCost,
				Difference:      amountDiff,
				InvoiceQuantity: lineItem.Quantity,
				AnalyticsUsage:  analyticsUsage,
				QuantityDiff:    quantityDiff,
			})
		}
	}

	// Check for analytics data that doesn't have corresponding invoice line items
	for priceID, analytic := range analyticsMap {
		found := false
		for _, lineItem := range inv.LineItems {
			if lineItem.PriceID != nil && *lineItem.PriceID == priceID {
				found = true
				break
			}
		}

		if !found && !analytic.TotalCost.IsZero() {
			diffs = append(diffs, LineItemDiff{
				MeterID:         analytic.MeterID,
				MeterName:       analytic.EventName,
				PriceID:         priceID,
				InvoiceAmount:   decimal.Zero,
				AnalyticsAmount: analytic.TotalCost,
				Difference:      decimal.Zero.Sub(analytic.TotalCost),
				InvoiceQuantity: decimal.Zero,
				AnalyticsUsage:  analytic.TotalUsage,
				QuantityDiff:    decimal.Zero.Sub(analytic.TotalUsage),
			})
		}
	}

	return diffs
}

func generateInvoiceAnalyticsDiffJSON(data []InvoiceAnalyticsDiff, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create JSON file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(data); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	return nil
}

func generateInvoiceAnalyticsDiffCSV(data []InvoiceAnalyticsDiff, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create CSV file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	header := []string{
		"Customer ID",
		"External Customer ID",
		"Invoice ID",
		"Invoice Number",
		"Period Start",
		"Period End",
		"Invoice Total",
		"Analytics Total",
		"Difference",
		"Currency",
		"Line Item Diffs Count",
	}

	if err := writer.Write(header); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	// Write data rows
	for _, row := range data {
		record := []string{
			row.CustomerID,
			row.ExternalCustomerID,
			row.InvoiceID,
			row.InvoiceNumber,
			row.PeriodStart.Format(time.RFC3339),
			row.PeriodEnd.Format(time.RFC3339),
			row.InvoiceTotal.String(),
			row.AnalyticsTotal.String(),
			row.Difference.String(),
			row.Currency,
			fmt.Sprintf("%d", len(row.LineItemDiffs)),
		}

		if err := writer.Write(record); err != nil {
			return fmt.Errorf("failed to write CSV record: %w", err)
		}
	}

	return nil
}

func newInvoiceAnalyticsDiffScript() (*invoiceAnalyticsDiffScript, error) {
	// Load configuration
	cfg, err := config.NewConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize logger
	log, err := logger.NewLogger(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create logger: %w", err)
	}

	// Initialize ClickHouse client for event repositories
	sentryService := sentry.NewSentryService(cfg, log)
	chStore, err := clickhouse.NewClickHouseStore(cfg, sentryService)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to clickhouse: %w", err)
	}

	// Initialize postgres client
	entClient, err := postgres.NewEntClients(cfg, log)
	if err != nil {
		log.Fatalf("Failed to connect to postgres: %v", err)
	}
	client := postgres.NewClient(entClient, log, sentryService)
	cacheClient := cache.NewInMemoryCache()

	// Create repositories
	customerRepo := entRepo.NewCustomerRepository(client, log, cacheClient)
	invoiceRepo := entRepo.NewInvoiceRepository(client, log, cacheClient)
	subscriptionRepo := entRepo.NewSubscriptionRepository(client, log, cacheClient)
	subscriptionLineItemRepo := entRepo.NewSubscriptionLineItemRepository(client, log, cacheClient)
	subscriptionPhaseRepo := entRepo.NewSubscriptionPhaseRepository(client, log, cacheClient)
	planRepo := entRepo.NewPlanRepository(client, log, cacheClient)
	priceRepo := entRepo.NewPriceRepository(client, log, cacheClient)
	meterRepo := entRepo.NewMeterRepository(client, log, cacheClient)
	featureRepo := entRepo.NewFeatureRepository(client, log, cacheClient)
	entitlementRepo := entRepo.NewEntitlementRepository(client, log, cacheClient)
	addonRepo := entRepo.NewAddonRepository(client, log, cacheClient)
	addonAssociationRepo := entRepo.NewAddonAssociationRepository(client, log, cacheClient)
	walletRepo := entRepo.NewWalletRepository(client, log, cacheClient)
	eventRepo := chRepo.NewEventRepository(chStore, log)
	processedEventRepo := chRepo.NewProcessedEventRepository(chStore, log)
	featureUsageRepo := chRepo.NewFeatureUsageRepository(chStore, log)

	// Create service params
	serviceParams := service.ServiceParams{
		Logger:                   log,
		Config:                   cfg,
		DB:                       client,
		CustomerRepo:             customerRepo,
		WalletRepo:               walletRepo,
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
		EventRepo:                eventRepo,
		ProcessedEventRepo:       processedEventRepo,
		FeatureUsageRepo:         featureUsageRepo,
	}

	// Create feature usage tracking service
	featureUsageService := service.NewFeatureUsageTrackingService(serviceParams, eventRepo, featureUsageRepo)

	return &invoiceAnalyticsDiffScript{
		log:                 log,
		customerRepo:        customerRepo,
		invoiceRepo:         invoiceRepo,
		featureUsageService: featureUsageService,
	}, nil
}
