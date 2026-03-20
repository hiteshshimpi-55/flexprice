package internal

import (
	"context"
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/flexprice/flexprice/internal/api/dto"
	"github.com/flexprice/flexprice/internal/cache"
	"github.com/flexprice/flexprice/internal/clickhouse"
	"github.com/flexprice/flexprice/internal/config"
	"github.com/flexprice/flexprice/internal/domain/customer"
	"github.com/flexprice/flexprice/internal/domain/wallet"
	"github.com/flexprice/flexprice/internal/logger"
	"github.com/flexprice/flexprice/internal/postgres"
	chRepo "github.com/flexprice/flexprice/internal/repository/clickhouse"
	entRepo "github.com/flexprice/flexprice/internal/repository/ent"
	"github.com/flexprice/flexprice/internal/sentry"
	"github.com/flexprice/flexprice/internal/service"
	"github.com/flexprice/flexprice/internal/types"
	"github.com/shopspring/decimal"
)

const defaultMarchUsageEnd = "2026-03-21T00:00:00Z"

type reconInputRow struct {
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

type reconOutputRow struct {
	reconInputRow
	ExternalCustomerID         string
	BillingPeriodUsageCredits  string
	PartialMarchUsageCredits   string
	OngoingWalletBalance       string
	CreditBalance              string
	FinalOngoingBalanceCredits string
}

type reconCSVEnrichScript struct {
	log                         *logger.Logger
	customerRepo                customer.Repository
	walletRepo                  wallet.Repository
	featureUsageTrackingService service.FeatureUsageTrackingService
	walletService               service.WalletService
}

// RunReconCSVEnrich enriches a reconciliation CSV with customer IDs, usage, and wallet balances.
func RunReconCSVEnrich() error {
	tenantID := os.Getenv("TENANT_ID")
	environmentID := os.Getenv("ENVIRONMENT_ID")
	inputCSV := os.Getenv("INPUT_CSV")
	outputCSV := os.Getenv("OUTPUT_CSV")
	workerCountStr := os.Getenv("WORKER_COUNT")
	marchUsageEndStr := os.Getenv("RECON_MARCH_USAGE_END")

	if tenantID == "" || environmentID == "" {
		return fmt.Errorf("TENANT_ID and ENVIRONMENT_ID are required")
	}

	if inputCSV == "" {
		inputCSV = "missing_invoices/ola-prod-recon.csv"
	}
	if outputCSV == "" {
		outputCSV = defaultOutputPath(inputCSV)
	}
	if marchUsageEndStr == "" {
		marchUsageEndStr = defaultMarchUsageEnd
	}

	workerCount := 10
	if workerCountStr != "" {
		n, err := strconv.Atoi(workerCountStr)
		if err != nil || n <= 0 {
			return fmt.Errorf("invalid WORKER_COUNT: %q", workerCountStr)
		}
		workerCount = n
	}

	marchUsageEnd, err := time.Parse(time.RFC3339, marchUsageEndStr)
	if err != nil {
		return fmt.Errorf("invalid RECON_MARCH_USAGE_END %q: %w", marchUsageEndStr, err)
	}

	log.Printf("Starting recon CSV enrich tenant=%s env=%s input=%s output=%s workers=%d march_end=%s\n",
		tenantID, environmentID, inputCSV, outputCSV, workerCount, marchUsageEnd.UTC().Format(time.RFC3339))

	script, err := newReconCSVEnrichScript()
	if err != nil {
		return fmt.Errorf("failed to initialize script: %w", err)
	}

	ctx := context.Background()
	ctx = context.WithValue(ctx, types.CtxTenantID, tenantID)
	ctx = context.WithValue(ctx, types.CtxEnvironmentID, environmentID)

	rows, err := readReconInputCSV(inputCSV)
	if err != nil {
		return err
	}

	filteredRows := make([]reconInputRow, 0, len(rows))
	customerIDs := make(map[string]struct{})
	for _, row := range rows {
		diff, err := decimal.NewFromString(strings.TrimSpace(row.Diff))
		if err != nil {
			script.log.Warnw("skipping row due to invalid diff", "customer_id", row.CustomerID, "subscription_id", row.SubscriptionID, "diff", row.Diff, "error", err)
			continue
		}
		if diff.IsZero() {
			continue
		}
		filteredRows = append(filteredRows, row)
		customerIDs[row.CustomerID] = struct{}{}
	}

	log.Printf("Loaded %d rows, filtered to %d rows with non-zero diff\n", len(rows), len(filteredRows))

	uniqueCustomerIDs := make([]string, 0, len(customerIDs))
	for customerID := range customerIDs {
		uniqueCustomerIDs = append(uniqueCustomerIDs, customerID)
	}

	customerMap, err := script.buildCustomerMap(ctx, uniqueCustomerIDs)
	if err != nil {
		return fmt.Errorf("failed to build customer map: %w", err)
	}

	results := make([]reconOutputRow, len(filteredRows))
	var (
		wg        sync.WaitGroup
		semaphore = make(chan struct{}, workerCount)
	)

	for i, row := range filteredRows {
		wg.Add(1)
		semaphore <- struct{}{}
		go func(index int, in reconInputRow) {
			defer wg.Done()
			defer func() { <-semaphore }()
			results[index] = script.enrichRow(ctx, in, customerMap, marchUsageEnd)
		}(i, row)
	}
	wg.Wait()

	if err := writeReconOutputCSV(results, outputCSV); err != nil {
		return err
	}

	log.Printf("Recon CSV enrichment completed: %s (rows=%d)\n", outputCSV, len(results))
	return nil
}

func (s *reconCSVEnrichScript) enrichRow(
	ctx context.Context,
	row reconInputRow,
	customerMap map[string]string,
	marchUsageEnd time.Time,
) reconOutputRow {
	output := reconOutputRow{
		reconInputRow:        row,
		ExternalCustomerID:   customerMap[row.CustomerID],
		OngoingWalletBalance: decimal.Zero.StringFixed(2),
		CreditBalance:        decimal.Zero.StringFixed(2),
	}

	periodStart, err := time.Parse(time.RFC3339, row.PeriodStart)
	if err != nil {
		s.log.Warnw("failed to parse period_start", "customer_id", row.CustomerID, "subscription_id", row.SubscriptionID, "period_start", row.PeriodStart, "error", err)
		return output
	}

	periodEnd, err := time.Parse(time.RFC3339, row.PeriodEnd)
	if err != nil {
		s.log.Warnw("failed to parse period_end", "customer_id", row.CustomerID, "subscription_id", row.SubscriptionID, "period_end", row.PeriodEnd, "error", err)
		return output
	}

	billingPeriodUsage := decimal.Zero
	partialMarchUsage := decimal.Zero

	if output.ExternalCustomerID != "" {
		billingPeriodUsage = s.fetchUsageCredits(ctx, output.ExternalCustomerID, row.SubscriptionID, periodStart, periodEnd)
		if periodEnd.Before(marchUsageEnd) {
			partialMarchUsage = s.fetchUsageCredits(ctx, output.ExternalCustomerID, row.SubscriptionID, periodEnd, marchUsageEnd)
		}
	} else {
		s.log.Warnw("external_customer_id missing; usage defaults to zero",
			"customer_id", row.CustomerID,
			"subscription_id", row.SubscriptionID)
	}

	ongoingWalletBalance, creditBalance := s.fetchWalletBalances(ctx, row.CustomerID)
	effectiveCreditBalance := creditBalance
	if strings.EqualFold(strings.TrimSpace(row.InvoiceGenerated), "true") {
		invoiceSubtotal := decimal.Zero
		if strings.TrimSpace(row.InvoiceSubtotal) != "" {
			parsedSubtotal, parseErr := decimal.NewFromString(strings.TrimSpace(row.InvoiceSubtotal))
			if parseErr != nil {
				s.log.Warnw("failed to parse invoice_subtotal, treating as zero",
					"customer_id", row.CustomerID,
					"subscription_id", row.SubscriptionID,
					"invoice_subtotal", row.InvoiceSubtotal,
					"error", parseErr)
			} else {
				invoiceSubtotal = parsedSubtotal
			}
		}
		effectiveCreditBalance = effectiveCreditBalance.Add(invoiceSubtotal)
	}
	finalOngoingBalance := effectiveCreditBalance.Sub(billingPeriodUsage).Sub(partialMarchUsage)

	output.BillingPeriodUsageCredits = billingPeriodUsage.StringFixed(2)
	output.PartialMarchUsageCredits = partialMarchUsage.StringFixed(2)
	output.OngoingWalletBalance = ongoingWalletBalance.StringFixed(2)
	output.CreditBalance = creditBalance.StringFixed(2)
	output.FinalOngoingBalanceCredits = finalOngoingBalance.StringFixed(2)
	return output
}

func (s *reconCSVEnrichScript) fetchUsageCredits(
	ctx context.Context,
	externalCustomerID string,
	subscriptionID string,
	start time.Time,
	end time.Time,
) decimal.Decimal {
	resp, err := s.featureUsageTrackingService.GetDetailedUsageAnalytics(ctx, &dto.GetUsageAnalyticsRequest{
		ExternalCustomerID: externalCustomerID,
		SubscriptionID:     subscriptionID,
		StartTime:          start,
		EndTime:            end,
	})
	if err != nil {
		s.log.Warnw("failed to fetch usage analytics",
			"external_customer_id", externalCustomerID,
			"subscription_id", subscriptionID,
			"start", start,
			"end", end,
			"error", err)
		return decimal.Zero
	}
	if resp == nil {
		return decimal.Zero
	}
	return resp.TotalCost
}

func (s *reconCSVEnrichScript) fetchWalletBalances(ctx context.Context, customerID string) (decimal.Decimal, decimal.Decimal) {
	wallets, err := s.walletRepo.GetWalletsByCustomerID(ctx, customerID)
	if err != nil {
		s.log.Warnw("failed to list wallets", "customer_id", customerID, "error", err)
		return decimal.Zero, decimal.Zero
	}

	ongoingBalance := decimal.Zero
	creditBalance := decimal.Zero

	for _, w := range wallets {
		balanceResp, err := s.walletService.GetWalletBalance(ctx, w.ID)
		if err != nil {
			s.log.Warnw("failed to fetch wallet balance", "customer_id", customerID, "wallet_id", w.ID, "error", err)
			continue
		}

		if balanceResp.RealTimeCreditBalance != nil {
			ongoingBalance = ongoingBalance.Add(*balanceResp.RealTimeCreditBalance)
		}
		if balanceResp.Wallet != nil {
			creditBalance = creditBalance.Add(balanceResp.Wallet.CreditBalance)
		} else {
			creditBalance = creditBalance.Add(w.CreditBalance)
		}
	}

	return ongoingBalance, creditBalance
}

func (s *reconCSVEnrichScript) buildCustomerMap(ctx context.Context, customerIDs []string) (map[string]string, error) {
	result := make(map[string]string, len(customerIDs))
	if len(customerIDs) == 0 {
		return result, nil
	}

	batchSize := 100
	for i := 0; i < len(customerIDs); i += batchSize {
		end := i + batchSize
		if end > len(customerIDs) {
			end = len(customerIDs)
		}

		filter := types.NewNoLimitCustomerFilter()
		filter.CustomerIDs = customerIDs[i:end]

		customers, err := s.customerRepo.List(ctx, filter)
		if err != nil {
			return nil, fmt.Errorf("failed to list customers (%d-%d): %w", i, end, err)
		}

		for _, c := range customers {
			result[c.ID] = c.ExternalID
		}
	}
	return result, nil
}

func readReconInputCSV(filePath string) ([]reconInputRow, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open input CSV: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read input CSV: %w", err)
	}
	if len(records) < 2 {
		return []reconInputRow{}, nil
	}

	headerIndex := make(map[string]int, len(records[0]))
	for idx, h := range records[0] {
		headerIndex[strings.TrimSpace(h)] = idx
	}

	required := []string{
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
	for _, col := range required {
		if _, ok := headerIndex[col]; !ok {
			return nil, fmt.Errorf("missing required column in input CSV: %s", col)
		}
	}

	rows := make([]reconInputRow, 0, len(records)-1)
	for _, record := range records[1:] {
		get := func(col string) string {
			idx := headerIndex[col]
			if idx >= len(record) {
				return ""
			}
			return strings.TrimSpace(record[idx])
		}
		rows = append(rows, reconInputRow{
			CustomerID:       get("customer_id"),
			SubscriptionID:   get("subscription_id"),
			InvoiceID:        get("invoice_id"),
			PeriodStart:      get("period_start"),
			PeriodEnd:        get("period_end"),
			AnalyticsAmount:  get("analytics_amount"),
			InvoiceSubtotal:  get("invoice_subtotal"),
			Diff:             get("diff"),
			InvoiceGenerated: get("invoice_generated"),
		})
	}

	return rows, nil
}

func writeReconOutputCSV(rows []reconOutputRow, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create output CSV: %w", err)
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
		"external_customer_id",
		"billing_period_usage_credits",
		"partial_march_usage_credits",
		"ongoing_realtime_credit_balance",
		"credit_balance",
		"final_ongoing_balance",
	}
	if err := writer.Write(header); err != nil {
		return fmt.Errorf("failed to write output header: %w", err)
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
			row.ExternalCustomerID,
			row.BillingPeriodUsageCredits,
			row.PartialMarchUsageCredits,
			row.OngoingWalletBalance,
			row.CreditBalance,
			row.FinalOngoingBalanceCredits,
		}
		if err := writer.Write(record); err != nil {
			return fmt.Errorf("failed to write output row: %w", err)
		}
	}
	return nil
}

