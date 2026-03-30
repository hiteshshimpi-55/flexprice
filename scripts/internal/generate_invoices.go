package internal

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/flexprice/flexprice/internal/cache"
	"github.com/flexprice/flexprice/internal/clickhouse"
	"github.com/flexprice/flexprice/internal/config"
	"github.com/flexprice/flexprice/internal/domain/invoice"
	"github.com/flexprice/flexprice/internal/domain/subscription"
	"github.com/flexprice/flexprice/internal/idempotency"
	"github.com/flexprice/flexprice/internal/logger"
	"github.com/flexprice/flexprice/internal/postgres"
	chRepo "github.com/flexprice/flexprice/internal/repository/clickhouse"
	entRepo "github.com/flexprice/flexprice/internal/repository/ent"
	"github.com/flexprice/flexprice/internal/sentry"
	"github.com/flexprice/flexprice/internal/service"
	"github.com/flexprice/flexprice/internal/types"
)

type generateInvoicesScript struct {
	log              *logger.Logger
	subscriptionRepo subscription.Repository
	billingSvc       service.BillingService
	invoiceRepo      invoice.Repository
}

// Hardcode internal customer IDs and the minimum CurrentPeriodStart (UTC) for this script.
var (
	generateInvoicesCustomerIDs = []string{
		"cust_01KFB8ZEPPT7HE8RXDR90CV14Y0",
		"cust_01KFB88PEZFMYKBZZ5QSE421NE0",
		"cust_01KFB88YMPA498VE1TZ6WJ51QV0",
		"cust_01KFB88RMC2AXWGB52PYM09RFC0",
		"cust_01KFB896D9DNE06CA6F04C6HEP0",
		"cust_01KFB89PPB4RDCQ2YTPBXGNEZV0",
		"cust_01KFB8A3JHXNK8QVEZJ5WJ7KPK0",
		"cust_01KFB8A255R5FPF6TW0E0YSEH00",
		"cust_01KFB89WXCXX2TT2PJHGHJMY600",
		"cust_01KFB89JEFYM6VV5HHPK1B79N50",
		"cust_01KFB8AZV7N264ZY19FCXR7JAF0",
		"cust_01KFB8AQA16XWTD7RCNSCH3NNG0",
		"cust_01KFB8B5JQB98NMCSWBZPXZ9JM0",
		"cust_01KFB8BC13KKBEAP0FTY40AQTP0",
		"cust_01KFB8ERXHATWFMK0VJ18245EF0",
		"cust_01KFB8EDFF6371BF1D7ZCAWMVF0",
		"cust_01KFB8FGAMDAEV0C1X0JPA18V00",
		"cust_01KFB8PVNRZDWXAM9A31H25GR10",
		"cust_01KFBHEB0JHY5ARCVJW3PP37HT0",
		"cust_01KFDBHQ5HVYEXGK2NETSTEK3C0",
		"cust_01KFDM9MJHHB3CT1WD9MQFMN6M0",
		"cust_01KFN7QPCXQK7D4H72EDN3T3PJ0",
		"cust_01KK2C8KAR7YX49W132AF2643J",
		"cust_01KFBFWVGQ0V6813V016GGH43V0",
		"cust_01KK2C8RWX94RN2E795A2M9XWS",
		"cust_01KK2C8ZNEK2GXWSPW8F7TY35S",
		"cust_01KK2C95YVCNWGXP7J18BMGAZV",
		"cust_01KFB881XZYRR5CF5ZS7RFR68Z0",
		"cust_01KFGPZGM1CG0SSTQYQ53WEM810",
		"cust_01KK2CFDGTEVDB60QJ1GTYSQN5",
		"cust_01KFDMYEGEJX2BJ6YHKA5V3DEV0",
		"cust_01KFBQXWW27Y1S6YYCVPV0504N0",
		"cust_01KJAWDAJEJ3JQAR8N01B6ZWG3",
		"cust_01KFBGMZF3RTHAG4EJFM2DYFKE0",
		"cust_01KJAHPJP1SXNZQ7XZ2RE61KRG",
		"cust_01KFCCRM9YM8HWJVEG6ZSC1H080",
		"cust_01KJDEJ35KC37CFCP62CEYSCC4",
		"cust_01KF9VSQCF6S3CWDGHYFRNVWWE0",
		"cust_01KFBNPV0EFKDXG08ERH4D7FR00",
		"cust_01KF9J21DRQKJTNEFX8AQTPHV70",
		"cust_01KFB87WPFQMSZVA0PFE71XCX90",
		"cust_01KF9M17XSGW6YJZNBY8KWY5QW0",
		"cust_01KFB8A6K3FQNTDR087VW6BNE20",
		"cust_01KFBNPDQX4PK6ESXSCGX3SXB30",
		"cust_01KFB8JG4SCTJRB4KY9T9TKMPF0",
		"cust_01KFB8BD6AQGJFAR5YY5V3TDWC0",
		"cust_01KFB8C79G8EAK7Z2SV468F7BV0",
		"cust_01KFB8E73PY9ZAGSEHRYAK8YT10",
		"cust_01KFDKVP0KM9AAV6N7P2FHVX450",
		"cust_01KFDEZGH6JTTBMST9VX6432YN0",
		"cust_01KFB8EMSYGXPTAEAGRP0BJ58J0",
		"cust_01KFB8DPTWVPM2XQSTSZYSMQA50",
		"cust_01KFDDK40HG28C7T9Z654Z74C10",
		"cust_01KFB8CFBANKV79TQ2NWGN5ABG0",
		"cust_01KFB89EV99Z834X4EKZ03ERCF0",
		"cust_01KKHFS13QATY0K13H2G1TYG9C",
		"cust_01KFDFAKT31VR9RJES2KCVQP840",
		"cust_01KFBNR7TKKMW0Q45D6443NG550",
		"cust_01KFBNR3DS8BS7DBRHNGTB19XJ0",
		"cust_01KFB8NHG99KVEN100YYM85VCG0",
	}
	generateInvoicesMinCurrentPeriodStart = time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)

	// generateInvoicesDryRunMaxSubs caps how many subscriptions are processed when DRY_RUN=true.
	generateInvoicesDryRunMaxSubs = 10
)

