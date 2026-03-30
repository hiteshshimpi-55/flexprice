package internal

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/flexprice/flexprice/internal/api/dto"
	"github.com/flexprice/flexprice/internal/cache"
	"github.com/flexprice/flexprice/internal/clickhouse"
	"github.com/flexprice/flexprice/internal/config"
	domainInvoice "github.com/flexprice/flexprice/internal/domain/invoice"
	"github.com/flexprice/flexprice/internal/domain/proration"
	domainSubscription "github.com/flexprice/flexprice/internal/domain/subscription"
	"github.com/flexprice/flexprice/internal/logger"
	"github.com/flexprice/flexprice/internal/postgres"
	chRepo "github.com/flexprice/flexprice/internal/repository/clickhouse"
	entRepo "github.com/flexprice/flexprice/internal/repository/ent"
	"github.com/flexprice/flexprice/internal/sentry"
	"github.com/flexprice/flexprice/internal/service"
	"github.com/flexprice/flexprice/internal/types"
)

type backfillInput struct {
	SubscriptionIDs []string `json:"subscription_ids"`
}

type backfillResult struct {
	SubscriptionID string `json:"subscription_id"`
	PeriodsTotal   int    `json:"periods_total"`
	PeriodsSkipped int    `json:"periods_skipped"`
	PeriodsCreated int    `json:"periods_created"`
	Error          string `json:"error,omitempty"`
}

type backfillOutputRow struct {
	SubscriptionID    string
	CustomerID        string
	PlanID            string
	BillingPeriod     string
	PeriodStart       string
	PeriodEnd         string
	SubscriptionStart string
	SubscriptionEnd   string
	InvoiceID         string // set when an invoice row was created; empty on dry run or zero-dollar skip
}

type backfillInvoicesScript struct {
	log            *logger.Logger
	subRepo        domainSubscription.Repository
	invoiceRepo    domainInvoice.Repository
	invoiceService service.InvoiceService
}

// readBackfillInputFile loads the subscription list JSON. If filePath is relative and missing,
// it retries scripts/<basename> so ./subscriptions.json from repo root finds scripts/subscriptions.json.
func readBackfillInputFile(filePath string) ([]byte, error) {
	b, err := os.ReadFile(filePath)
	if err == nil {
		return b, nil
	}
	if !errors.Is(err, os.ErrNotExist) ||
		filepath.IsAbs(filePath) {
		return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}
	base := filepath.Base(filepath.Clean(filePath))
	if base == "" || base == "." {
		return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}
	alt := filepath.Join("scripts", base)
	b, err2 := os.ReadFile(alt)
	if err2 != nil {
		return nil, fmt.Errorf("failed to read file %s (also tried %s): %w", filePath, alt, err)
	}
	log.Printf("Using input file %s (not found at %s)", alt, filePath)
	return b, nil
}

func backfillWorkerCount() int {
	const defaultWorkers = 4
	raw := os.Getenv("BACKFILL_WORKERS")
	if raw == "" {
		return defaultWorkers
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return defaultWorkers
	}
	return n
}