func defaultOutputPath(inputPath string) string {
	ext := filepath.Ext(inputPath)
	base := strings.TrimSuffix(inputPath, ext)
	if ext == "" {
		return base + "-enriched.csv"
	}
	return base + "-enriched" + ext
}

func newReconCSVEnrichScript() (*reconCSVEnrichScript, error) {
	cfg, err := config.NewConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	logg, err := logger.NewLogger(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create logger: %w", err)
	}

	sentryService := sentry.NewSentryService(cfg, logg)
	chStore, err := clickhouse.NewClickHouseStore(cfg, sentryService)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to clickhouse: %w", err)
	}

	entClient, err := postgres.NewEntClients(cfg, logg)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to postgres: %w", err)
	}
	client := postgres.NewClient(entClient, logg, sentryService)
	cacheClient := cache.NewInMemoryCache()

	customerRepo := entRepo.NewCustomerRepository(client, logg, cacheClient)
	walletRepo := entRepo.NewWalletRepository(client, logg, cacheClient)
	subscriptionRepo := entRepo.NewSubscriptionRepository(client, logg, cacheClient)
	subscriptionLineItemRepo := entRepo.NewSubscriptionLineItemRepository(client, logg, cacheClient)
	subscriptionPhaseRepo := entRepo.NewSubscriptionPhaseRepository(client, logg, cacheClient)
	planRepo := entRepo.NewPlanRepository(client, logg, cacheClient)
	priceRepo := entRepo.NewPriceRepository(client, logg, cacheClient)
	meterRepo := entRepo.NewMeterRepository(client, logg, cacheClient)
	featureRepo := entRepo.NewFeatureRepository(client, logg, cacheClient)
	entitlementRepo := entRepo.NewEntitlementRepository(client, logg, cacheClient)
	addonRepo := entRepo.NewAddonRepository(client, logg, cacheClient)
	addonAssociationRepo := entRepo.NewAddonAssociationRepository(client, logg, cacheClient)
	invoiceRepo := entRepo.NewInvoiceRepository(client, logg, cacheClient)
	settingsRepo := entRepo.NewSettingsRepository(client, logg, cacheClient)
	eventRepo := chRepo.NewEventRepository(chStore, logg)
	processedEventRepo := chRepo.NewProcessedEventRepository(chStore, logg)
	featureUsageRepo := chRepo.NewFeatureUsageRepository(chStore, logg)

	serviceParams := service.ServiceParams{
		Logger:                   logg,
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
		SettingsRepo:             settingsRepo,
		EventRepo:                eventRepo,
		ProcessedEventRepo:       processedEventRepo,
		FeatureUsageRepo:         featureUsageRepo,
	}

	return &reconCSVEnrichScript{
		log:                         logg,
		customerRepo:                customerRepo,
		walletRepo:                  walletRepo,
		featureUsageTrackingService: service.NewFeatureUsageTrackingService(serviceParams, eventRepo, featureUsageRepo),
		walletService:               service.NewWalletService(serviceParams),
	}, nil
}