func generateInvoicesIsDryRun() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("DRY_RUN")), "true")
}

// GenerateInvoices creates draft invoices with status=deleted for active subscriptions
// in a tenant/environment where CurrentPeriodStart >= generateInvoicesMinCurrentPeriodStart,
// for customers listed in generateInvoicesCustomerIDs.
// No invoice number is assigned and no credits are deducted.
// With DRY_RUN=true (-dry-run true from scripts), only the first generateInvoicesDryRunMaxSubs
// subscriptions are considered; nothing is written to Postgres; CSV includes a JSON column.
func GenerateInvoices() error {
	dryRun := generateInvoicesIsDryRun()
	tenantID := os.Getenv("TENANT_ID")
	environmentID := os.Getenv("ENVIRONMENT_ID")

	if tenantID == "" || environmentID == "" {
		return fmt.Errorf("TENANT_ID and ENVIRONMENT_ID are required")
	}

	customerIDs := make([]string, 0, len(generateInvoicesCustomerIDs))
	for _, id := range generateInvoicesCustomerIDs {
		id = strings.TrimSpace(id)
		if id != "" {
			customerIDs = append(customerIDs, id)
		}
	}
	if len(customerIDs) == 0 {
		return fmt.Errorf("set generateInvoicesCustomerIDs in scripts/internal/generate_invoices.go")
	}

	minPeriodStart := generateInvoicesMinCurrentPeriodStart

	if dryRun {
		log.Printf("DRY RUN: first %d subscriptions max, no DB writes; `-dry-run true` / DRY_RUN=true\n", generateInvoicesDryRunMaxSubs)
	}
	log.Printf("Starting invoice generation for tenant: %s, environment: %s (min_current_period_start >= %s, %d customer ids)\n",
		tenantID, environmentID, minPeriodStart.Format(time.RFC3339), len(customerIDs))

	script, err := newGenerateInvoicesScript()
	if err != nil {
		return fmt.Errorf("failed to initialize script: %w", err)
	}

	ctx := context.Background()
	ctx = context.WithValue(ctx, types.CtxTenantID, tenantID)
	ctx = context.WithValue(ctx, types.CtxEnvironmentID, environmentID)

	subscriptionFilter := &types.SubscriptionFilter{
		QueryFilter: types.NewNoLimitQueryFilter(),
	}
	subscriptionFilter.SubscriptionStatus = []types.SubscriptionStatus{
		types.SubscriptionStatusActive,
	}

	var subs []*subscription.Subscription
	seen := make(map[string]struct{})
	for _, cid := range customerIDs {
		subscriptionFilter.CustomerID = cid
		batch, listErr := script.subscriptionRepo.ListAll(ctx, subscriptionFilter)
		if listErr != nil {
			return fmt.Errorf("failed to list subscriptions for customer %s: %w", cid, listErr)
		}
		for _, s := range batch {
			if _, ok := seen[s.ID]; ok {
				continue
			}
			seen[s.ID] = struct{}{}
			subs = append(subs, s)
		}
	}
	subscriptionFilter.CustomerID = ""

	log.Printf("Found %d active subscriptions (after customer filter)\n", len(subs))

	if dryRun && len(subs) > generateInvoicesDryRunMaxSubs {
		subs = subs[:generateInvoicesDryRunMaxSubs]
		log.Printf("DRY RUN: truncated to first %d subscriptions\n", len(subs))
	}

	totalCreated := 0
	totalSkipped := 0
	totalPeriodSkipped := 0
	totalErrors := 0

	// CSV rows: invoice_id, subscription_id, customer_id, amount_due, currency, period_start, period_end
	var csvRows [][]string

	for i, sub := range subs {
		log.Printf("[%d/%d] Processing subscription %s (customer %s)\n", i+1, len(subs), sub.ID, sub.CustomerID)
		time.Sleep(200 * time.Millisecond) // Rate limiting

		if sub.CurrentPeriodStart.IsZero() || sub.CurrentPeriodEnd.IsZero() {
			log.Printf("Skipping subscription %s — missing period dates\n", sub.ID)
			totalSkipped++
			continue
		}

		if sub.CurrentPeriodStart.Before(minPeriodStart) {
			log.Printf("Skipping subscription %s — current_period_start %s before cutoff %s\n",
				sub.ID, sub.CurrentPeriodStart.Format(time.RFC3339), minPeriodStart.Format(time.RFC3339))
			totalPeriodSkipped++
			continue
		}

		// Step 1: Prepare the invoice request with line items from usage
		invReq, err := script.billingSvc.PrepareSubscriptionInvoiceRequest(
			ctx,
			sub,
			sub.CurrentPeriodStart,
			sub.CurrentPeriodEnd,
			types.ReferencePointPeriodEnd,
		)
		if err != nil {
			log.Printf("Error preparing invoice for subscription %s: %v\n", sub.ID, err)
			totalErrors++
			continue
		}

		// Step 2: Convert to domain invoice and override status to deleted
		inv, err := invReq.ToInvoice(ctx)
		if err != nil {
			log.Printf("Error converting invoice request for subscription %s: %v\n", sub.ID, err)
			totalErrors++
			continue
		}
		inv.InvoiceStatus = types.InvoiceStatusDraft
		inv.Status = types.StatusDeleted

		// Generate idempotency key using the same pattern as the existing flow, with "-temp" suffix
		periodStart := sub.CurrentPeriodStart.Truncate(time.Minute)
		periodEnd := sub.CurrentPeriodEnd.Truncate(time.Minute)
		idempGen := idempotency.NewGenerator()
		idempKey := idempGen.GenerateKey(idempotency.ScopeSubscriptionInvoice, map[string]interface{}{
			"tenant_id":       types.GetTenantID(ctx),
			"environment_id":  types.GetEnvironmentID(ctx),
			"customer_id":     sub.CustomerID,
			"subscription_id": sub.ID,
			"period_start":    &periodStart,
			"period_end":      &periodEnd,
		}) + "-temp"
		inv.IdempotencyKey = &idempKey

		baseRow := []string{
			inv.ID,
			sub.ID,
			sub.CustomerID,
			inv.AmountDue.String(),
			inv.Currency,
			sub.CurrentPeriodStart.Format(time.RFC3339),
			sub.CurrentPeriodEnd.Format(time.RFC3339),
		}

		if dryRun {
			invJSON, jerr := json.Marshal(inv)
			if jerr != nil {
				log.Printf("Warning: JSON encode invoice for subscription %s: %v\n", sub.ID, jerr)
				baseRow = append(baseRow, "")
			} else {
				baseRow = append(baseRow, string(invJSON))
			}
			log.Printf("[DRY RUN] Prepared invoice %s for subscription %s (customer: %s, amount: %s) — not persisted\n",
				inv.ID, sub.ID, sub.CustomerID, inv.AmountDue.String())
		} else {
			if err := script.invoiceRepo.CreateWithLineItems(ctx, inv); err != nil {
				log.Printf("Error creating invoice for subscription %s: %v\n", sub.ID, err)
				totalErrors++
				continue
			}
			log.Printf("Created invoice %s (status=deleted, draft) for subscription %s (customer: %s, amount: %s)\n",
				inv.ID, sub.ID, sub.CustomerID, inv.AmountDue.String())
		}
		csvRows = append(csvRows, baseRow)
		totalCreated++
	}

	modeLabel := "live"
	if dryRun {
		modeLabel = "DRY RUN"
	}
	log.Printf("Invoice generation completed (%s). Processed: %d, Created: %d, Skipped (missing dates): %d, Skipped (before cutoff): %d, Errors: %d\n",
		modeLabel, len(subs), totalCreated, totalSkipped, totalPeriodSkipped, totalErrors)

	// Write CSV dump
	if len(csvRows) > 0 {
		prefix := "generated_invoices"
		if dryRun {
			prefix = "dry_run_generated_invoices"
		}
		csvFileName := fmt.Sprintf("%s_%s.csv", prefix, time.Now().Format("2006-01-02_150405"))
		f, err := os.Create(csvFileName)
		if err != nil {
			return fmt.Errorf("failed to create CSV file: %w", err)
		}
		defer f.Close()

		w := csv.NewWriter(f)
		header := []string{"invoice_id", "subscription_id", "customer_id", "amount_due", "currency", "period_start", "period_end"}
		if dryRun {
			header = append(header, "invoice_json")
		}
		if err := w.Write(header); err != nil {
			return fmt.Errorf("failed to write CSV header: %w", err)
		}
		if err := w.WriteAll(csvRows); err != nil {
			return fmt.Errorf("failed to write CSV rows: %w", err)
		}
		w.Flush()

		if err := w.Error(); err != nil {
			return fmt.Errorf("failed to write CSV: %w", err)
		}
		log.Printf("CSV dump written to %s (%d rows)\n", csvFileName, len(csvRows))
	}

	return nil
}

