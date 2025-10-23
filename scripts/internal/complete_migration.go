package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/flexprice/flexprice/internal/api/dto"
	"github.com/flexprice/flexprice/internal/cache"
	"github.com/flexprice/flexprice/internal/config"
	"github.com/flexprice/flexprice/internal/domain/plan"
	"github.com/flexprice/flexprice/internal/domain/price"
	"github.com/flexprice/flexprice/internal/domain/subscription"
	"github.com/flexprice/flexprice/internal/logger"
	"github.com/flexprice/flexprice/internal/postgres"
	entRepo "github.com/flexprice/flexprice/internal/repository/ent"
	"github.com/flexprice/flexprice/internal/sentry"
	"github.com/flexprice/flexprice/internal/service"
	"github.com/flexprice/flexprice/internal/types"
)

// CompleteMigration performs the complete migration process with configurable steps:
// 1. Create a master plan (STEP_1_CREATE_PLAN=true)
// 2. Add prices to it with group associated (STEP_2_CREATE_PRICES=true)
// 3. Cancel the subscription by modifying current_period_end and end_date with t1 and cancel_at_period_end true (STEP_3_CANCEL_SUBSCRIPTIONS=true)
// 4. Create new subscription with master plan with start day as t1 and billing cycle calendar (STEP_4_CREATE_SUBSCRIPTIONS=true)
func CompleteMigration() error {
	// Get environment variables
	tenantID := os.Getenv("TENANT_ID")
	environmentID := os.Getenv("ENVIRONMENT_ID")
	dryRunMode := os.Getenv("DRY_RUN_MODE")
	entityGroupsJSON := os.Getenv("ENTITY_GROUPS")
	t1TimeStr := os.Getenv("T1_TIME")
	masterPlanID := os.Getenv("MASTER_PLAN_ID") // For steps that need existing master plan

	// Step control flags
	step1CreatePlan := os.Getenv("STEP_1_CREATE_PLAN") == "true"
	step2CreatePrices := os.Getenv("STEP_2_CREATE_PRICES") == "true"
	step3CancelSubscriptions := os.Getenv("STEP_3_CANCEL_SUBSCRIPTIONS") == "true"
	step4CreateSubscriptions := os.Getenv("STEP_4_CREATE_SUBSCRIPTIONS") == "true"

	// If no steps specified, run all steps
	if !step1CreatePlan && !step2CreatePrices && !step3CancelSubscriptions && !step4CreateSubscriptions {
		step1CreatePlan = true
		step2CreatePrices = true
		step3CancelSubscriptions = true
		step4CreateSubscriptions = true
	}

	if tenantID == "" || environmentID == "" {
		return fmt.Errorf("TENANT_ID and ENVIRONMENT_ID are required")
	}

	// Validate required variables based on steps
	if step2CreatePrices && entityGroupsJSON == "" {
		return fmt.Errorf("ENTITY_GROUPS environment variable is required for step 2 (create prices)")
	}

	if (step3CancelSubscriptions || step4CreateSubscriptions) && t1TimeStr == "" {
		return fmt.Errorf("T1_TIME environment variable is required for steps 3 and 4 (format: 2006-01-02T15:04:05Z)")
	}

	if (step2CreatePrices || step4CreateSubscriptions) && !step1CreatePlan && masterPlanID == "" {
		return fmt.Errorf("MASTER_PLAN_ID environment variable is required when not creating a new master plan")
	}

	// Parse entity group mappings (only if needed)
	var entityGroupMappings map[string]string
	if step2CreatePrices && entityGroupsJSON != "" {
		if err := json.Unmarshal([]byte(entityGroupsJSON), &entityGroupMappings); err != nil {
			return fmt.Errorf("failed to parse ENTITY_GROUPS JSON: %w", err)
		}
	}

	// Parse T1 time (only if needed)
	var t1Time time.Time
	if (step3CancelSubscriptions || step4CreateSubscriptions) && t1TimeStr != "" {
		var err error
		t1Time, err = time.Parse(time.RFC3339, t1TimeStr)
		if err != nil {
			return fmt.Errorf("failed to parse T1_TIME: %w", err)
		}
	}

	isDryRun := dryRunMode == "true"

	// Setup
	cfg, err := config.NewConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	log, err := logger.NewLogger(cfg)
	if err != nil {
		return fmt.Errorf("failed to create logger: %w", err)
	}

	entClient, err := postgres.NewEntClient(cfg, log)
	if err != nil {
		return fmt.Errorf("failed to connect to postgres: %w", err)
	}
	defer entClient.Close()

	pgClient := postgres.NewClient(entClient, log, sentry.NewSentryService(cfg, log))
	cacheClient := cache.NewInMemoryCache()

	// Initialize repositories - all required for subscription creation
	priceRepo := entRepo.NewPriceRepository(pgClient, log, cacheClient)
	planRepo := entRepo.NewPlanRepository(pgClient, log, cacheClient)
	subscriptionRepo := entRepo.NewSubscriptionRepository(pgClient, log, cacheClient)
	customerRepo := entRepo.NewCustomerRepository(pgClient, log, cacheClient)
	meterRepo := entRepo.NewMeterRepository(pgClient, log, cacheClient)
	creditGrantRepo := entRepo.NewCreditGrantRepository(pgClient, log, cacheClient)
	creditGrantApplicationRepo := entRepo.NewCreditGrantApplicationRepository(pgClient, log, cacheClient)
	walletRepo := entRepo.NewWalletRepository(pgClient, log, cacheClient)
	featureRepo := entRepo.NewFeatureRepository(pgClient, log, cacheClient)
	entitlementRepo := entRepo.NewEntitlementRepository(pgClient, log, cacheClient)
	subscriptionLineItemRepo := entRepo.NewSubscriptionLineItemRepository(pgClient, log, cacheClient)
	tenantRepo := entRepo.NewTenantRepository(pgClient, log, cacheClient)
	environmentRepo := entRepo.NewEnvironmentRepository(pgClient, log)
	taxRateRepo := entRepo.NewTaxRateRepository(pgClient, log, cacheClient)
	taxAssociationRepo := entRepo.NewTaxAssociationRepository(pgClient, log, cacheClient)
	taxAppliedRepo := entRepo.NewTaxAppliedRepository(pgClient, log, cacheClient)
	couponRepo := entRepo.NewCouponRepository(pgClient, log, cacheClient)
	couponAssociationRepo := entRepo.NewCouponAssociationRepository(pgClient, log, cacheClient)
	couponApplicationRepo := entRepo.NewCouponApplicationRepository(pgClient, log, cacheClient)
	creditNoteRepo := entRepo.NewCreditNoteRepository(pgClient, log, cacheClient)
	creditNoteLineItemRepo := entRepo.NewCreditNoteLineItemRepository(pgClient, log, cacheClient)
	connectionRepo := entRepo.NewConnectionRepository(pgClient, log, cacheClient)
	entityIntegrationMappingRepo := entRepo.NewEntityIntegrationMappingRepository(pgClient, log, cacheClient)
	settingsRepo := entRepo.NewSettingsRepository(pgClient, log, cacheClient)
	alertLogsRepo := entRepo.NewAlertLogsRepository(pgClient, log, cacheClient)
	groupRepo := entRepo.NewGroupRepository(pgClient, log, cacheClient)
	priceUnitRepo := entRepo.NewPriceUnitRepository(pgClient, log, cacheClient)

	// Additional repositories from main.go
	userRepo := entRepo.NewUserRepository(pgClient, log)
	authRepo := entRepo.NewAuthRepository(pgClient, log)
	invoiceRepo := entRepo.NewInvoiceRepository(pgClient, log, cacheClient)
	paymentRepo := entRepo.NewPaymentRepository(pgClient, log, cacheClient)
	taskRepo := entRepo.NewTaskRepository(pgClient, log)
	secretRepo := entRepo.NewSecretRepository(pgClient, log, cacheClient)
	costSheetRepo := entRepo.NewCostSheetRepository(pgClient, log)
	addonRepo := entRepo.NewAddonRepository(pgClient, log, cacheClient)
	addonAssociationRepo := entRepo.NewAddonAssociationRepository(pgClient, log, cacheClient)
	subscriptionScheduleRepo := entRepo.NewSubscriptionScheduleRepository(pgClient, log, cacheClient)

	// Initialize services
	serviceParams := service.ServiceParams{
		DB:     pgClient,
		Config: cfg,
		Logger: log,
		// Core repositories
		AuthRepo:                 authRepo,
		UserRepo:                 userRepo,
		PlanRepo:                 planRepo,
		PriceRepo:                priceRepo,
		PriceUnitRepo:            priceUnitRepo,
		SubRepo:                  subscriptionRepo,
		SubscriptionScheduleRepo: subscriptionScheduleRepo,
		SubscriptionLineItemRepo: subscriptionLineItemRepo,
		CustomerRepo:             customerRepo,
		MeterRepo:                meterRepo,

		// Financial repositories
		CreditGrantRepo:            creditGrantRepo,
		CreditGrantApplicationRepo: creditGrantApplicationRepo,
		WalletRepo:                 walletRepo,
		InvoiceRepo:                invoiceRepo,
		PaymentRepo:                paymentRepo,
		CreditNoteRepo:             creditNoteRepo,
		CreditNoteLineItemRepo:     creditNoteLineItemRepo,
		CostSheetRepo:              costSheetRepo,

		// Feature and entitlement repositories
		FeatureRepo:     featureRepo,
		EntitlementRepo: entitlementRepo,

		// Tax repositories
		TaxRateRepo:        taxRateRepo,
		TaxAssociationRepo: taxAssociationRepo,
		TaxAppliedRepo:     taxAppliedRepo,

		// Coupon repositories
		CouponRepo:            couponRepo,
		CouponAssociationRepo: couponAssociationRepo,
		CouponApplicationRepo: couponApplicationRepo,

		// Addon repositories
		AddonRepo:            addonRepo,
		AddonAssociationRepo: addonAssociationRepo,

		// System repositories
		TenantRepo:                   tenantRepo,
		EnvironmentRepo:              environmentRepo,
		TaskRepo:                     taskRepo,
		SecretRepo:                   secretRepo,
		ConnectionRepo:               connectionRepo,
		EntityIntegrationMappingRepo: entityIntegrationMappingRepo,
		SettingsRepo:                 settingsRepo,
		AlertLogsRepo:                alertLogsRepo,
		GroupRepo:                    groupRepo,
	}

	planService := service.NewPlanService(serviceParams)
	priceService := service.NewPriceService(serviceParams)
	subscriptionService := service.NewSubscriptionService(serviceParams)

	ctx := context.WithValue(context.Background(), types.CtxTenantID, tenantID)
	ctx = context.WithValue(ctx, types.CtxEnvironmentID, environmentID)

	log.Infow("Starting complete migration process",
		"tenant_id", tenantID,
		"environment_id", environmentID,
		"dry_run", isDryRun,
		"step_1_create_plan", step1CreatePlan,
		"step_2_create_prices", step2CreatePrices,
		"step_3_cancel_subscriptions", step3CancelSubscriptions,
		"step_4_create_subscriptions", step4CreateSubscriptions,
		"entity_group_mappings", len(entityGroupMappings),
		"t1_time", t1Time)

	var masterPlan *dto.CreatePlanResponse
	var newPrices []*price.Price
	var activeSubscriptions []*subscription.Subscription

	// Step 1: Create Master plan
	if step1CreatePlan {
		log.Infow("=== STEP 1: Creating Master Plan ===")
		var err error
		masterPlan, err = createMasterPlan(ctx, planService, log, isDryRun)
		if err != nil {
			return fmt.Errorf("failed to create Master plan: %w", err)
		}
		log.Infow("Step 1 completed", "master_plan_id", masterPlan.ID)
	} else if masterPlanID != "" {
		// Use existing master plan
		masterPlan = &dto.CreatePlanResponse{
			Plan: &plan.Plan{ID: masterPlanID, Name: "Master"},
		}
		log.Infow("Using existing master plan", "master_plan_id", masterPlanID)
	}

	// Step 2: Create prices for master plan with groups
	if step2CreatePrices {
		log.Infow("=== STEP 2: Creating Prices for Master Plan ===")
		if masterPlan == nil {
			return fmt.Errorf("master plan is required for step 2, but step 1 was not run and MASTER_PLAN_ID not provided")
		}

		// Extract entity IDs from mappings
		entityIDs := make([]string, 0, len(entityGroupMappings))
		for entityID := range entityGroupMappings {
			entityIDs = append(entityIDs, entityID)
		}

		// Query prices based on entity IDs
		activePrices, err := queryPricesByEntityIDs(ctx, priceRepo, entityIDs, log)
		if err != nil {
			return fmt.Errorf("failed to query prices by entity IDs: %w", err)
		}

		// Create new prices for Master plan based on existing prices
		newPrices, err = createPricesForMasterPlan(ctx, activePrices, masterPlan.ID, priceService, entityGroupMappings, log, isDryRun)
		if err != nil {
			return fmt.Errorf("failed to create new prices for Master plan: %w", err)
		}
		log.Infow("Step 2 completed", "new_prices_created", len(newPrices))
	}

	// Step 3: Cancel existing subscriptions at T1
	if step3CancelSubscriptions {
		log.Infow("=== STEP 3: Canceling Subscriptions at T1 ===")
		// Fetch all active subscriptions
		var err error
		activeSubscriptions, err = fetchActiveSubscriptions(ctx, subscriptionRepo, log)
		if err != nil {
			return fmt.Errorf("failed to fetch active subscriptions: %w", err)
		}

		// Cancel existing subscriptions at T1 with period end modification
		if err := cancelSubscriptionsAtT1WithPeriodEnd(ctx, activeSubscriptions, t1Time, subscriptionRepo, log, isDryRun); err != nil {
			return fmt.Errorf("failed to cancel subscriptions at T1: %w", err)
		}
		log.Infow("Step 3 completed", "subscriptions_canceled", len(activeSubscriptions))
	}

	// Step 4: Create new subscriptions with Master plan
	if step4CreateSubscriptions {
		log.Infow("=== STEP 4: Creating New Subscriptions with Master Plan ===")
		if masterPlan == nil {
			return fmt.Errorf("master plan is required for step 4, but step 1 was not run and MASTER_PLAN_ID not provided")
		}

		// Fetch active subscriptions if not already fetched in step 3
		if activeSubscriptions == nil {
			var err error
			activeSubscriptions, err = fetchActiveSubscriptions(ctx, subscriptionRepo, log)
			if err != nil {
				return fmt.Errorf("failed to fetch active subscriptions: %w", err)
			}
		}

		// Create new subscriptions with Master plan starting at T1 with calendar billing
		if err := createNewSubscriptionsWithMasterPlanCalendar(ctx, activeSubscriptions, masterPlan.ID, t1Time, subscriptionService, log, isDryRun); err != nil {
			return fmt.Errorf("failed to create new subscriptions with Master plan: %w", err)
		}
		log.Infow("Step 4 completed", "new_subscriptions_created", len(activeSubscriptions))
	}

	// Summary
	var masterPlanIDStr string
	if masterPlan != nil {
		masterPlanIDStr = masterPlan.ID
	}

	log.Infow("Complete migration process finished successfully",
		"steps_executed", map[string]bool{
			"step_1_create_plan":          step1CreatePlan,
			"step_2_create_prices":        step2CreatePrices,
			"step_3_cancel_subscriptions": step3CancelSubscriptions,
			"step_4_create_subscriptions": step4CreateSubscriptions,
		},
		"master_plan_id", masterPlanIDStr,
		"new_prices_created", len(newPrices),
		"subscriptions_processed", len(activeSubscriptions),
		"entity_mappings", len(entityGroupMappings))

	return nil
}

