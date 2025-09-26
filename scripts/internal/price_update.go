package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/flexprice/flexprice/internal/cache"
	"github.com/flexprice/flexprice/internal/config"
	"github.com/flexprice/flexprice/internal/domain/price"
	"github.com/flexprice/flexprice/internal/logger"
	"github.com/flexprice/flexprice/internal/postgres"
	entRepo "github.com/flexprice/flexprice/internal/repository/ent"
	"github.com/flexprice/flexprice/internal/sentry"
	"github.com/flexprice/flexprice/internal/types"
	"github.com/samber/lo"
	"github.com/shopspring/decimal"
)

type priceUpdateScript struct {
	log       *logger.Logger
	priceRepo price.Repository
}

// PricePreview represents a price object for preview in dry run mode
type PricePreview struct {
	ID                    string            `json:"id"`
	OriginalAmount        string            `json:"original_amount"`
	NewAmount             string            `json:"new_amount"`
	OriginalDisplayAmount string            `json:"original_display_amount"`
	NewDisplayAmount      string            `json:"new_display_amount"`
	Currency              string            `json:"currency"`
	Type                  string            `json:"type"`
	BillingPeriod         string            `json:"billing_period"`
	BillingPeriodCount    int               `json:"billing_period_count"`
	BillingModel          string            `json:"billing_model"`
	BillingCadence        string            `json:"billing_cadence"`
	LookupKey             string            `json:"lookup_key"`
	Description           string            `json:"description"`
	Metadata              map[string]string `json:"metadata"`
	CreatedAt             time.Time         `json:"created_at"`
	UpdatedAt             time.Time         `json:"updated_at"`
}

// DryRunPreview represents the complete preview data for dry run
type DryRunPreview struct {
	PlanID        string         `json:"plan_id"`
	TenantID      string         `json:"tenant_id"`
	EnvironmentID string         `json:"environment_id"`
	DryRun        bool           `json:"dry_run"`
	TotalPrices   int            `json:"total_prices"`
	Prices        []PricePreview `json:"prices"`
	GeneratedAt   time.Time      `json:"generated_at"`
}

// UpdatePlanPrices updates all prices for a given plan by dividing their amounts by 10^6.
// This function supports dry run mode to preview changes before execution.
//
// Required environment variables:
//   - PLAN_ID: The ID of the plan whose prices need to be updated
//   - TENANT_ID: The tenant ID
//   - ENVIRONMENT_ID: The environment ID
//   - USER_ID: The user ID (optional, defaults to system user)
//
// Optional environment variables:
//   - DRY_RUN: Set to "true" to preview changes without executing them (default: "false")
//
// Dry Run Features:
//   - When DRY_RUN=true, the function generates a detailed JSON preview file
//   - The JSON file contains all price objects with original and new values
//   - Preview files are saved to the temp/ directory with timestamp
//   - File naming format: price_update_preview_{plan_id}_{timestamp}.json
//
// Usage examples:
//
//	# Dry run to preview changes (generates JSON preview file)
//	go run scripts/main.go -cmd=update-plan-prices -plan-id=plan_123 -tenant-id=tenant_456 -environment-id=env_789 -dry-run=true
//
//	# Execute the actual updates
//	go run scripts/main.go -cmd=update-plan-prices -plan-id=plan_123 -tenant-id=tenant_456 -environment-id=env_789 -dry-run=false
//
//	# Using environment variables (note: command line flags override env vars)
//	DRY_RUN=true PLAN_ID=plan_123 TENANT_ID=tenant_456 ENVIRONMENT_ID=env_789 go run scripts/main.go -cmd=update-plan-prices
func UpdatePlanPrices() error {
	// Get plan ID from environment
	planID := os.Getenv("PLAN_ID")
	if planID == "" {
		return fmt.Errorf("PLAN_ID environment variable is required")
	}

	// Get dry run flag from environment
	dryRunStr := os.Getenv("DRY_RUN")
	dryRun := false
	if dryRunStr != "" {
		var err error
		dryRun, err = strconv.ParseBool(dryRunStr)
		if err != nil {
			return fmt.Errorf("invalid DRY_RUN value: %w", err)
		}
	}

	// Initialize script
	script, err := newPriceUpdateScript()
	if err != nil {
		return fmt.Errorf("failed to initialize script: %w", err)
	}

	// Get tenant and environment IDs
	tenantID := os.Getenv("TENANT_ID")
	environmentID := os.Getenv("ENVIRONMENT_ID")

	// Create context with tenant and environment info
	ctx := context.Background()
	ctx = context.WithValue(ctx, types.CtxTenantID, tenantID)
	ctx = context.WithValue(ctx, types.CtxEnvironmentID, environmentID)
	ctx = context.WithValue(ctx, types.CtxUserID, os.Getenv("USER_ID"))

	// Execute the price update
	return script.updatePlanPrices(ctx, planID, tenantID, environmentID, dryRun)
}