// processBackfillSubscription handles one subscription: billing periods, existence checks,
// and invoice creation (or dry-run rows). Intended to run concurrently; callers share one
// script (repos/services use the process-wide DB pool).
func processBackfillSubscription(
	ctx context.Context,
	script *backfillInvoicesScript,
	tenantID, environmentID string,
	subID string,
	now time.Time,
	isDryRun bool,
	idx, total int,
) (backfillResult, []backfillOutputRow) {
	result := backfillResult{SubscriptionID: subID}
	var outputRows []backfillOutputRow

	log.Printf("[%d/%d] Processing subscription: %s", idx, total, subID)

	sub, err := script.subRepo.Get(ctx, subID)
	if err != nil {
		result.Error = fmt.Sprintf("failed to fetch subscription: %v", err)
		log.Printf("  ERROR [%s]: %s", subID, result.Error)
		return result, outputRows
	}

	if sub.TenantID != tenantID {
		result.Error = fmt.Sprintf("tenant mismatch: subscription has %s, expected %s", sub.TenantID, tenantID)
		log.Printf("  SKIP [%s]: %s", subID, result.Error)
		return result, outputRows
	}
	if sub.EnvironmentID != environmentID {
		result.Error = fmt.Sprintf("environment mismatch: subscription has %s, expected %s", sub.EnvironmentID, environmentID)
		log.Printf("  SKIP [%s]: %s", subID, result.Error)
		return result, outputRows
	}

	anchor := sub.BillingAnchor
	if sub.BillingCycle == types.BillingCycleCalendar {
		anchor = types.CalculateCalendarBillingAnchor(sub.StartDate, sub.BillingPeriod)
	}

	endDate := now
	if sub.EndDate != nil && sub.EndDate.Before(now) {
		endDate = *sub.EndDate
	}

	periods, err := types.CalculateBillingPeriods(sub.StartDate, &endDate, anchor, sub.BillingPeriodCount, sub.BillingPeriod)
	if err != nil {
		result.Error = fmt.Sprintf("failed to calculate periods: %v", err)
		log.Printf("  ERROR [%s]: %s", subID, result.Error)
		return result, outputRows
	}

	result.PeriodsTotal = len(periods)
	log.Printf("  [%s] Found %d billing period(s) from %s to %s", subID, len(periods), sub.StartDate.Format("2006-01-02"), endDate.Format("2006-01-02"))

	for j, period := range periods {

		exists, err := script.invoiceRepo.ExistsForPeriod(ctx, sub.ID, period.Start, period.End)
		if err != nil {
			log.Printf("  [%s] [%d/%d] ERROR checking period %s - %s: %v", subID, j+1, len(periods), period.Start.Format("2006-01-02"), period.End.Format("2006-01-02"), err)
			continue
		}

		if exists {
			result.PeriodsSkipped++
			continue
		}

		if isDryRun {
			log.Printf("  [%s] [%d/%d] WOULD CREATE invoice for period %s - %s", subID, j+1, len(periods), period.Start.Format("2006-01-02"), period.End.Format("2006-01-02"))
			subEnd := ""
			if sub.EndDate != nil {
				subEnd = sub.EndDate.Format("2006-01-02")
			}
			outputRows = append(outputRows, backfillOutputRow{
				SubscriptionID:    sub.ID,
				CustomerID:        sub.CustomerID,
				PlanID:            sub.PlanID,
				BillingPeriod:     string(sub.BillingPeriod),
				PeriodStart:       period.Start.Format("2006-01-02"),
				PeriodEnd:         period.End.Format("2006-01-02"),
				SubscriptionStart: sub.StartDate.Format("2006-01-02"),
				SubscriptionEnd:   subEnd,
			})
			result.PeriodsCreated++
			continue
		}

		log.Printf("  [%s] [%d/%d] Creating invoice for period %s - %s", subID, j+1, len(periods), period.Start.Format("2006-01-02"), period.End.Format("2006-01-02"))

		inv, _, err := script.invoiceService.CreateSubscriptionInvoice(ctx,
			&dto.CreateSubscriptionInvoiceRequest{
				SubscriptionID: sub.ID,
				PeriodStart:    period.Start,
				PeriodEnd:      period.End,
				ReferencePoint: types.ReferencePointPeriodEnd,
			},
			nil,
			types.InvoiceFlowRenewal,
			false,
		)
		if err != nil {
			log.Printf("  [%s] [%d/%d] ERROR creating invoice: %v", subID, j+1, len(periods), err)
			continue
		}

		result.PeriodsCreated++
		subEnd := ""
		if sub.EndDate != nil {
			subEnd = sub.EndDate.Format("2006-01-02")
		}
		invID := ""
		if inv != nil {
			invID = inv.ID
			log.Printf("  [%s] [%d/%d] Created invoice %s", subID, j+1, len(periods), inv.ID)
		} else {
			log.Printf("  [%s] [%d/%d] Invoice skipped (zero-dollar)", subID, j+1, len(periods))
		}
		outputRows = append(outputRows, backfillOutputRow{
			SubscriptionID:    sub.ID,
			CustomerID:        sub.CustomerID,
			PlanID:            sub.PlanID,
			BillingPeriod:     string(sub.BillingPeriod),
			PeriodStart:       period.Start.Format("2006-01-02"),
			PeriodEnd:         period.End.Format("2006-01-02"),
			SubscriptionStart: sub.StartDate.Format("2006-01-02"),
			SubscriptionEnd:   subEnd,
			InvoiceID:         invID,
		})
	}

	log.Printf("  Done [%s]: %d total, %d skipped, %d created", subID, result.PeriodsTotal, result.PeriodsSkipped, result.PeriodsCreated)
	return result, outputRows
}