// cancelSubscriptionsAtT1WithPeriodEnd cancels existing subscriptions at T1 time by modifying current_period_end and end_date
func cancelSubscriptionsAtT1WithPeriodEnd(ctx context.Context, subscriptions []*subscription.Subscription, t1Time time.Time, subscriptionRepo subscription.Repository, log *logger.Logger, isDryRun bool) error {
	log.Infow("Canceling subscriptions at T1 with period end modification", "t1_time", t1Time, "subscription_count", len(subscriptions))

	if isDryRun {
		log.Infow("DRY RUN: Would cancel subscriptions at T1 with period end modification", "count", len(subscriptions))
		for _, sub := range subscriptions {
			log.Infow("DRY RUN: Would cancel subscription",
				"subscription_id", sub.ID,
				"customer_id", sub.CustomerID,
				"current_period_start", sub.CurrentPeriodStart,
				"current_period_end", sub.CurrentPeriodEnd,
				"new_period_end", t1Time,
				"new_end_date", t1Time,
				"cancel_at_period_end", true)
		}
		return nil
	}

	// Cancel each subscription at T1 by modifying period end and end date
	for _, sub := range subscriptions {
		// Create a copy of the subscription and update the cancellation fields
		updatedSub := *sub
		updatedSub.CurrentPeriodEnd = t1Time
		updatedSub.EndDate = &t1Time
		updatedSub.CancelAtPeriodEnd = true
		updatedSub.CancelAt = &t1Time

		// Update the subscription in the database
		if err := subscriptionRepo.Update(ctx, &updatedSub); err != nil {
			log.Errorw("Failed to cancel subscription at T1",
				"subscription_id", sub.ID,
				"t1_time", t1Time,
				"error", err)
			return fmt.Errorf("failed to cancel subscription %s at T1: %w", sub.ID, err)
		}

		log.Infow("Canceled subscription at T1 with period end modification",
			"subscription_id", sub.ID,
			"customer_id", sub.CustomerID,
			"original_period_end", sub.CurrentPeriodEnd,
			"new_period_end", t1Time,
			"end_date", t1Time,
			"cancel_at_period_end", true)
	}

	log.Infow("Canceled subscriptions at T1 with period end modification", "count", len(subscriptions))
	return nil
}