func newGenerateInvoicesScript() (*generateInvoicesScript, error) {
	cfg, err := config.NewConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	appLog, err := logger.NewLogger(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create logger: %w", err)
	}

	sentryService := sentry.NewSentryService(cfg, appLog)

	chStore, err := clickhouse.NewClickHouseStore(cfg, sentryService)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to clickhouse: %w", err)
	}

	entClient, err := postgres.NewEntClients(cfg, appLog)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to postgres: %w", err)
	}
	client := postgres.NewClient(entClient, appLog, sentryService)
	cacheClient := cache.NewInMemoryCache()

	// All repositories — PrepareSubscriptionInvoiceRequest transitively uses
	// many services (subscription, addon, coupon, tax, proration, etc.) that
	// share ServiceParams, so we initialise every repo to avoid nil panics.
	customerRepo := entRepo.NewCustomerRepository(client, appLog, cacheClient)
	planRepo := entRepo.NewPlanRepository(client, appLog, cacheClient)
	subscriptionRepo := entRepo.NewSubscriptionRepository(client, appLog, cacheClient)
	subscriptionLineItemRepo := entRepo.NewSubscriptionLineItemRepository(client, appLog, cacheClient)
	subscriptionPhaseRepo := entRepo.NewSubscriptionPhaseRepository(client, appLog, cacheClient)
	priceRepo := entRepo.NewPriceRepository(client, appLog, cacheClient)
	priceUnitRepo := entRepo.NewPriceUnitRepository(client, appLog, cacheClient)
	meterRepo := entRepo.NewMeterRepository(client, appLog, cacheClient)
	invoiceRepo := entRepo.NewInvoiceRepository(client, appLog, cacheClient)
	invoiceLineItemRepo := entRepo.NewInvoiceLineItemRepository(client, appLog, cacheClient)
	featureRepo := entRepo.NewFeatureRepository(client, appLog, cacheClient)
	entitlementRepo := entRepo.NewEntitlementRepository(client, appLog, cacheClient)
	walletRepo := entRepo.NewWalletRepository(client, appLog, cacheClient)
	creditGrantRepo := entRepo.NewCreditGrantRepository(client, appLog, cacheClient)
	creditGrantAppRepo := entRepo.NewCreditGrantApplicationRepository(client, appLog, cacheClient)
	couponRepo := entRepo.NewCouponRepository(client, appLog, cacheClient)
	couponAssocRepo := entRepo.NewCouponAssociationRepository(client, appLog, cacheClient)
	couponAppRepo := entRepo.NewCouponApplicationRepository(client, appLog, cacheClient)
	taxRateRepo := entRepo.NewTaxRateRepository(client, appLog, cacheClient)
	taxAssocRepo := entRepo.NewTaxAssociationRepository(client, appLog, cacheClient)
	taxAppliedRepo := entRepo.NewTaxAppliedRepository(client, appLog, cacheClient)
	addonRepo := entRepo.NewAddonRepository(client, appLog, cacheClient)
	addonAssocRepo := entRepo.NewAddonAssociationRepository(client, appLog, cacheClient)
	settingsRepo := entRepo.NewSettingsRepository(client, appLog, cacheClient)
	eventRepo := chRepo.NewEventRepository(chStore, appLog)
	processedEventRepo := chRepo.NewProcessedEventRepository(chStore, appLog)
	featureUsageRepo := chRepo.NewFeatureUsageRepository(chStore, appLog)
	costSheetUsageRepo := chRepo.NewCostSheetUsageRepository(chStore, appLog)

	serviceParams := service.ServiceParams{
		Logger:                     appLog,
		Config:                     cfg,
		DB:                         client,
		CustomerRepo:               customerRepo,
		PlanRepo:                   planRepo,
		SubRepo:                    subscriptionRepo,
		SubscriptionLineItemRepo:   subscriptionLineItemRepo,
		SubscriptionPhaseRepo:      subscriptionPhaseRepo,
		PriceRepo:                  priceRepo,
		PriceUnitRepo:              priceUnitRepo,
		MeterRepo:                  meterRepo,
		InvoiceRepo:                invoiceRepo,
		InvoiceLineItemRepo:        invoiceLineItemRepo,
		FeatureRepo:                featureRepo,
		EntitlementRepo:            entitlementRepo,
		WalletRepo:                 walletRepo,
		CreditGrantRepo:            creditGrantRepo,
		CreditGrantApplicationRepo: creditGrantAppRepo,
		CouponRepo:                 couponRepo,
		CouponAssociationRepo:      couponAssocRepo,
		CouponApplicationRepo:      couponAppRepo,
		TaxRateRepo:                taxRateRepo,
		TaxAssociationRepo:         taxAssocRepo,
		TaxAppliedRepo:             taxAppliedRepo,
		AddonRepo:                  addonRepo,
		AddonAssociationRepo:       addonAssocRepo,
		SettingsRepo:               settingsRepo,
		EventRepo:                  eventRepo,
		ProcessedEventRepo:         processedEventRepo,
		FeatureUsageRepo:           featureUsageRepo,
		CostSheetUsageRepo:         costSheetUsageRepo,
	}

	billingSvc := service.NewBillingService(serviceParams)

	return &generateInvoicesScript{
		log:              appLog,
		subscriptionRepo: subscriptionRepo,
		billingSvc:       billingSvc,
		invoiceRepo:      invoiceRepo,
	}, nil
}
