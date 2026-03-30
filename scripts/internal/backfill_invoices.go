package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/flexprice/flexprice/internal/api/dto"
	"github.com/flexprice/flexprice/internal/cache"
	"github.com/flexprice/flexprice/internal/clickhouse"
	"github.com/flexprice/flexprice/internal/config"
	domainInvoice "github.com/flexprice/flexprice/internal/domain/invoice"
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

type backfillInvoicesScript struct {
	log            *logger.Logger
	subRepo        domainSubscription.Repository
	invoiceRepo    domainInvoice.Repository
	invoiceService service.InvoiceService
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
	rawBytes, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	var input backfillInput
	if err := json.Unmarshal(rawBytes, &input); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}

	if len(input.SubscriptionIDs) == 0 {
		return fmt.Errorf("no subscription_ids found in input file")
	}

	log.Printf("Found %d subscription(s) to process", len(input.SubscriptionIDs))

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
	var results []backfillResult

	for i, subID := range input.SubscriptionIDs {
		log.Printf("[%d/%d] Processing subscription: %s", i+1, len(input.SubscriptionIDs), subID)

		result := backfillResult{SubscriptionID: subID}

		sub, err := script.subRepo.Get(ctx, subID)
		if err != nil {
			result.Error = fmt.Sprintf("failed to fetch subscription: %v", err)
			log.Printf("  ERROR: %s", result.Error)
			results = append(results, result)
			continue
		}

		// Validate tenant/environment match
		if sub.TenantID != tenantID {
			result.Error = fmt.Sprintf("tenant mismatch: subscription has %s, expected %s", sub.TenantID, tenantID)
			log.Printf("  SKIP: %s", result.Error)
			results = append(results, result)
			continue
		}
		if sub.EnvironmentID != environmentID {
			result.Error = fmt.Sprintf("environment mismatch: subscription has %s, expected %s", sub.EnvironmentID, environmentID)
			log.Printf("  SKIP: %s", result.Error)
			results = append(results, result)
			continue
		}

		// Determine billing anchor
		anchor := sub.BillingAnchor
		if sub.BillingCycle == types.BillingCycleCalendar {
			anchor = types.CalculateCalendarBillingAnchor(sub.StartDate, sub.BillingPeriod)
		}

		// Determine end boundary
		endDate := now
		if sub.EndDate != nil && sub.EndDate.Before(now) {
			endDate = *sub.EndDate
		}

		// Calculate billing periods
		periods, err := types.CalculateBillingPeriods(sub.StartDate, &endDate, anchor, sub.BillingPeriodCount, sub.BillingPeriod)
		if err != nil {
			result.Error = fmt.Sprintf("failed to calculate periods: %v", err)
			log.Printf("  ERROR: %s", result.Error)
			results = append(results, result)
			continue
		}

		result.PeriodsTotal = len(periods)
		log.Printf("  Found %d billing period(s) from %s to %s", len(periods), sub.StartDate.Format("2006-01-02"), endDate.Format("2006-01-02"))

		for j, period := range periods {
			// Only process periods that have ended (skip current/future periods)
			if period.End.After(now) {
				result.PeriodsSkipped++
				continue
			}

			exists, err := script.invoiceRepo.ExistsForPeriod(ctx, sub.ID, period.Start, period.End)
			if err != nil {
				log.Printf("  [%d/%d] ERROR checking period %s - %s: %v", j+1, len(periods), period.Start.Format("2006-01-02"), period.End.Format("2006-01-02"), err)
				continue
			}

			if exists {
				result.PeriodsSkipped++
				continue
			}

			if isDryRun {
				log.Printf("  [%d/%d] WOULD CREATE invoice for period %s - %s", j+1, len(periods), period.Start.Format("2006-01-02"), period.End.Format("2006-01-02"))
				result.PeriodsCreated++
				continue
			}

			log.Printf("  [%d/%d] Creating invoice for period %s - %s", j+1, len(periods), period.Start.Format("2006-01-02"), period.End.Format("2006-01-02"))

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
				log.Printf("  [%d/%d] ERROR creating invoice: %v", j+1, len(periods), err)
				continue
			}

			result.PeriodsCreated++
			if inv != nil {
				log.Printf("  [%d/%d] Created invoice %s", j+1, len(periods), inv.ID)
			} else {
				log.Printf("  [%d/%d] Invoice skipped (zero-dollar)", j+1, len(periods))
			}
		}

		results = append(results, result)
		log.Printf("  Done: %d total, %d skipped, %d created", result.PeriodsTotal, result.PeriodsSkipped, result.PeriodsCreated)
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
	}

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
	eventRepo := chRepo.NewEventRepository(chStore, lg)
	processedEventRepo := chRepo.NewProcessedEventRepository(chStore, lg)
	featureUsageRepo := chRepo.NewFeatureUsageRepository(chStore, lg)

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
		EventRepo:                  eventRepo,
		ProcessedEventRepo:         processedEventRepo,
		FeatureUsageRepo:           featureUsageRepo,
	}

	invoiceService := service.NewInvoiceService(serviceParams)

	return &backfillInvoicesScript{
		log:            lg,
		subRepo:        subscriptionRepo,
		invoiceRepo:    invoiceRepo,
		invoiceService: invoiceService,
	}, nil
}
