# Group-Level Entitlements Implementation Guide

## Overview

This document outlines the implementation plan for adding group-level entitlements to the Flexprice billing system. Currently, entitlements operate on a one-to-one basis (one plan/addon → one feature). This implementation will enable entitlements to be scoped to price groups within plans.

**Target Use Case:** Apply entitlement limits to a group of prices rather than all prices in a plan.

**Example:** 
- Plan: "Enterprise API"
- Price Group: "Premium API Tiers" (contains: Basic API Price, Pro API Price, Enterprise API Price)
- Entitlement: "10,000 API Calls" applied to the "Premium API Tiers" group
- Result: Customers with any price from this group get 10,000 calls (pooled across the group)

---

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Database Migration](#database-migration)
3. [Type System Changes](#type-system-changes)
4. [Schema Changes](#schema-changes)
5. [Repository Changes](#repository-changes)
6. [DTO Changes](#dto-changes)
7. [Service Layer Changes](#service-layer-changes)
8. [API Changes](#api-changes)
9. [Testing Strategy](#testing-strategy)
10. [Rollout Plan](#rollout-plan)
11. [Backward Compatibility](#backward-compatibility)

---

## Architecture Overview

### Current Architecture
```
Plan → Entitlement → Feature
  ↓
Prices (all prices in plan get same entitlement)
```

### New Architecture
```
Plan → Entitlement (with optional price_group_id) → Feature
  ↓
Price Group → Multiple Prices
  ↓
Scoped Entitlement (only prices in group get this entitlement)
```

### Key Design Decisions

1. **Schema Field:** Add `price_group_id` (nullable) to entitlement table
2. **Allocation Mode:** Add `allocation_mode` field with values: `pooled` (default) or `per_price`
3. **Priority:** Group-level entitlements override plan-level entitlements for the same feature
4. **Backward Compatibility:** NULL `price_group_id` = plan-level entitlement (existing behavior)

---

## Database Migration

### Step 1: Migration File

**File:** `migrations/postgres/YYYYMMDDHHMMSS_add_group_support_to_entitlements.sql`

```sql
-- Migration: Add group support to entitlements
-- Date: YYYY-MM-DD
-- Description: Adds price_group_id and allocation_mode to entitlements table

BEGIN;

-- Step 1: Add new columns
ALTER TABLE entitlements 
ADD COLUMN price_group_id VARCHAR(50),
ADD COLUMN allocation_mode VARCHAR(20) DEFAULT 'pooled';

-- Step 2: Add comment for documentation
COMMENT ON COLUMN entitlements.price_group_id IS 'Optional reference to price group. NULL means plan-level (all prices)';
COMMENT ON COLUMN entitlements.allocation_mode IS 'How limits are allocated: pooled (shared across group) or per_price (individual per price)';

-- Step 3: Add foreign key constraint (optional - depends on if you want hard FK)
-- ALTER TABLE entitlements 
-- ADD CONSTRAINT fk_entitlements_price_group_id 
-- FOREIGN KEY (price_group_id, tenant_id, environment_id) 
-- REFERENCES groups(id, tenant_id, environment_id) 
-- ON DELETE CASCADE;

-- Step 4: Create index for group lookups
CREATE INDEX idx_entitlements_price_group_id 
ON entitlements(tenant_id, environment_id, entity_id, price_group_id) 
WHERE status = 'published';

-- Step 5: Drop old unique constraint
ALTER TABLE entitlements 
DROP CONSTRAINT IF EXISTS entitlements_tenant_id_environment_id_entity_type_entity_id_feature_id_key;

-- Step 6: Create new unique constraint including price_group_id
CREATE UNIQUE INDEX entitlements_unique_constraint 
ON entitlements(tenant_id, environment_id, entity_type, entity_id, COALESCE(price_group_id, ''), feature_id) 
WHERE status = 'published';

-- Step 7: Add check constraint for allocation_mode
ALTER TABLE entitlements 
ADD CONSTRAINT check_allocation_mode 
CHECK (allocation_mode IN ('pooled', 'per_price'));

COMMIT;
```

### Step 2: Rollback Migration

**File:** `migrations/postgres/YYYYMMDDHHMMSS_add_group_support_to_entitlements_down.sql`

```sql
BEGIN;

-- Remove new constraint
DROP INDEX IF EXISTS entitlements_unique_constraint;

-- Restore old constraint
CREATE UNIQUE INDEX entitlements_tenant_id_environment_id_entity_type_entity_id_feature_id_key 
ON entitlements(tenant_id, environment_id, entity_type, entity_id, feature_id) 
WHERE status = 'published';

-- Remove index
DROP INDEX IF EXISTS idx_entitlements_price_group_id;

-- Remove columns
ALTER TABLE entitlements 
DROP COLUMN IF EXISTS price_group_id,
DROP COLUMN IF EXISTS allocation_mode;

COMMIT;
```

---

## Type System Changes

### File: `internal/types/entitlement.go`

```go
// Add new allocation mode constants
const (
	ENTITLEMENT_ALLOCATION_MODE_POOLED    AllocationMode = "pooled"
	ENTITLEMENT_ALLOCATION_MODE_PER_PRICE AllocationMode = "per_price"
)

type AllocationMode string

func (a AllocationMode) Validate() error {
	switch a {
	case ENTITLEMENT_ALLOCATION_MODE_POOLED, ENTITLEMENT_ALLOCATION_MODE_PER_PRICE:
		return nil
	default:
		return fmt.Errorf("invalid allocation mode: %s", a)
	}
}

func (a AllocationMode) IsPooled() bool {
	return a == ENTITLEMENT_ALLOCATION_MODE_POOLED
}

func (a AllocationMode) IsPerPrice() bool {
	return a == ENTITLEMENT_ALLOCATION_MODE_PER_PRICE
}
```

### File: `internal/types/filter.go`

```go
// Update EntitlementFilter to support price_group_id filtering
type EntitlementFilter struct {
	*QueryFilter
	EntityIDs      []string
	EntityType     *EntitlementEntityType
	FeatureIDs     []string
	FeatureID      *string
	PriceGroupIDs  []string  // NEW
	Status         *Status
}

// Add helper method
func (f *EntitlementFilter) WithPriceGroupIDs(ids []string) *EntitlementFilter {
	f.PriceGroupIDs = ids
	return f
}
```

---

## Schema Changes

### File: `ent/schema/entitlement.go`

```go
// Fields of the Entitlement.
func (Entitlement) Fields() []ent.Field {
	return []ent.Field{
		// ... existing fields ...
		
		// NEW: Price group support
		field.String("price_group_id").
			SchemaType(map[string]string{
				"postgres": "varchar(50)",
			}).
			Optional().
			Nillable().
			Comment("Optional reference to price group. NULL means plan-level entitlement"),
		
		// NEW: Allocation mode
		field.String("allocation_mode").
			SchemaType(map[string]string{
				"postgres": "varchar(20)",
			}).
			Default("pooled").
			Optional().
			Comment("How limits are allocated: pooled (shared) or per_price (individual)"),
	}
}

// Indexes of the Entitlement.
func (Entitlement) Indexes() []ent.Index {
	return []ent.Index{
		// Update unique constraint to include price_group_id
		index.Fields("tenant_id", "environment_id", "entity_type", "entity_id", "price_group_id", "feature_id").
			Unique().
			Annotations(entsql.IndexWhere("status = 'published'")),

		index.Fields("tenant_id", "environment_id", "entity_type", "entity_id"),
		index.Fields("tenant_id", "environment_id", "feature_id"),
		
		// NEW: Index for group lookups
		index.Fields("tenant_id", "environment_id", "entity_id", "price_group_id").
			Annotations(entsql.IndexWhere("status = 'published' AND price_group_id IS NOT NULL")),
	}
}
```

**Note:** After modifying schema, run:
```bash
go generate ./ent
```

---

## Repository Changes

### File: `internal/repository/entitlement.go`

Update the List method to support price_group_id filtering:

```go
func (r *entitlementRepository) List(ctx context.Context, filter *types.EntitlementFilter) ([]*entitlement.Entitlement, error) {
	query := r.client.Entitlement.Query().
		Where(entitlement.TenantID(types.GetTenantID(ctx)))

	// ... existing filters ...

	// NEW: Filter by price group IDs
	if len(filter.PriceGroupIDs) > 0 {
		query = query.Where(entitlement.PriceGroupIDIn(filter.PriceGroupIDs...))
	}

	return query.All(ctx)
}
```

---

## DTO Changes

### File: `internal/api/dto/entitlement.go`

```go
// CreateEntitlementRequest - add new fields
type CreateEntitlementRequest struct {
	// ... existing fields ...
	
	PriceGroupID   *string `json:"price_group_id,omitempty" validate:"omitempty,ulid"`
	AllocationMode string  `json:"allocation_mode,omitempty" validate:"omitempty,oneof=pooled per_price"`
}

// UpdateEntitlementRequest - add new fields
type UpdateEntitlementRequest struct {
	// ... existing fields ...
	
	AllocationMode *string `json:"allocation_mode,omitempty" validate:"omitempty,oneof=pooled per_price"`
}

// EntitlementResponse - add new fields
type EntitlementResponse struct {
	*entitlement.Entitlement
	
	// ... existing fields ...
	
	PriceGroupID   string         `json:"price_group_id,omitempty"`
	AllocationMode string         `json:"allocation_mode,omitempty"`
	Group          *GroupResponse `json:"group,omitempty"` // NEW: Expand group info
}

// AggregatedEntitlement - add new field
type AggregatedEntitlement struct {
	IsEnabled        bool                            `json:"is_enabled"`
	UsageLimit       *int64                          `json:"usage_limit,omitempty"`
	UsageResetPeriod types.EntitlementUsageResetPeriod `json:"usage_reset_period,omitempty"`
	IsSoftLimit      bool                            `json:"is_soft_limit"`
	StaticValues     []string                        `json:"static_values,omitempty"`
	AllocationMode   string                          `json:"allocation_mode,omitempty"` // NEW
	PriceGroupID     string                          `json:"price_group_id,omitempty"`  // NEW
}

// EntitlementSource - add group info
type EntitlementSource struct {
	SubscriptionID string                  `json:"subscription_id,omitempty"`
	EntityID       string                  `json:"entity_id"`
	EntityType     EntitlementSourceType   `json:"entity_type"`
	EntityName     string                  `json:"entity_name,omitempty"`
	Quantity       int                     `json:"quantity"`
	EntitlementID  string                  `json:"entitlement_id"`
	IsEnabled      bool                    `json:"is_enabled"`
	UsageLimit     *int64                  `json:"usage_limit,omitempty"`
	StaticValue    string                  `json:"static_value,omitempty"`
	PriceGroupID   string                  `json:"price_group_id,omitempty"`   // NEW
	GroupName      string                  `json:"group_name,omitempty"`       // NEW
}
```

### File: `internal/api/dto/entitlement_conversion.go`

```go
func (r *CreateEntitlementRequest) ToEntitlement(ctx context.Context) *entitlement.Entitlement {
	e := &entitlement.Entitlement{
		// ... existing fields ...
		
		PriceGroupID:   lo.FromPtr(r.PriceGroupID),
		AllocationMode: types.AllocationMode(r.AllocationMode),
	}
	
	// Set default allocation mode if not provided
	if e.AllocationMode == "" {
		e.AllocationMode = types.ENTITLEMENT_ALLOCATION_MODE_POOLED
	}
	
	return e
}
```

---

## Service Layer Changes

### 1. File: `internal/service/entitlement.go`

#### Update CreateEntitlement

```go
func (s *entitlementService) CreateEntitlement(ctx context.Context, req dto.CreateEntitlementRequest) (*dto.EntitlementResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	// ... existing entity validation ...

	// NEW: Validate price group if provided
	if req.PriceGroupID != nil && *req.PriceGroupID != "" {
		if err := s.validatePriceGroup(ctx, *req.PriceGroupID, entityID, entityType); err != nil {
			return nil, err
		}
	}

	// ... rest of creation logic ...

	// NEW: Add expanded group info if present
	if result.PriceGroupID != "" {
		groupService := NewGroupService(s.ServiceParams)
		group, err := groupService.GetGroup(ctx, result.PriceGroupID)
		if err == nil {
			response.Group = group
		}
	}

	return response, nil
}

// NEW: Validation helper
func (s *entitlementService) validatePriceGroup(
	ctx context.Context,
	groupID string,
	entityID string,
	entityType types.EntitlementEntityType,
) error {
	// Validate group exists
	groupService := NewGroupService(s.ServiceParams)
	group, err := groupService.GetGroup(ctx, groupID)
	if err != nil {
		return ierr.WithError(err).
			WithHint("Price group not found").
			WithReportableDetails(map[string]interface{}{
				"group_id": groupID,
			}).
			Mark(ierr.ErrNotFound)
	}

	// Validate group is of type "price"
	if group.EntityType != "price" {
		return ierr.NewError("group must be of entity type 'price'").
			WithHint("Only price groups can be used for entitlements").
			WithReportableDetails(map[string]interface{}{
				"group_id":          groupID,
				"group_entity_type": group.EntityType,
			}).
			Mark(ierr.ErrValidation)
	}

	// Validate group has prices belonging to this entity
	priceService := NewPriceService(s.ServiceParams)
	priceFilter := types.NewNoLimitPriceFilter().
		WithEntityIDs([]string{entityID}).
		WithEntityType(getPriceEntityType(entityType)).
		WithGroupID(groupID)

	prices, err := priceService.GetPrices(ctx, priceFilter)
	if err != nil {
		return err
	}

	if len(prices.Items) == 0 {
		return ierr.NewError("no prices found in group for this entity").
			WithHint("Group must contain at least one price belonging to this plan/addon").
			WithReportableDetails(map[string]interface{}{
				"group_id":  groupID,
				"entity_id": entityID,
			}).
			Mark(ierr.ErrValidation)
	}

	return nil
}

// Helper to convert entitlement entity type to price entity type
func getPriceEntityType(entType types.EntitlementEntityType) types.PriceEntityType {
	switch entType {
	case types.ENTITLEMENT_ENTITY_TYPE_PLAN:
		return types.PRICE_ENTITY_TYPE_PLAN
	case types.ENTITLEMENT_ENTITY_TYPE_ADDON:
		return types.PRICE_ENTITY_TYPE_ADDON
	default:
		return types.PRICE_ENTITY_TYPE_PLAN
	}
}
```

#### Add New Helper Function

```go
// GetPlanGroupEntitlements retrieves entitlements for specific price groups within a plan
func (s *entitlementService) GetPlanGroupEntitlements(
	ctx context.Context,
	planID string,
	groupIDs []string,
) (*dto.ListEntitlementsResponse, error) {
	if planID == "" {
		return nil, ierr.NewError("plan_id is required").
			WithHint("Plan ID must be provided").
			Mark(ierr.ErrValidation)
	}

	if len(groupIDs) == 0 {
		return &dto.ListEntitlementsResponse{Items: []*dto.EntitlementResponse{}}, nil
	}

	// Create filter for group-level entitlements
	filter := types.NewNoLimitEntitlementFilter()
	filter.WithEntityIDs([]string{planID})
	filter.WithEntityType(types.ENTITLEMENT_ENTITY_TYPE_PLAN)
	filter.WithPriceGroupIDs(groupIDs)
	filter.WithStatus(types.StatusPublished)
	filter.WithExpand(fmt.Sprintf("%s,%s,%s", types.ExpandFeatures, types.ExpandMeters, types.ExpandGroups))

	return s.ListEntitlements(ctx, filter)
}
```

### 2. File: `internal/service/subscription.go`

#### Update GetSubscriptionEntitlements

```go
// GetSubscriptionEntitlements retrieves all entitlements associated with a subscription
// This includes entitlements from:
// 1. The subscription's plan (plan-level, price_group_id = NULL)
// 2. The subscription's plan with specific price groups (group-level)
// 3. Active addon associations
func (s *subscriptionService) GetSubscriptionEntitlements(ctx context.Context, subscriptionID string) ([]*dto.EntitlementResponse, error) {
	// Get the subscription with line items
	sub, lineItems, err := s.SubRepo.GetWithLineItems(ctx, subscriptionID)
	if err != nil {
		return nil, ierr.WithError(err).
			WithHint("Failed to get subscription").
			Mark(ierr.ErrNotFound)
	}

	entitlementService := NewEntitlementService(s.ServiceParams)

	// Step 1: Get plan-level entitlements (price_group_id = NULL)
	planEntitlements, err := entitlementService.GetPlanEntitlements(ctx, sub.PlanID)
	if err != nil {
		return nil, ierr.WithError(err).
			WithHint("Failed to get plan entitlements").
			Mark(ierr.ErrDatabase)
	}

	// Step 2: Extract unique group IDs from subscription line items
	groupIDs := s.extractGroupIDsFromLineItems(ctx, lineItems)

	// Step 3: Get group-level entitlements if groups exist
	var groupEntitlements *dto.ListEntitlementsResponse
	if len(groupIDs) > 0 {
		groupEntitlements, err = entitlementService.GetPlanGroupEntitlements(ctx, sub.PlanID, groupIDs)
		if err != nil {
			return nil, ierr.WithError(err).
				WithHint("Failed to get group entitlements").
				Mark(ierr.ErrDatabase)
		}
	}

	// Step 4: Get addon entitlements
	addonService := NewAddonService(s.ServiceParams)
	activeAddons, err := addonService.GetActiveAddonAssociation(ctx, dto.GetActiveAddonAssociationRequest{
		EntityID:   subscriptionID,
		EntityType: types.AddonAssociationEntityTypeSubscription,
		StartDate:  &sub.CurrentPeriodStart,
	})
	if err != nil {
		return nil, ierr.WithError(err).
			WithHint("Failed to get active addon associations").
			Mark(ierr.ErrDatabase)
	}

	addonIDs := lo.Uniq(lo.Map(activeAddons, func(assoc *dto.AddonAssociationResponse, _ int) string {
		return assoc.AddonID
	}))

	var addonEntitlements *dto.ListEntitlementsResponse
	if len(addonIDs) > 0 {
		addonEntFilter := types.NewNoLimitEntitlementFilter().
			WithEntityIDs(addonIDs).
			WithEntityType(types.ENTITLEMENT_ENTITY_TYPE_ADDON).
			WithStatus(types.StatusPublished).
			WithExpand(fmt.Sprintf("%s,%s", types.ExpandFeatures, types.ExpandMeters))

		addonEntitlements, err = entitlementService.ListEntitlements(ctx, addonEntFilter)
		if err != nil {
			return nil, err
		}
	}

	// Step 5: Merge entitlements with priority: group > plan > addon
	allEntitlements := s.mergeEntitlementsWithPriority(
		planEntitlements.Items,
		groupEntitlements.Items,
		addonEntitlements.Items,
	)

	return allEntitlements, nil
}

// NEW: Helper to extract group IDs from line items
func (s *subscriptionService) extractGroupIDsFromLineItems(
	ctx context.Context,
	lineItems []*subscription.SubscriptionLineItem,
) []string {
	if len(lineItems) == 0 {
		return []string{}
	}

	// Get unique price IDs
	priceIDs := lo.Uniq(lo.Map(lineItems, func(item *subscription.SubscriptionLineItem, _ int) string {
		return item.PriceID
	}))

	// Fetch prices
	priceService := NewPriceService(s.ServiceParams)
	priceFilter := types.NewNoLimitPriceFilter().
		WithPriceIDs(priceIDs).
		WithAllowExpiredPrices(true)

	prices, err := priceService.GetPrices(ctx, priceFilter)
	if err != nil {
		s.Logger.Warnw("failed to fetch prices for group extraction", "error", err)
		return []string{}
	}

	// Extract group IDs
	groupIDs := make([]string, 0)
	for _, p := range prices.Items {
		if p.GroupID != "" {
			groupIDs = append(groupIDs, p.GroupID)
		}
	}

	return lo.Uniq(groupIDs)
}

// NEW: Merge entitlements with priority
func (s *subscriptionService) mergeEntitlementsWithPriority(
	planEnts []*dto.EntitlementResponse,
	groupEnts []*dto.EntitlementResponse,
	addonEnts []*dto.EntitlementResponse,
) []*dto.EntitlementResponse {
	// Create map by (feature_id, price_group_id) for deduplication
	entitlementMap := make(map[string]*dto.EntitlementResponse)

	// Priority 1: Plan-level entitlements (price_group_id = NULL)
	for _, ent := range planEnts {
		key := fmt.Sprintf("%s::", ent.FeatureID) // :: indicates no group
		entitlementMap[key] = ent
	}

	// Priority 2: Group-level entitlements (override plan-level for same feature)
	for _, ent := range groupEnts {
		key := fmt.Sprintf("%s:%s", ent.FeatureID, ent.PriceGroupID)
		entitlementMap[key] = ent

		// Also override plan-level if group is more specific
		planKey := fmt.Sprintf("%s::", ent.FeatureID)
		if _, exists := entitlementMap[planKey]; exists {
			// Group-level takes precedence, remove plan-level
			delete(entitlementMap, planKey)
		}
	}

	// Priority 3: Addon entitlements (additive, don't override)
	for _, ent := range addonEnts {
		key := fmt.Sprintf("%s:addon:%s", ent.FeatureID, ent.EntityID)
		entitlementMap[key] = ent
	}

	// Convert map to slice
	result := make([]*dto.EntitlementResponse, 0, len(entitlementMap))
	for _, ent := range entitlementMap {
		result = append(result, ent)
	}

	return result
}
```

### 3. File: `internal/service/billing.go`

#### Update CalculateUsageCharges

```go
func (s *billingService) CalculateUsageCharges(
	ctx context.Context,
	sub *subscription.Subscription,
	usage *dto.GetUsageBySubscriptionResponse,
	periodStart,
	periodEnd time.Time,
) ([]dto.CreateInvoiceLineItemRequest, decimal.Decimal, error) {

	if usage == nil {
		return nil, decimal.Zero, nil
	}

	usageCharges := make([]dto.CreateInvoiceLineItemRequest, 0)
	totalUsageCost := decimal.Zero

	// Get aggregated entitlements
	subscriptionService := NewSubscriptionService(s.ServiceParams)
	aggregatedEntitlements, err := subscriptionService.GetAggregatedSubscriptionEntitlements(ctx, sub.ID, nil)
	if err != nil {
		return nil, decimal.Zero, err
	}

	// NEW: Build price lookup map with group information
	priceService := NewPriceService(s.ServiceParams)
	priceMap := make(map[string]*price.Price)
	priceIDs := lo.Map(sub.LineItems, func(item *subscription.SubscriptionLineItem, _ int) string {
		return item.PriceID
	})
	priceFilter := types.NewNoLimitPriceFilter().
		WithPriceIDs(priceIDs).
		WithAllowExpiredPrices(true)
	prices, err := priceService.GetPrices(ctx, priceFilter)
	if err != nil {
		return nil, decimal.Zero, err
	}
	for _, p := range prices.Items {
		priceMap[p.ID] = p.Price
	}

	// NEW: Map aggregated entitlements by (meter_id, group_id) for efficient lookup
	type EntitlementKey struct {
		MeterID  string
		GroupID  string
	}
	entitlementsByKey := make(map[EntitlementKey]*dto.AggregatedEntitlement)

	for _, feature := range aggregatedEntitlements.Features {
		if feature.Feature != nil && types.FeatureType(feature.Feature.Type) == types.FeatureTypeMetered &&
			feature.Feature.MeterID != "" && feature.Entitlement != nil {
			
			key := EntitlementKey{
				MeterID: feature.Feature.MeterID,
				GroupID: feature.Entitlement.PriceGroupID,
			}
			entitlementsByKey[key] = feature.Entitlement
		}
	}

	// Process line items
	for _, item := range sub.LineItems {
		if item.PriceType != types.PRICE_TYPE_USAGE {
			continue
		}

		// Find matching usage charges
		var matchingCharges []*dto.SubscriptionUsageByMetersResponse
		for _, charge := range usage.Charges {
			if charge.Price.ID == item.PriceID {
				matchingCharges = append(matchingCharges, charge)
			}
		}

		if len(matchingCharges) == 0 {
			continue
		}

		// Get price for this line item
		price, ok := priceMap[item.PriceID]
		if !ok {
			s.Logger.Warnw("price not found for line item", "price_id", item.PriceID)
			continue
		}

		// NEW: Find applicable entitlement considering group scope
		matchingEntitlement := s.findApplicableEntitlement(
			entitlementsByKey,
			item.MeterID,
			price.GroupID,
		)

		// Process each matching charge
		for _, matchingCharge := range matchingCharges {
			quantityForCalculation := decimal.NewFromFloat(matchingCharge.Quantity)

			// Apply entitlement if found
			if !matchingCharge.IsOverage && matchingEntitlement != nil && matchingEntitlement.IsEnabled {
				if matchingEntitlement.UsageLimit != nil {
					// Apply usage limit logic based on allocation mode
					if matchingEntitlement.AllocationMode == string(types.ENTITLEMENT_ALLOCATION_MODE_POOLED) {
						// Pooled: Track usage at group level
						// TODO: Implement pooled tracking
						quantityForCalculation = s.calculatePooledUsage(
							ctx, sub, item, matchingEntitlement, matchingCharge, periodStart, periodEnd,
						)
					} else {
						// Per-price: Track usage at price level (existing behavior)
						quantityForCalculation = s.calculatePerPriceUsage(
							ctx, sub, item, matchingEntitlement, matchingCharge, periodStart, periodEnd,
						)
					}

					// Recalculate amount
					if matchingCharge.Price != nil {
						adjustedAmount := priceService.CalculateCost(ctx, matchingCharge.Price, quantityForCalculation)
						matchingCharge.Amount = adjustedAmount.InexactFloat64()
					}
				} else {
					// Unlimited usage
					quantityForCalculation = decimal.Zero
					matchingCharge.Amount = 0
				}
			}

			// Add to invoice line items
			lineItemAmount := decimal.NewFromFloat(matchingCharge.Amount)
			totalUsageCost = totalUsageCost.Add(lineItemAmount)

			// ... rest of line item creation ...
		}
	}

	return usageCharges, totalUsageCost, nil
}

// NEW: Helper to find applicable entitlement
func (s *billingService) findApplicableEntitlement(
	entitlementsByKey map[EntitlementKey]*dto.AggregatedEntitlement,
	meterID string,
	priceGroupID string,
) *dto.AggregatedEntitlement {
	// Priority 1: Try group-level entitlement (if price has group)
	if priceGroupID != "" {
		key := EntitlementKey{
			MeterID: meterID,
			GroupID: priceGroupID,
		}
		if ent, ok := entitlementsByKey[key]; ok {
			return ent
		}
	}

	// Priority 2: Fall back to plan-level entitlement (no group)
	key := EntitlementKey{
		MeterID: meterID,
		GroupID: "",
	}
	if ent, ok := entitlementsByKey[key]; ok {
		return ent
	}

	return nil
}

// NEW: Calculate pooled usage
func (s *billingService) calculatePooledUsage(
	ctx context.Context,
	sub *subscription.Subscription,
	item *subscription.SubscriptionLineItem,
	entitlement *dto.AggregatedEntitlement,
	charge *dto.SubscriptionUsageByMetersResponse,
	periodStart, periodEnd time.Time,
) decimal.Decimal {
	// TODO: Implement pooled tracking logic
	// This is complex and requires tracking usage across all prices in the group
	
	// For now, use existing logic (same as billing period reset)
	if entitlement.UsageResetPeriod == types.EntitlementUsageResetPeriod(sub.BillingPeriod) {
		usageAllowed := decimal.NewFromFloat(float64(*entitlement.UsageLimit))
		adjustedQuantity := decimal.NewFromFloat(charge.Quantity).Sub(usageAllowed)
		return decimal.Max(adjustedQuantity, decimal.Zero)
	}
	
	return decimal.NewFromFloat(charge.Quantity)
}

// NEW: Calculate per-price usage
func (s *billingService) calculatePerPriceUsage(
	ctx context.Context,
	sub *subscription.Subscription,
	item *subscription.SubscriptionLineItem,
	entitlement *dto.AggregatedEntitlement,
	charge *dto.SubscriptionUsageByMetersResponse,
	periodStart, periodEnd time.Time,
) decimal.Decimal {
	// Use existing per-price logic (billing period reset)
	if entitlement.UsageResetPeriod == types.EntitlementUsageResetPeriod(sub.BillingPeriod) {
		usageAllowed := decimal.NewFromFloat(float64(*entitlement.UsageLimit))
		adjustedQuantity := decimal.NewFromFloat(charge.Quantity).Sub(usageAllowed)
		return decimal.Max(adjustedQuantity, decimal.Zero)
	}
	
	return decimal.NewFromFloat(charge.Quantity)
}
```

#### Update AggregateEntitlements

```go
func (s *billingService) AggregateEntitlements(entitlements []*dto.EntitlementResponse, subscriptionID string) []*dto.AggregatedFeature {
	// Map to store entitlements by feature ID
	featureIDs := make([]string, 0)
	entitlementsByFeature := make(map[string][]*dto.EntitlementResponse)
	sourcesByFeature := make(map[string][]*dto.EntitlementSource)

	// Process each entitlement
	for _, ent := range entitlements {
		// Skip disabled entitlements
		if !ent.IsEnabled || ent.Status != types.StatusPublished {
			continue
		}

		featureIDs = append(featureIDs, ent.FeatureID)

		if _, ok := entitlementsByFeature[ent.FeatureID]; !ok {
			entitlementsByFeature[ent.FeatureID] = make([]*dto.EntitlementResponse, 0)
			sourcesByFeature[ent.FeatureID] = make([]*dto.EntitlementSource, 0)
		}

		entitlementsByFeature[ent.FeatureID] = append(entitlementsByFeature[ent.FeatureID], ent)

		// Create source for this entitlement
		entityType := dto.EntitlementSourceEntityTypePlan
		entityName := ""

		if ent.EntityType == types.ENTITLEMENT_ENTITY_TYPE_PLAN {
			entityType = dto.EntitlementSourceEntityTypePlan
			if ent.Plan != nil {
				entityName = ent.Plan.Name
			}
		} else if ent.EntityType == types.ENTITLEMENT_ENTITY_TYPE_ADDON {
			entityType = dto.EntitlementSourceEntityTypeAddon
			if ent.Addon != nil {
				entityName = ent.Addon.Name
			}
		}

		source := &dto.EntitlementSource{
			SubscriptionID: subscriptionID,
			EntityID:       ent.EntityID,
			EntityType:     entityType,
			EntityName:     entityName,
			Quantity:       1,
			EntitlementID:  ent.ID,
			IsEnabled:      ent.IsEnabled,
			UsageLimit:     ent.UsageLimit,
			StaticValue:    ent.StaticValue,
			PriceGroupID:   ent.PriceGroupID,  // NEW
		}

		// NEW: Add group name if present
		if ent.Group != nil {
			source.GroupName = ent.Group.Name
		}

		sourcesByFeature[ent.FeatureID] = append(sourcesByFeature[ent.FeatureID], source)
	}

	featureIDs = lo.Uniq(featureIDs)

	// Aggregate entitlements by feature
	aggregatedFeatures := make([]*dto.AggregatedFeature, 0, len(featureIDs))

	for _, featureID := range featureIDs {
		entResponses := entitlementsByFeature[featureID]
		if len(entResponses) == 0 {
			continue
		}

		if entResponses[0].Feature == nil {
			continue
		}

		featureResponse := entResponses[0].Feature

		// Convert to domain entitlements for aggregation
		domainEntitlements := make([]*entitlement.Entitlement, 0, len(entResponses))
		for _, entResp := range entResponses {
			domainEnt := &entitlement.Entitlement{
				ID:               entResp.ID,
				EntityType:       types.EntitlementEntityType(entResp.EntityType),
				EntityID:         entResp.EntityID,
				FeatureID:        entResp.FeatureID,
				FeatureType:      types.FeatureType(entResp.FeatureType),
				IsEnabled:        entResp.IsEnabled,
				UsageLimit:       entResp.UsageLimit,
				UsageResetPeriod: types.EntitlementUsageResetPeriod(entResp.UsageResetPeriod),
				IsSoftLimit:      entResp.IsSoftLimit,
				StaticValue:      entResp.StaticValue,
				PriceGroupID:     entResp.PriceGroupID,     // NEW
				AllocationMode:   types.AllocationMode(entResp.AllocationMode), // NEW
			}
			domainEntitlements = append(domainEntitlements, domainEnt)
		}

		// Aggregate based on feature type
		var aggregatedEntitlement *dto.AggregatedEntitlement
		switch types.FeatureType(featureResponse.Type) {
		case types.FeatureTypeMetered:
			aggregatedEntitlement = aggregateMeteredEntitlementsForBilling(domainEntitlements)
		case types.FeatureTypeBoolean:
			aggregatedEntitlement = aggregateBooleanEntitlementsForBilling(domainEntitlements)
		case types.FeatureTypeStatic:
			aggregatedEntitlement = aggregateStaticEntitlementsForBilling(domainEntitlements)
		default:
			continue
		}

		// NEW: Set allocation mode from first entitlement (they should all be same)
		if len(domainEntitlements) > 0 {
			aggregatedEntitlement.AllocationMode = string(domainEntitlements[0].AllocationMode)
			aggregatedEntitlement.PriceGroupID = domainEntitlements[0].PriceGroupID
		}

		aggregatedFeature := &dto.AggregatedFeature{
			Feature:     featureResponse,
			Entitlement: aggregatedEntitlement,
			Sources:     sourcesByFeature[featureID],
		}

		aggregatedFeatures = append(aggregatedFeatures, aggregatedFeature)
	}

	return aggregatedFeatures
}
```

---

## API Changes

### File: `internal/api/v1/entitlement.go`

No changes needed to HTTP handlers - DTOs handle the new fields automatically.

The API will automatically support:

**Create Entitlement with Group:**
```json
POST /api/v1/entitlements
{
  "entity_type": "PLAN",
  "entity_id": "plan_123",
  "price_group_id": "grp_456",
  "feature_id": "feat_789",
  "feature_type": "METERED",
  "is_enabled": true,
  "usage_limit": 10000,
  "usage_reset_period": "BILLING_PERIOD",
  "allocation_mode": "pooled"
}
```

**Response includes group info:**
```json
{
  "id": "ent_abc",
  "entity_type": "PLAN",
  "entity_id": "plan_123",
  "price_group_id": "grp_456",
  "feature_id": "feat_789",
  "allocation_mode": "pooled",
  "group": {
    "id": "grp_456",
    "name": "Premium API Tiers",
    "entity_type": "price"
  },
  ...
}
```

---

## Testing Strategy

### Unit Tests

**File: `internal/service/entitlement_test.go`**

```go
func TestCreateEntitlementWithGroup(t *testing.T) {
	// Test creating entitlement with price_group_id
}

func TestValidatePriceGroupExists(t *testing.T) {
	// Test group validation
}

func TestValidatePriceGroupHasPrices(t *testing.T) {
	// Test that group has prices in plan
}

func TestCreateEntitlementWithInvalidGroup(t *testing.T) {
	// Test error handling for invalid groups
}
```

**File: `internal/service/subscription_test.go`**

```go
func TestGetSubscriptionEntitlementsWithGroups(t *testing.T) {
	// Test retrieval of group entitlements
}

func TestMergeEntitlementsWithPriority(t *testing.T) {
	// Test: group > plan > addon priority
}

func TestExtractGroupIDsFromLineItems(t *testing.T) {
	// Test group ID extraction
}
```

**File: `internal/service/billing_test.go`**

```go
func TestCalculateUsageChargesWithGroupEntitlements(t *testing.T) {
	// Test usage calculation with group entitlements
}

func TestFindApplicableEntitlement(t *testing.T) {
	// Test: should prefer group-level over plan-level
}

func TestPooledAllocationMode(t *testing.T) {
	// Test pooled limit sharing across group
}

func TestPerPriceAllocationMode(t *testing.T) {
	// Test individual limits per price
}
```

### Integration Tests

**File: `internal/service/integration/entitlement_integration_test.go`**

```go
func TestEndToEndGroupEntitlement(t *testing.T) {
	// Create plan → group → prices → entitlement
	// Create subscription
	// Verify entitlements are applied correctly
	// Test billing with usage
}
```

### Manual Testing Checklist

- [ ] Create price group with multiple prices
- [ ] Create group-level entitlement
- [ ] Create subscription with prices from group
- [ ] Verify entitlement shows in subscription
- [ ] Submit usage events
- [ ] Verify billing applies entitlement correctly
- [ ] Test pooled vs per-price allocation
- [ ] Test priority (group overrides plan)
- [ ] Test backward compatibility (NULL group_id)

---

## Rollout Plan

### Phase 1: Database Migration (Week 1)
- [ ] Run migration on staging
- [ ] Verify schema changes
- [ ] Test rollback
- [ ] Run migration on production

### Phase 2: Code Deployment (Week 2)
- [ ] Deploy backend changes
- [ ] Monitor logs for errors
- [ ] Verify backward compatibility
- [ ] Test creating group entitlements

### Phase 3: Feature Enable (Week 3)
- [ ] Enable for internal testing
- [ ] Create test scenarios
- [ ] Enable for beta customers
- [ ] Collect feedback

### Phase 4: Full Rollout (Week 4)
- [ ] Enable for all customers
- [ ] Update documentation
- [ ] Announce feature

---

## Backward Compatibility

### Existing Entitlements
- All existing entitlements have `price_group_id = NULL`
- They continue to work as plan-level entitlements
- No data migration needed

### API Compatibility
- `price_group_id` is optional in requests
- Omitting it creates plan-level entitlement (existing behavior)
- Old API calls work without modification

### Database Compatibility
- NULL values handled correctly in queries
- Unique constraint includes `COALESCE(price_group_id, '')`
- Indexes optimized for both NULL and non-NULL cases

---

## Monitoring & Observability

### Metrics to Track
```go
// Add metrics in service layer
metrics.Counter("entitlements.created.with_group")
metrics.Counter("entitlements.created.plan_level")
metrics.Histogram("billing.entitlement_lookup_duration")
```

### Logging
```go
s.Logger.Infow("applying group entitlement",
    "subscription_id", sub.ID,
    "price_group_id", entitlement.PriceGroupID,
    "allocation_mode", entitlement.AllocationMode,
    "usage_limit", entitlement.UsageLimit,
)
```

### Alerts
- Alert if entitlement lookup time > 100ms
- Alert if group validation fails > 5% of requests
- Alert if pooled tracking logic fails

---

## Documentation Updates

### Files to Update
1. `api/README.md` - Add group entitlement examples
2. `docs/entitlements.md` - Document group behavior
3. `docs/api-reference.md` - Update API docs
4. Swagger/OpenAPI spec - Add new fields

### Example Documentation

```markdown
## Group-Level Entitlements

Entitlements can now be scoped to specific price groups within a plan.

### Use Case
Apply different entitlement limits to different tiers of pricing within the same plan.

### Example
```json
{
  "entity_type": "PLAN",
  "entity_id": "plan_enterprise",
  "price_group_id": "grp_premium_api",
  "feature_id": "feat_api_calls",
  "usage_limit": 10000,
  "allocation_mode": "pooled"
}
```

### Allocation Modes
- **pooled** (default): Limit is shared across all prices in the group
- **per_price**: Each price gets its own separate limit
```

---

## Future Enhancements

### Phase 2 Features
1. **Usage Tracking Dashboard**: Show pooled usage across groups
2. **Group-level Analytics**: Usage patterns by price group
3. **Dynamic Limits**: Adjust limits based on group performance
4. **Cascading Entitlements**: Inherit and override from group hierarchies

### Performance Optimizations
1. Cache group entitlements per subscription
2. Pre-compute entitlement maps at subscription creation
3. Background job to refresh entitlement caches

---

## Risk Assessment

### High Risk Areas
1. **Billing Calculation**: Complex logic with pooled tracking
2. **Migration**: Large entitlement tables
3. **Query Performance**: Additional JOINs and filters

### Mitigation Strategies
1. Extensive testing with real data
2. Gradual rollout with feature flags
3. Database query optimization
4. Rollback plan ready

---

## Success Criteria

### Technical Success
- [ ] Zero data loss during migration
- [ ] No performance degradation in billing
- [ ] All tests passing
- [ ] Backward compatibility maintained

### Business Success
- [ ] Customers can create group entitlements
- [ ] Billing correctly applies group limits
- [ ] Support tickets < 5 for feature issues
- [ ] Adoption rate > 20% within 3 months

---

## Appendix

### Related Files
- Schema: `ent/schema/entitlement.go`
- Service: `internal/service/entitlement.go`
- Service: `internal/service/subscription.go`
- Service: `internal/service/billing.go`
- Types: `internal/types/entitlement.go`
- DTO: `internal/api/dto/entitlement.go`

### References
- Original discussion: [Link to discussion]
- Design doc: [Link to design]
- API spec: [Link to OpenAPI spec]

---

**Document Version:** 1.0  
**Last Updated:** 2024-11-04  
**Status:** Draft  
**Owner:** Engineering Team

