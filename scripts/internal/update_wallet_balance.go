package internal

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/flexprice/flexprice/internal/cache"
	"github.com/flexprice/flexprice/internal/config"
	"github.com/flexprice/flexprice/internal/domain/customer"
	"github.com/flexprice/flexprice/internal/domain/wallet"
	"github.com/flexprice/flexprice/internal/logger"
	"github.com/flexprice/flexprice/internal/postgres"
	entRepo "github.com/flexprice/flexprice/internal/repository/ent"
	"github.com/flexprice/flexprice/internal/sentry"
	"github.com/flexprice/flexprice/internal/types"
	"github.com/shopspring/decimal"
	"github.com/xuri/excelize/v2"
)

// UpdateWalletBalanceFromExcel reads customer IDs from an Excel file and updates their wallet balances
// Required environment variables:
//   - TENANT_ID: The tenant ID
//   - ENVIRONMENT_ID: The environment ID
//   - EXCEL_FILE_PATH: Path to the Excel file (optional, defaults to temp/R1.stage Negative balance sheet.xlsx)
//   - DRY_RUN: Set to "true" to preview changes without applying them
func UpdateWalletBalanceFromExcel() error {
	// Get environment variables
	tenantID := os.Getenv("TENANT_ID")
	environmentID := os.Getenv("ENVIRONMENT_ID")
	excelFilePath := os.Getenv("EXCEL_FILE_PATH")
	dryRun := os.Getenv("DRY_RUN") == "true"

	if tenantID == "" || environmentID == "" {
		return fmt.Errorf("TENANT_ID and ENVIRONMENT_ID are required")
	}

	// Default Excel file path
	if excelFilePath == "" {
		excelFilePath = "temp/balance_sheet	.xlsx"
	}

	log.Printf("Starting wallet balance update from Excel\n")
	log.Printf("  Tenant ID: %s\n", tenantID)
	log.Printf("  Environment ID: %s\n", environmentID)
	log.Printf("  Excel File: %s\n", excelFilePath)
	log.Printf("  Dry Run: %v\n", dryRun)

	// Initialize script
	script, err := newWalletUpdateScript()
	if err != nil {
		return fmt.Errorf("failed to initialize script: %w", err)
	}

	ctx := context.Background()
	ctx = context.WithValue(ctx, types.CtxTenantID, tenantID)
	ctx = context.WithValue(ctx, types.CtxEnvironmentID, environmentID)

	// Read Excel file
	records, err := readExcelFile(excelFilePath)
	if err != nil {
		return fmt.Errorf("failed to read Excel file: %w", err)
	}

	log.Printf("Found %d records in Excel file\n", len(records))

	// Process each record
	var successCount, errorCount, skippedCount int
	for i, record := range records {
		err := script.processRecord(ctx, record, dryRun)
		if err != nil {
			log.Printf("[%d] ERROR processing %s: %v\n", i+1, record.ExternalCustomerID, err)
			errorCount++
		} else if record.Skipped {
			log.Printf("[%d] SKIPPED %s: %s\n", i+1, record.ExternalCustomerID, record.SkipReason)
			skippedCount++
		} else {
			successCount++
		}
	}

	log.Printf("\n=== Summary ===\n")
	log.Printf("Total records: %d\n", len(records))
	log.Printf("Success: %d\n", successCount)
	log.Printf("Skipped: %d\n", skippedCount)
	log.Printf("Errors: %d\n", errorCount)

	if dryRun {
		log.Printf("\n*** DRY RUN - No changes were made ***\n")
	}

	return nil
}

// ExcelRecord represents a row from the Excel file
type ExcelRecord struct {
	ExternalCustomerID string
	NewBalance         decimal.Decimal
	Skipped            bool
	SkipReason         string
}