func newPriceUpdateScript() (*priceUpdateScript, error) {
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
	entClient, err := postgres.NewEntClient(cfg, log)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to postgres: %w", err)
	}
	client := postgres.NewClient(entClient, log, sentry.NewSentryService(cfg, log))
	cacheClient := cache.NewInMemoryCache()

	// Initialize price repository
	priceRepo := entRepo.NewPriceRepository(client, log, cacheClient)

	return &priceUpdateScript{
		log:       log,
		priceRepo: priceRepo,
	}, nil
}

func (s *priceUpdateScript) updatePlanPrices(ctx context.Context, planID, tenantID, environmentID string, dryRun bool) error {
	s.log.Infow("Starting price update", "plan_id", planID, "dry_run", dryRun)

	// Fetch prices for the plan
	prices, err := s.priceRepo.GetByPlanID(ctx, planID)
	if err != nil {
		return fmt.Errorf("failed to fetch prices for plan %s: %w", planID, err)
	}

	if len(prices) == 0 {
		s.log.Infow("No prices found for plan", "plan_id", planID)
		return nil
	}

	s.log.Infow("Found prices to update", "count", len(prices), "plan_id", planID)

	// Process each price
	updatedCount := 0
	var pricePreviews []PricePreview

	excludePriceIDs := []string{
		"price_01K5V2V9E4259WB27RCJQ3JHED",
		"price_01K5V2V7X6TRS37J8RYD71QQM1",
		"price_01K5V2V36VVRMZY8A3JC0ZDGMS",
		"price_01K5V2V6XMPJPGVGGFKPAA7N83",
		"price_01K5V2V572NYH8D58NMV2ZARYW",
	}
	for _, price := range prices {

		if lo.Contains(excludePriceIDs, price.ID) {
			continue
		}

		// Store original values for logging
		originalAmount := price.Amount
		originalDisplayAmount := price.DisplayAmount

		// Divide amount by 10^6
		divisor := decimal.NewFromInt(1000000) // 10^6
		newAmount := price.Amount.Div(divisor)

		// Update the price object
		price.Amount = newAmount

		// Update display amount to reflect the new amount
		// Extract currency symbol from original display amount
		currencySymbol := s.extractCurrencySymbol(originalDisplayAmount)
		price.DisplayAmount = fmt.Sprintf("%s%s", currencySymbol, newAmount.String())

		// Log the change
		s.log.Infow("Price update",
			"price_id", price.ID,
			"original_amount", originalAmount.String(),
			"new_amount", newAmount.String(),
			"original_display", originalDisplayAmount,
			"new_display", price.DisplayAmount,
		)

		// Create price preview for dry run
		pricePreview := PricePreview{
			ID:                    price.ID,
			OriginalAmount:        originalAmount.String(),
			NewAmount:             newAmount.String(),
			OriginalDisplayAmount: originalDisplayAmount,
			NewDisplayAmount:      price.DisplayAmount,
			Currency:              price.Currency,
			Type:                  string(price.Type),
			BillingPeriod:         string(price.BillingPeriod),
			BillingPeriodCount:    price.BillingPeriodCount,
			BillingModel:          string(price.BillingModel),
			BillingCadence:        string(price.BillingCadence),
			LookupKey:             price.LookupKey,
			Description:           price.Description,
			Metadata:              map[string]string(price.Metadata),
			CreatedAt:             price.CreatedAt,
			UpdatedAt:             price.UpdatedAt,
		}
		pricePreviews = append(pricePreviews, pricePreview)
		if dryRun {
			s.log.Infow("DRY RUN: Would update price", "price_id", price.ID)
		} else {
			// Update the price in the database
			if err := s.priceRepo.Update(ctx, price); err != nil {
				s.log.Errorw("Failed to update price", "price_id", price.ID, "error", err)
				return fmt.Errorf("failed to update price %s: %w", price.ID, err)
			}
			s.log.Infow("Successfully updated price", "price_id", price.ID)
		}

		updatedCount++
	}

	if dryRun {
		// Generate JSON preview file
		if err := s.generateDryRunPreview(planID, tenantID, environmentID, pricePreviews); err != nil {
			s.log.Errorw("Failed to generate dry run preview", "error", err)
			return fmt.Errorf("failed to generate dry run preview: %w", err)
		}

		s.log.Infow("DRY RUN COMPLETE", "prices_previewed", updatedCount, "plan_id", planID)
		fmt.Printf("\n=== DRY RUN SUMMARY ===\n")
		fmt.Printf("Plan ID: %s\n", planID)
		fmt.Printf("Prices to be updated: %d\n", updatedCount)
		fmt.Printf("All prices will have their amounts divided by 1,000,000\n")
		fmt.Printf("Preview JSON file generated successfully\n")
		fmt.Printf("To execute the changes, run the command with DRY_RUN=false\n")
		fmt.Printf("========================\n\n")
	} else {
		s.log.Infow("Price update completed", "prices_updated", updatedCount, "plan_id", planID)
		fmt.Printf("\n=== UPDATE COMPLETE ===\n")
		fmt.Printf("Plan ID: %s\n", planID)
		fmt.Printf("Prices updated: %d\n", updatedCount)
		fmt.Printf("All prices have been divided by 1,000,000\n")
		fmt.Printf("======================\n\n")
	}

	return nil
}