// createNewSubscriptionsWithMasterPlanCalendar creates new subscriptions with Master plan starting at T1 with calendar billing
func createNewSubscriptionsWithMasterPlanCalendar(ctx context.Context, oldSubscriptions []*subscription.Subscription, masterPlanID string, t1Time time.Time, subscriptionService service.SubscriptionService, log *logger.Logger, isDryRun bool) error {
	log.Infow("Creating new subscriptions with Master plan and calendar billing", "master_plan_id", masterPlanID, "t1_time", t1Time, "subscription_count", len(oldSubscriptions))

	if isDryRun {
		log.Infow("DRY RUN: Would create new subscriptions with Master plan and calendar billing", "count", len(oldSubscriptions))
		for _, oldSub := range oldSubscriptions {
			log.Infow("DRY RUN: Would create new subscription",
				"old_subscription_id", oldSub.ID,
				"customer_id", oldSub.CustomerID,
				"master_plan_id", masterPlanID,
				"start_date", t1Time,
				"billing_cycle", types.BillingCycleCalendar,
				"billing_period", oldSub.BillingPeriod,
				"billing_period_count", oldSub.BillingPeriodCount)
		}
		return nil
	}

	// Create new subscription for each old subscription
	for _, oldSub := range oldSubscriptions {
		createReq := dto.CreateSubscriptionRequest{
			CustomerID:         oldSub.CustomerID,
			PlanID:             masterPlanID,
			Currency:           oldSub.Currency,
			StartDate:          &t1Time,
			BillingCadence:     oldSub.BillingCadence,
			BillingPeriod:      oldSub.BillingPeriod,
			BillingPeriodCount: oldSub.BillingPeriodCount,
			BillingCycle:       types.BillingCycleCalendar, // Calendar billing as requested
			Metadata: types.Metadata{
				"migrated_from_subscription": oldSub.ID,
				"migration_t1_time":          t1Time.Format(time.RFC3339),
				"created_by":                 "complete_migration_script",
				"original_plan_id":           oldSub.PlanID,
			},
		}

		newSub, err := subscriptionService.CreateSubscription(ctx, createReq)
		if err != nil {
			log.Errorw("Failed to create new subscription with Master plan",
				"old_subscription_id", oldSub.ID,
				"customer_id", oldSub.CustomerID,
				"master_plan_id", masterPlanID,
				"t1_time", t1Time,
				"error", err)
			return fmt.Errorf("failed to create new subscription for customer %s with Master plan: %w", oldSub.CustomerID, err)
		}

		log.Infow("Created new subscription with Master plan and calendar billing",
			"new_subscription_id", newSub.ID,
			"old_subscription_id", oldSub.ID,
			"customer_id", oldSub.CustomerID,
			"master_plan_id", masterPlanID,
			"start_date", t1Time,
			"billing_cycle", types.BillingCycleCalendar,
			"billing_period", oldSub.BillingPeriod,
			"billing_period_count", oldSub.BillingPeriodCount)
	}

	log.Infow("Created new subscriptions with Master plan and calendar billing", "count", len(oldSubscriptions))
	return nil
}

