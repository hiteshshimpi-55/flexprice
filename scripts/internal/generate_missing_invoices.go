package internal

import (
	"context"
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/flexprice/flexprice/internal/api/dto"
	"github.com/flexprice/flexprice/internal/cache"
	"github.com/flexprice/flexprice/internal/clickhouse"
	"github.com/flexprice/flexprice/internal/config"
	"github.com/flexprice/flexprice/internal/logger"
	"github.com/flexprice/flexprice/internal/postgres"
	chRepo "github.com/flexprice/flexprice/internal/repository/clickhouse"
	entRepo "github.com/flexprice/flexprice/internal/repository/ent"
	"github.com/flexprice/flexprice/internal/sentry"
	"github.com/flexprice/flexprice/internal/service"
	"github.com/flexprice/flexprice/internal/types"
)

// csvInvoiceRow represents a parsed row from the reconciliation CSV
type csvInvoiceRow struct {
	CustomerID       string
	SubscriptionID   string
	InvoiceID        string
	PeriodStart      time.Time
	PeriodEnd        time.Time
	AnalyticsAmount  float64
	InvoiceSubtotal  string
	Diff             string
	InvoiceGenerated bool
}

// GenerateMissingInvoices reads a reconciliation CSV and generates invoices
// for rows where invoice_generated=false AND analytics_amount > 0.
//
// Environment variables:
//   - TENANT_ID (required)
//   - ENVIRONMENT_ID (required)
//   - FILE_PATH (required) — path to the reconciliation CSV
//   - DRY_RUN (optional, default "true") — "true" prints summary only, "false" generates invoices
func GenerateMissingInvoices() error {
	tenantID := os.Getenv("TENANT_ID")
	environmentID := os.Getenv("ENVIRONMENT_ID")
	filePath := os.Getenv("FILE_PATH")
	dryRun := os.Getenv("DRY_RUN")

	if tenantID == "" || environmentID == "" {
		return fmt.Errorf("TENANT_ID and ENVIRONMENT_ID are required")
	}

	if filePath == "" {
		return fmt.Errorf("FILE_PATH is required (path to reconciliation CSV)")
	}

	// Default to dry-run mode for safety
	isDryRun := true
	if strings.EqualFold(dryRun, "false") {
		isDryRun = false
	}

	mode := "DRY-RUN"
	if !isDryRun {
		mode = "EXECUTE"
	}

	log.Printf("[%s] Starting generate-missing-invoices for tenant=%s env=%s file=%s\n",
		mode, tenantID, environmentID, filePath)

	filePath = "/Users/hiteshshimpi/Desktop/Work/flexprice/missing_invoices/" + filePath
	// 1. Read and parse CSV
	rows, err := readReconciliationCSV(filePath)
	if err != nil {
		return fmt.Errorf("failed to read CSV: %w", err)
	}
	log.Printf("Read %d total rows from CSV\n", len(rows))

	// 2. Filter: invoice_generated=false AND analytics_amount > 0
	var filtered []csvInvoiceRow
	for _, row := range rows {
		if !row.InvoiceGenerated && row.AnalyticsAmount > 0 {
			filtered = append(filtered, row)
		}
	}

	log.Printf("Found %d rows requiring invoice generation (invoice_generated=false AND analytics_amount > 0)\n", len(filtered))

	if len(filtered) == 0 {
		log.Println("No invoices to generate. Exiting.")
		return nil
	}

	// 3. Print summary (both modes)
	log.Println("\n=== Invoice Generation Summary ===")
	log.Printf("%-40s %-40s %-30s %-30s %-15s\n",
		"SUBSCRIPTION_ID", "CUSTOMER_ID", "PERIOD_START", "PERIOD_END", "ANALYTICS_AMT")
	log.Println(strings.Repeat("-", 155))

	uniqueSubscriptions := make(map[string]bool)
	uniqueCustomers := make(map[string]bool)
	for _, row := range filtered {
		uniqueSubscriptions[row.SubscriptionID] = true
		uniqueCustomers[row.CustomerID] = true
		log.Printf("%-40s %-40s %-30s %-30s %-15.2f\n",
			row.SubscriptionID,
			row.CustomerID,
			row.PeriodStart.Format(time.RFC3339),
			row.PeriodEnd.Format(time.RFC3339),
			row.AnalyticsAmount,
		)
	}

	log.Printf("\nTotal invoices to generate: %d\n", len(filtered))
	log.Printf("Unique subscriptions: %d\n", len(uniqueSubscriptions))
	log.Printf("Unique customers: %d\n", len(uniqueCustomers))

	if isDryRun {
		outputFilePath := strings.Replace(filePath, ".csv", "_dry_run.csv", 1)
		if err := writeFilteredReconciliationCSV(filtered, outputFilePath); err != nil {
			log.Printf("Failed to write dry-run CSV: %v\n", err)
		} else {
			log.Printf("Wrote dry-run filtered rows to: %s\n", outputFilePath)
		}

		log.Println("\n[DRY-RUN] No invoices were generated. Run with --dry-run=false to execute.")
		return nil
	}

	// 4. Execute mode: generate invoices
	log.Println("\n=== Generating Invoices ===")

	invoiceService, err := newInvoiceGenerationDeps()
	if err != nil {
		return fmt.Errorf("failed to initialize dependencies: %w", err)
	}

	ctx := context.Background()
	ctx = context.WithValue(ctx, types.CtxTenantID, tenantID)
	ctx = context.WithValue(ctx, types.CtxEnvironmentID, environmentID)

	var succeeded, failed int

	for i, row := range filtered {
		log.Printf("[%d/%d] Generating invoice for subscription=%s period=%s to %s ...\n",
			i+1, len(filtered),
			row.SubscriptionID,
			row.PeriodStart.Format(time.RFC3339),
			row.PeriodEnd.Format(time.RFC3339),
		)

		req := &dto.CreateSubscriptionInvoiceRequest{
			SubscriptionID: row.SubscriptionID,
			PeriodStart:    row.PeriodStart,
			PeriodEnd:      row.PeriodEnd,
		}

		inv, err := invoiceService.GenerateSubscriptionInvoiceForPeriod(ctx, req)
		if err != nil {
			log.Printf("  ERROR: failed to generate invoice: %v\n", err)
			failed++
			continue
		}

		if inv == nil {
			log.Printf("  SKIPPED: zero amount invoice (no charges for the period)\n")
			succeeded++ // not an error, just no charges
			continue
		}

		log.Printf("  SUCCESS: invoice_id=%s subscription_id=%s subtotal=%s total=%s\n",
			inv.ID,
			row.SubscriptionID,
			inv.Subtotal.StringFixed(2),
			inv.Total.StringFixed(2),
		)
		succeeded++
	}

	log.Println("\n=== Generation Complete ===")
	log.Printf("Total attempted: %d\n", len(filtered))
	log.Printf("Succeeded: %d\n", succeeded)
	log.Printf("Failed: %d\n", failed)

	return nil
}

