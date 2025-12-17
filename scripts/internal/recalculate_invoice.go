package internal

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/flexprice/flexprice/internal/cache"
	"github.com/flexprice/flexprice/internal/config"
	"github.com/flexprice/flexprice/internal/domain/invoice"
	"github.com/flexprice/flexprice/internal/logger"
	"github.com/flexprice/flexprice/internal/postgres"
	entRepo "github.com/flexprice/flexprice/internal/repository/ent"
	"github.com/flexprice/flexprice/internal/sentry"
	"github.com/flexprice/flexprice/internal/service"
	"github.com/flexprice/flexprice/internal/types"
)

// RecalculateInvoice force-drafts a finalized invoice and recalculates it
// Required environment variables:
//   - TENANT_ID: The tenant ID
//   - ENVIRONMENT_ID: The environment ID
//   - INVOICE_ID: The invoice ID to recalculate
func RecalculateInvoice() error {
	// Get environment variables for the script
	tenantID := os.Getenv("TENANT_ID")
	environmentID := os.Getenv("ENVIRONMENT_ID")
	invoiceID := os.Getenv("INVOICE_ID")

	if tenantID == "" || environmentID == "" || invoiceID == "" {
		return fmt.Errorf("TENANT_ID, ENVIRONMENT_ID, and INVOICE_ID are required")
	}

	log.Printf("Starting invoice recalculation for tenant: %s, environment: %s, invoice: %s\n",
		tenantID, environmentID, invoiceID)

	// Initialize script
	script, err := newRecalculateInvoiceScript()
	if err != nil {
		return fmt.Errorf("failed to initialize script: %w", err)
	}

	ctx := context.Background()
	ctx = context.WithValue(ctx, types.CtxTenantID, tenantID)
	ctx = context.WithValue(ctx, types.CtxEnvironmentID, environmentID)

	// Step 1: Get the invoice to verify it exists
	inv, err := script.invoiceRepo.Get(ctx, invoiceID)
	if err != nil {
		return fmt.Errorf("failed to get invoice: %w", err)
	}

	log.Printf("Found invoice: ID=%s, Status=%s, Type=%s\n",
		inv.ID, inv.InvoiceStatus, inv.InvoiceType)

	// Step 2: Validate this is a subscription invoice
	if inv.InvoiceType != types.InvoiceTypeSubscription {
		return fmt.Errorf("invoice %s is not a subscription invoice (type: %s)", invoiceID, inv.InvoiceType)
	}
	if inv.SubscriptionID == nil {
		return fmt.Errorf("invoice %s has no subscription ID", invoiceID)
	}

	// Step 3: Force draft the invoice if it's finalized
	if inv.InvoiceStatus == types.InvoiceStatusFinalized {
		log.Printf("Invoice is finalized, forcing draft status...\n")
		if err := script.invoiceRepo.ForceDraft(ctx, invoiceID); err != nil {
			return fmt.Errorf("failed to force draft invoice: %w", err)
		}
		log.Printf("Successfully set invoice status to DRAFT\n")
	} else if inv.InvoiceStatus == types.InvoiceStatusDraft {
		log.Printf("Invoice is already in draft status, proceeding with recalculation...\n")
	} else {
		return fmt.Errorf("invoice %s is in status %s, cannot recalculate", invoiceID, inv.InvoiceStatus)
	}

	// Step 4: Recalculate the invoice (with finalize=true to finalize it after recalculation)
	log.Printf("Recalculating invoice...\n")
	result, err := script.invoiceService.RecalculateInvoice(ctx, invoiceID, true)
	if err != nil {
		return fmt.Errorf("failed to recalculate invoice: %w", err)
	}

	log.Printf("Successfully recalculated invoice!\n")
	log.Printf("  Invoice ID: %s\n", result.ID)
	log.Printf("  Status: %s\n", result.InvoiceStatus)
	log.Printf("  Amount Due: %s %s\n", result.AmountDue.String(), result.Currency)
	log.Printf("  Line Items: %d\n", len(result.LineItems))

	return nil
}

type recalculateInvoiceScript struct {
	log            *logger.Logger
	invoiceRepo    invoice.Repository
	invoiceService service.InvoiceService
}

func newRecalculateInvoiceScript() (*recalculateInvoiceScript, error) {
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
	taxRateRepo := entRepo.NewTaxRateRepository(client, log, cacheClient)
	taxAssociationRepo := entRepo.NewTaxAssociationRepository(client, log, cacheClient)
	taxAppliedRepo := entRepo.NewTaxAppliedRepository(client, log, cacheClient)
	creditNoteRepo := entRepo.NewCreditNoteRepository(client, log, cacheClient)

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
		TaxRateRepo:              taxRateRepo,
		TaxAssociationRepo:       taxAssociationRepo,
		TaxAppliedRepo:           taxAppliedRepo,
		CreditNoteRepo:           creditNoteRepo,
	}

	// Create invoice service
	invoiceService := service.NewInvoiceService(serviceParams)

	return &recalculateInvoiceScript{
		log:            log,
		invoiceRepo:    invoiceRepo,
		invoiceService: invoiceService,
	}, nil
}