// fetchActiveSubscriptions fetches all active subscriptions for the tenant and environment
func fetchActiveSubscriptions(ctx context.Context, subscriptionRepo subscription.Repository, log *logger.Logger) ([]*subscription.Subscription, error) {
	log.Infow("Fetching active subscriptions")

	// Create filter for active subscriptions
	filter := types.NewNoLimitSubscriptionFilter()
	filter.SubscriptionStatus = []types.SubscriptionStatus{types.SubscriptionStatusActive}

	subscriptions, err := subscriptionRepo.List(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to list active subscriptions: %w", err)
	}

	log.Infow("Found active subscriptions", "count", len(subscriptions))
	return subscriptions, nil
}

// createPricesForMasterPlan creates new prices for the Master plan based on existing prices
func createPricesForMasterPlan(ctx context.Context, prices []*price.Price, planID string, priceService service.PriceService, entityGroupMappings map[string]string, log *logger.Logger, isDryRun bool) ([]*price.Price, error) {
	log.Infow("Creating new prices for Master plan", "plan_id", planID, "price_count", len(prices))

	if isDryRun {
		log.Infow("DRY RUN: Would create new prices for Master plan", "count", len(prices))
		newPrices := make([]*price.Price, 0, len(prices))
		for _, originalPrice := range prices {
			log.Infow("DRY RUN: Would create new price for Master plan",
				"original_price_id", originalPrice.ID,
				"original_entity_id", originalPrice.EntityID,
				"original_entity_type", originalPrice.EntityType,
				"new_entity_id", planID,
				"new_entity_type", types.PRICE_ENTITY_TYPE_PLAN)

			// Create a mock new price for dry run
			newPrice := *originalPrice
			newPrice.ID = "dry-run-price-" + originalPrice.ID
			newPrice.EntityID = planID
			newPrice.EntityType = types.PRICE_ENTITY_TYPE_PLAN
			newPrices = append(newPrices, &newPrice)
		}
		return newPrices, nil
	}

	// Create new prices for the Master plan using DTO and bulk creation
	newPrices := make([]*price.Price, 0, len(prices))
	createPriceRequests := make([]dto.CreatePriceRequest, 0, len(prices))

	for _, originalPrice := range prices {
		// Create a CreatePriceRequest based on the original price

		createReq := dto.CreatePriceRequest{
			Amount:               originalPrice.Amount.String(),
			Currency:             originalPrice.Currency,
			EntityType:           types.PRICE_ENTITY_TYPE_PLAN,
			EntityID:             planID,
			Type:                 originalPrice.Type,
			PriceUnitType:        originalPrice.PriceUnitType,
			BillingPeriod:        originalPrice.BillingPeriod,
			BillingPeriodCount:   originalPrice.BillingPeriodCount,
			BillingModel:         originalPrice.BillingModel,
			BillingCadence:       originalPrice.BillingCadence,
			MeterID:              originalPrice.MeterID,
			LookupKey:            originalPrice.LookupKey,
			InvoiceCadence:       originalPrice.InvoiceCadence,
			TrialPeriod:          originalPrice.TrialPeriod,
			Description:          originalPrice.Description,
			TierMode:             originalPrice.TierMode,
			StartDate:            originalPrice.StartDate,
			EndDate:              originalPrice.EndDate,
			ParentPriceID:        originalPrice.ID, // Link to original price
			SkipEntityValidation: true,
			GroupID:              entityGroupMappings[originalPrice.EntityID], // Skip validation since we know the master plan exists
		}

		// Copy tiers if they exist
		if len(originalPrice.Tiers) > 0 {
			createReq.Tiers = make([]dto.CreatePriceTier, len(originalPrice.Tiers))
			for i, tier := range originalPrice.Tiers {
				createReq.Tiers[i] = dto.CreatePriceTier{
					UpTo:       tier.UpTo,
					UnitAmount: tier.UnitAmount.String(),
				}
				if tier.FlatAmount != nil {
					flatAmountStr := tier.FlatAmount.String()
					createReq.Tiers[i].FlatAmount = &flatAmountStr
				}
			}
		}

		// Copy transform quantity if it exists
		if originalPrice.TransformQuantity != (price.JSONBTransformQuantity{}) {
			transformQuantity := price.TransformQuantity(originalPrice.TransformQuantity)
			createReq.TransformQuantity = &transformQuantity
		}

		// Set up metadata to indicate this is a migrated price
		createReq.Metadata = make(map[string]string)
		if originalPrice.Metadata != nil {
			// Copy existing metadata
			for k, v := range originalPrice.Metadata {
				createReq.Metadata[k] = v
			}
		}
		createReq.Metadata["created_by"] = "plan_grouping_script"
		createReq.Metadata["original_price_id"] = originalPrice.ID
		createReq.Metadata["original_entity_id"] = originalPrice.EntityID
		createReq.Metadata["original_entity_type"] = string(originalPrice.EntityType)

		createPriceRequests = append(createPriceRequests, createReq)
	}

	// Create prices in bulk using the price service
	bulkCreateReq := dto.CreateBulkPriceRequest{
		Items: createPriceRequests,
	}

	bulkResponse, err := priceService.CreateBulkPrice(ctx, bulkCreateReq)
	if err != nil {
		log.Errorw("Failed to create new prices for Master plan in bulk",
			"plan_id", planID,
			"price_count", len(createPriceRequests),
			"error", err)
		return nil, fmt.Errorf("failed to create new prices for Master plan in bulk: %w", err)
	}

	// Extract the created prices from the response
	for _, priceResp := range bulkResponse.Items {
		newPrices = append(newPrices, priceResp.Price)
	}

	log.Infow("Created new prices for Master plan in bulk", "count", len(newPrices))
	return newPrices, nil
}