// readReconciliationCSV reads and parses the reconciliation CSV file.
// Expected columns: customer_id, subscription_id, invoice_id, period_start, period_end,
// analytics_amount, invoice_subtotal, diff, invoice_generated
func readReconciliationCSV(filePath string) ([]csvInvoiceRow, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV: %w", err)
	}

	if len(records) < 2 {
		return nil, fmt.Errorf("CSV file has no data rows (only %d rows found)", len(records))
	}

	// Skip header row
	var rows []csvInvoiceRow
	for i, record := range records[1:] {
		if len(record) < 9 {
			log.Printf("WARNING: skipping row %d, expected 9 columns but got %d\n", i+2, len(record))
			continue
		}

		periodStart, err := time.Parse(time.RFC3339, strings.TrimSpace(record[3]))
		if err != nil {
			log.Printf("WARNING: skipping row %d, invalid period_start '%s': %v\n", i+2, record[3], err)
			continue
		}

		periodEnd, err := time.Parse(time.RFC3339, strings.TrimSpace(record[4]))
		if err != nil {
			log.Printf("WARNING: skipping row %d, invalid period_end '%s': %v\n", i+2, record[4], err)
			continue
		}

		analyticsAmount, err := strconv.ParseFloat(strings.TrimSpace(record[5]), 64)
		if err != nil {
			log.Printf("WARNING: skipping row %d, invalid analytics_amount '%s': %v\n", i+2, record[5], err)
			continue
		}

		invoiceGenerated := strings.EqualFold(strings.TrimSpace(record[8]), "true")

		rows = append(rows, csvInvoiceRow{
			CustomerID:       strings.TrimSpace(record[0]),
			SubscriptionID:   strings.TrimSpace(record[1]),
			InvoiceID:        strings.TrimSpace(record[2]),
			PeriodStart:      periodStart,
			PeriodEnd:        periodEnd,
			AnalyticsAmount:  analyticsAmount,
			InvoiceSubtotal:  strings.TrimSpace(record[6]),
			Diff:             strings.TrimSpace(record[7]),
			InvoiceGenerated: invoiceGenerated,
		})
	}

	return rows, nil
}

// writeFilteredReconciliationCSV writes the filtered reconciliation rows to a CSV file.
func writeFilteredReconciliationCSV(rows []csvInvoiceRow, filename string) error {
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
			row.PeriodStart.Format(time.RFC3339),
			row.PeriodEnd.Format(time.RFC3339),
			fmt.Sprintf("%f", row.AnalyticsAmount),
			row.InvoiceSubtotal,
			row.Diff,
			fmt.Sprintf("%t", row.InvoiceGenerated),
		}
		if err := writer.Write(record); err != nil {
			return fmt.Errorf("failed to write CSV record: %w", err)
		}
	}

	return nil
}

// newInvoiceGenerationDeps initialises the InvoiceService with all required dependencies.
func newInvoiceGenerationDeps() (service.InvoiceService, error) {
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

	return service.NewInvoiceService(serviceParams), nil
}