func BackfillInvoices() error {
	filePath := os.Getenv("FILE_PATH")
	tenantID := os.Getenv("TENANT_ID")
	environmentID := os.Getenv("ENVIRONMENT_ID")
	userID := os.Getenv("USER_ID")
	isDryRun := os.Getenv("DRY_RUN") == "true"

	if filePath == "" {
		return fmt.Errorf("FILE_PATH is required")
	}
	if tenantID == "" || environmentID == "" {
		return fmt.Errorf("TENANT_ID and ENVIRONMENT_ID are required")
	}

	if isDryRun {
		log.Println("DRY RUN MODE - No invoices will be created")
	}

	// Read and parse input file
	rawBytes, err := readBackfillInputFile(filePath)
	if err != nil {
		return err
	}

	var input backfillInput
	if err := json.Unmarshal(rawBytes, &input); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}

	if len(input.SubscriptionIDs) == 0 {
		return fmt.Errorf("no subscription_ids found in input file")
	}

	nSubs := len(input.SubscriptionIDs)
	workers := backfillWorkerCount()
	log.Printf("Found %d subscription(s) to process (BACKFILL_WORKERS=%d)", nSubs, workers)

	// Initialize infrastructure
	script, err := newBackfillInvoicesScript()
	if err != nil {
		return fmt.Errorf("failed to initialize: %w", err)
	}

	// Build context
	ctx := context.Background()
	ctx = types.SetTenantID(ctx, tenantID)
	ctx = types.SetEnvironmentID(ctx, environmentID)
	if userID != "" {
		ctx = types.SetUserID(ctx, userID)
	}

	now := time.Now().UTC()
	results := make([]backfillResult, nSubs)
	outputChunks := make([][]backfillOutputRow, nSubs)

	if workers > nSubs {
		workers = nSubs
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, workers)

	for i, subID := range input.SubscriptionIDs {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, subID string) {
			defer wg.Done()
			defer func() { <-sem }()
			res, rows := processBackfillSubscription(ctx, script, tenantID, environmentID, subID, now, isDryRun, i+1, nSubs)
			results[i] = res
			outputChunks[i] = rows
		}(i, subID)
	}
	wg.Wait()

	var outputRows []backfillOutputRow
	for _, chunk := range outputChunks {
		outputRows = append(outputRows, chunk...)
	}

	// Print summary
	totalCreated, totalSkipped, totalErrors := 0, 0, 0
	for _, r := range results {
		totalCreated += r.PeriodsCreated
		totalSkipped += r.PeriodsSkipped
		if r.Error != "" {
			totalErrors++
		}
	}
	log.Printf("=== SUMMARY ===")
	log.Printf("Subscriptions: %d processed, %d errors", len(results), totalErrors)
	log.Printf("Periods: %d created, %d skipped", totalCreated, totalSkipped)
	if isDryRun {
		log.Printf("(DRY RUN - no invoices were actually created)")

		if len(outputRows) > 0 {
			if err := writeBackfillOutputCSV(outputRows, "dry_run"); err != nil {
				return fmt.Errorf("failed to write dry run CSV: %w", err)
			}
		} else {
			log.Println("No missing invoices found - nothing to export")
		}
	} else if len(outputRows) > 0 {
		if err := writeBackfillOutputCSV(outputRows, "created"); err != nil {
			return fmt.Errorf("failed to write created invoices CSV: %w", err)
		}
	} else {
		log.Println("No invoices written to CSV (no new periods created)")
	}

	return nil
}

func writeBackfillOutputCSV(rows []backfillOutputRow, nameSuffix string) error {
	timestamp := time.Now().Format("20060102_150405")
	filename := filepath.Join("scripts", "internal", fmt.Sprintf("backfill_invoices_%s_%s.csv", nameSuffix, timestamp))

	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create CSV file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	header := []string{
		"subscription_id",
		"customer_id",
		"plan_id",
		"billing_period",
		"period_start",
		"period_end",
		"subscription_start",
		"subscription_end",
		"invoice_id",
	}
	if err := writer.Write(header); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	for _, row := range rows {
		record := []string{
			row.SubscriptionID,
			row.CustomerID,
			row.PlanID,
			row.BillingPeriod,
			row.PeriodStart,
			row.PeriodEnd,
			row.SubscriptionStart,
			row.SubscriptionEnd,
			row.InvoiceID,
		}
		if err := writer.Write(record); err != nil {
			return fmt.Errorf("failed to write CSV row: %w", err)
		}
	}

	log.Printf("Backfill CSV exported to %s (%d rows)", filename, len(rows))
	return nil
}