// generateDryRunPreview creates a JSON file with the preview of all price changes
func (s *priceUpdateScript) generateDryRunPreview(planID, tenantID, environmentID string, pricePreviews []PricePreview) error {
	// Create the preview data structure
	preview := DryRunPreview{
		PlanID:        planID,
		TenantID:      tenantID,
		EnvironmentID: environmentID,
		DryRun:        true,
		TotalPrices:   len(pricePreviews),
		Prices:        pricePreviews,
		GeneratedAt:   time.Now().UTC(),
	}

	// Create filename with timestamp
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("price_update_preview_%s_%s.json", planID, timestamp)

	// Create temp directory if it doesn't exist
	tempDir := "temp"
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Full file path
	filePath := filepath.Join(tempDir, filename)

	// Marshal to JSON with pretty printing
	jsonData, err := json.MarshalIndent(preview, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal preview data to JSON: %w", err)
	}

	// Write to file
	if err := os.WriteFile(filePath, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write preview file: %w", err)
	}

	s.log.Infow("Dry run preview generated", "file_path", filePath, "total_prices", len(pricePreviews))
	fmt.Printf("Preview JSON file saved to: %s\n", filePath)

	return nil
}

// extractCurrencySymbol extracts the currency symbol from a display amount string
// e.g., "$12.50" -> "$", "€12.50" -> "€", "12.50" -> ""
func (s *priceUpdateScript) extractCurrencySymbol(displayAmount string) string {
	if displayAmount == "" {
		return ""
	}

	// Common currency symbols to look for
	currencySymbols := []string{"$", "€", "£", "¥", "₹", "₽", "₩", "₪", "₦", "₨", "₡", "₱", "₴", "₵", "₸", "₺", "₼", "₾", "₿"}

	for _, symbol := range currencySymbols {
		if strings.HasPrefix(displayAmount, symbol) {
			return symbol
		}
	}

	// If no symbol found, return empty string
	return ""
}