func readExcelFile(filePath string) ([]ExcelRecord, error) {
	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open Excel file: %w", err)
	}
	defer f.Close()

	// Get the first sheet name
	sheetName := f.GetSheetName(0)
	if sheetName == "" {
		return nil, fmt.Errorf("no sheets found in Excel file")
	}

	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("failed to get rows: %w", err)
	}

	if len(rows) < 2 {
		return nil, fmt.Errorf("Excel file has no data rows (only header or empty)")
	}

	// Parse header to find column indices
	header := rows[0]
	customerIDCol := 0
	balance := -1

	for i, col := range header {
		colLower := strings.ToLower(strings.TrimSpace(col))
		switch {
		case strings.Contains(colLower, "account_id") || strings.Contains(colLower, "account id") || strings.Contains(colLower, "external_id"):
			customerIDCol = i
		case strings.Contains(colLower, "original_balance"):
			balance = i
		}
	}

	if customerIDCol == -1 {
		return nil, fmt.Errorf("could not find account_id column in Excel header: %v", header)
	}

	log.Printf("Column mapping: account_id=%d, balance=%d\n", customerIDCol, balance)

	var records []ExcelRecord
	for i, row := range rows[1:] {
		if len(row) <= customerIDCol {
			log.Printf("Row %d: skipping - not enough columns\n", i+2)
			continue
		}

		externalID := strings.TrimSpace(row[customerIDCol])
		if externalID == "" {
			continue
		}

		record := ExcelRecord{
			ExternalCustomerID: externalID,
			NewBalance:         decimal.NewFromFloat(0),
		}

		if balance != -1 && len(row) > balance && row[balance] != "" {
			balanceValue, err := decimal.NewFromString(strings.TrimSpace(row[balance]))
			if err != nil {
				log.Printf("Row %d: invalid balance value '%s', using 0\n", i+2, row[balance])
			} else {
				record.NewBalance = balanceValue
			}
		}

		records = append(records, record)
	}

	return records, nil
}

type walletUpdateScript struct {
	log          *logger.Logger
	customerRepo customer.Repository
	walletRepo   wallet.Repository
}

func newWalletUpdateScript() (*walletUpdateScript, error) {
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

	// Initialize postgres client
	sentryService := sentry.NewSentryService(cfg, log)
	entClient, err := postgres.NewEntClients(cfg, log)
	if err != nil {
		log.Fatalf("Failed to connect to postgres: %v", err)
	}
	client := postgres.NewClient(entClient, log, sentryService)
	cacheClient := cache.NewInMemoryCache()

	// Create repositories
	customerRepo := entRepo.NewCustomerRepository(client, log, cacheClient)
	walletRepo := entRepo.NewWalletRepository(client, log, cacheClient)

	return &walletUpdateScript{
		log:          log,
		customerRepo: customerRepo,
		walletRepo:   walletRepo,
	}, nil
}

func (s *walletUpdateScript) processRecord(ctx context.Context, record ExcelRecord, dryRun bool) error {
	// Look up customer by external ID (lookup key)
	cust, err := s.customerRepo.GetByLookupKey(ctx, record.ExternalCustomerID)
	if err != nil {
		return fmt.Errorf("customer not found: %w", err)
	}

	// Get customer's wallets
	wallets, err := s.walletRepo.GetWalletsByCustomerID(ctx, cust.ID)
	if err != nil {
		return fmt.Errorf("failed to get wallets: %w", err)
	}

	if len(wallets) == 0 {
		record.Skipped = true
		record.SkipReason = "no wallets found"
		return nil
	}

	// Update each wallet
	for _, w := range wallets {
		log.Printf("  Wallet %s: current balance=%s, credit_balance=%s -> new balance=%s, credit_balance=%s\n",
			w.ID, w.Balance.String(), w.CreditBalance.String(),
			record.NewBalance.String(), record.NewBalance.String())

		if !dryRun {
			err := s.walletRepo.UpdateWalletBalance(ctx, w.ID, record.NewBalance, record.NewBalance)
			if err != nil {
				return fmt.Errorf("failed to update wallet %s: %w", w.ID, err)
			}
			log.Printf("  Updated wallet %s successfully\n", w.ID)
		}
	}

	return nil
}