func newBackfillInvoicesScript() (*backfillInvoicesScript, error) {
	cfg, err := config.NewConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	lg, err := logger.NewLogger(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create logger: %w", err)
	}

	sentryService := sentry.NewSentryService(cfg, lg)

	chStore, err := clickhouse.NewClickHouseStore(cfg, sentryService)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to clickhouse: %w", err)
	}

	entClients, err := postgres.NewEntClients(cfg, lg)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to postgres: %w", err)
	}
	client := postgres.NewClient(entClients, lg, sentryService)
	cacheClient := cache.NewInMemoryCache()

	// Create repositories
	customerRepo := entRepo.NewCustomerRepository(client, lg, cacheClient)
	walletRepo := entRepo.NewWalletRepository(client, lg, cacheClient)
	subscriptionRepo := entRepo.NewSubscriptionRepository(client, lg, cacheClient)
	subscriptionLineItemRepo := entRepo.NewSubscriptionLineItemRepository(client, lg, cacheClient)
	subscriptionPhaseRepo := entRepo.NewSubscriptionPhaseRepository(client, lg, cacheClient)
	planRepo := entRepo.NewPlanRepository(client, lg, cacheClient)
	priceRepo := entRepo.NewPriceRepository(client, lg, cacheClient)
	meterRepo := entRepo.NewMeterRepository(client, lg, cacheClient)
	featureRepo := entRepo.NewFeatureRepository(client, lg, cacheClient)
	entitlementRepo := entRepo.NewEntitlementRepository(client, lg, cacheClient)
	addonRepo := entRepo.NewAddonRepository(client, lg, cacheClient)
	addonAssociationRepo := entRepo.NewAddonAssociationRepository(client, lg, cacheClient)
	invoiceRepo := entRepo.NewInvoiceRepository(client, lg, cacheClient)
	invoiceLineItemRepo := entRepo.NewInvoiceLineItemRepository(client, lg, cacheClient)
	creditGrantRepo := entRepo.NewCreditGrantRepository(client, lg, cacheClient)
	creditGrantAppRepo := entRepo.NewCreditGrantApplicationRepository(client, lg, cacheClient)
	couponRepo := entRepo.NewCouponRepository(client, lg, cacheClient)
	couponAssociationRepo := entRepo.NewCouponAssociationRepository(client, lg, cacheClient)
	couponApplicationRepo := entRepo.NewCouponApplicationRepository(client, lg, cacheClient)
	taxRateRepo := entRepo.NewTaxRateRepository(client, lg, cacheClient)
	taxAssociationRepo := entRepo.NewTaxAssociationRepository(client, lg, cacheClient)
	taxAppliedRepo := entRepo.NewTaxAppliedRepository(client, lg, cacheClient)
	settingsRepo := entRepo.NewSettingsRepository(client, lg, cacheClient)
	priceUnitRepo := entRepo.NewPriceUnitRepository(client, lg, cacheClient)
	eventRepo := chRepo.NewEventRepository(chStore, lg)
	processedEventRepo := chRepo.NewProcessedEventRepository(chStore, lg)
	featureUsageRepo := chRepo.NewFeatureUsageRepository(chStore, lg)
	prorationCalculator := proration.NewCalculator(lg)

	serviceParams := service.ServiceParams{
		Logger:                     lg,
		Config:                     cfg,
		DB:                         client,
		CustomerRepo:               customerRepo,
		WalletRepo:                 walletRepo,
		SubRepo:                    subscriptionRepo,
		SubscriptionLineItemRepo:   subscriptionLineItemRepo,
		SubscriptionPhaseRepo:      subscriptionPhaseRepo,
		PlanRepo:                   planRepo,
		PriceRepo:                  priceRepo,
		PriceUnitRepo:              priceUnitRepo,
		MeterRepo:                  meterRepo,
		FeatureRepo:                featureRepo,
		EntitlementRepo:            entitlementRepo,
		AddonRepo:                  addonRepo,
		AddonAssociationRepo:       addonAssociationRepo,
		InvoiceRepo:                invoiceRepo,
		InvoiceLineItemRepo:        invoiceLineItemRepo,
		CreditGrantRepo:            creditGrantRepo,
		CreditGrantApplicationRepo: creditGrantAppRepo,
		CouponRepo:                 couponRepo,
		CouponAssociationRepo:      couponAssociationRepo,
		CouponApplicationRepo:      couponApplicationRepo,
		TaxRateRepo:                taxRateRepo,
		TaxAssociationRepo:         taxAssociationRepo,
		TaxAppliedRepo:             taxAppliedRepo,
		SettingsRepo:               settingsRepo,
		EventRepo:                  eventRepo,
		ProcessedEventRepo:         processedEventRepo,
		FeatureUsageRepo:           featureUsageRepo,
		ProrationCalculator:        prorationCalculator,
	}

	invoiceService := service.NewInvoiceService(serviceParams)

	return &backfillInvoicesScript{
		log:            lg,
		subRepo:        subscriptionRepo,
		invoiceRepo:    invoiceRepo,
		invoiceService: invoiceService,
	}, nil
}
