package subscription

import (
	"context"
	"time"

	"github.com/flexprice/flexprice/internal/service"
	subscriptionModels "github.com/flexprice/flexprice/internal/temporal/models/subscription"
	temporalService "github.com/flexprice/flexprice/internal/temporal/service"
	"github.com/flexprice/flexprice/internal/types"
	"github.com/samber/lo"
)

const ActivityPrefix = "SubscriptionActivities"

const (
	// Workflow name - must match the function name
	WorkflowProcessSubscriptionBillingPeriodUpdate = "ProcessSubscriptionBillingPeriodUpdateWorkflow"
)

// SubscriptionActivities contains all subscription-related activities
// When registered with Temporal, methods will be called as "SubscriptionActivities.ScheduleSubscriptionUpdateBillingPeriod"
type SubscriptionActivities struct {
	subscriptionService service.SubscriptionService
}

// NewPlanActivities creates a new PlanActivities instance
func NewSubscriptionActivities(subscriptionService service.SubscriptionService) *SubscriptionActivities {
	return &SubscriptionActivities{
		subscriptionService: subscriptionService,
	}
}

// SyncPlanPrices syncs plan prices
// This method will be registered as "SyncPlanPrices" in Temporal
func (s *SubscriptionActivities) ScheduleSubscriptionUpdateBillingPeriod(ctx context.Context, input subscriptionModels.ScheduleSubscriptionUpdateBillingPeriodWorkflowInput) (*subscriptionModels.ScheduleSubscriptionUpdateBillingPeriodWorkflowResult, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}

	now := time.Now().UTC()

	response := &subscriptionModels.ScheduleSubscriptionUpdateBillingPeriodWorkflowResult{
		SubscriptionIDs: make([]string, 0),
	}

	offset := 0
	for {
		filter := &types.SubscriptionFilter{
			QueryFilter: &types.QueryFilter{
				Limit:  lo.ToPtr(input.BatchSize),
				Offset: lo.ToPtr(offset),
				Status: lo.ToPtr(types.StatusPublished),
			},
			SubscriptionStatus: []types.SubscriptionStatus{
				types.SubscriptionStatusActive,
				types.SubscriptionStatusTrialing,
			},
			TimeRangeFilter: &types.TimeRangeFilter{
				EndTime: &now,
			},
		}

		subs, err := s.subscriptionService.ListAllTenantSubscriptions(ctx, filter)
		if err != nil {
			return response, err
		}

		temporalSvc := temporalService.GetGlobalTemporalService()
		for _, sub := range subs.Items {
			// update context to include the tenant id
			ctx = context.WithValue(ctx, types.CtxTenantID, sub.TenantID)
			ctx = context.WithValue(ctx, types.CtxEnvironmentID, sub.EnvironmentID)
			ctx = context.WithValue(ctx, types.CtxUserID, sub.CreatedBy)

			// Here we need to launch a new workflow to update the billing period

			_, err := temporalSvc.ExecuteWorkflow(
				ctx,
				WorkflowProcessSubscriptionBillingPeriodUpdate,
				subscriptionModels.ProcessSubscriptionUpdateBillingPeriodWorkflowInput{
					SubscriptionID: sub.ID,
					TenantID:       sub.TenantID,
					EnvironmentID:  sub.EnvironmentID,
					UserID:         sub.CreatedBy,
					PeriodStart:    sub.CurrentPeriodStart,
					PeriodEnd:      sub.CurrentPeriodEnd,
				},
			)
			if err != nil {
				return response, err
			}

			response.SubscriptionIDs = append(response.SubscriptionIDs, sub.ID)
		}
		offset += len(subs.Items)
		if len(subs.Items) < input.BatchSize {
			break // No more subscriptions to fetch
		}
	}

	return response, nil
}