// createMasterPlan creates a new Master plan
func createMasterPlan(ctx context.Context, planService service.PlanService, log *logger.Logger, isDryRun bool) (*dto.CreatePlanResponse, error) {
	log.Infow("Creating Master plan", "dry_run", isDryRun)

	if isDryRun {
		log.Infow("DRY RUN: Would create Master plan")
		return &dto.CreatePlanResponse{
			Plan: &plan.Plan{
				ID:   "dry-run-master-plan-id",
				Name: "Master",
			},
		}, nil
	}

	req := dto.CreatePlanRequest{
		Name:        "Master",
		Description: "Master plan containing all active prices grouped by entity",
		LookupKey:   "master",
		Metadata: types.Metadata{
			"created_by": "plan_grouping_script",
			"purpose":    "master_plan_for_grouped_prices",
		},
	}

	plan, err := planService.CreatePlan(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create Master plan: %w", err)
	}

	log.Infow("Created Master plan", "plan_id", plan.ID, "plan_name", plan.Name)
	return plan, nil
}

// queryPricesByEntityIDs queries prices based on specific entity IDs
func queryPricesByEntityIDs(ctx context.Context, priceRepo price.Repository, entityIDs []string, log *logger.Logger) ([]*price.Price, error) {
	log.Infow("Querying prices by entity IDs", "entity_ids", entityIDs)

	// Create filter for prices with specific entity IDs and active status
	filter := types.NewNoLimitPriceFilter().
		WithEntityIDs(entityIDs).
		WithStatus(types.StatusPublished)

	prices, err := priceRepo.List(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to list prices by entity IDs: %w", err)
	}

	log.Infow("Found prices for entity IDs", "count", len(prices), "entity_ids", entityIDs)
	return prices, nil
}
