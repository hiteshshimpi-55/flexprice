# Trial Period Handling in Temporal Billing Period Update Workflow

## Document Status
**Status**: Draft for Review  
**Created**: December 2, 2025  
**Author**: AI Assistant  
**Reviewers**: To be assigned

---

## 1. Executive Summary

This document outlines the implementation strategy for adding trial period handling to the Temporal-based billing period update workflow. Currently, the system has trial period fields (`TrialStart`, `TrialEnd`) in subscriptions but does not handle them during billing period processing, leading to incorrect invoice generation and subscription status management during trial periods.

### Key Goals
1. **Prevent invoice generation during trial periods**
2. **Automatically transition subscriptions from `trialing` to `active` status when trial ends**
3. **Handle trial period expiration in the billing period calculation**
4. **Maintain backward compatibility with existing non-trial subscriptions**

---

## 2. Current State Analysis

### 2.1 Existing Trial Infrastructure

**Subscription Model** (`internal/domain/subscription/model.go`):
```go
// Lines 68-72
TrialStart *time.Time `db:"trial_start" json:"trial_start"`
TrialEnd   *time.Time `db:"trial_end"   json:"trial_end"`
```

**Subscription Status Types** (`internal/types/subscription.go`):
```go
// Line 31
SubscriptionStatusTrialing SubscriptionStatus = "trialing"
```

**Line Item Trial Period** (from Price model):
```go
TrialPeriod int // Number of days for trial period
```

### 2.2 Current Workflow Flow

The `ProcessSubscriptionBillingPeriodUpdateWorkflow` currently executes in this order:

1. **CheckSubscriptionPauseStatusActivity** - Handles pause/resume logic
2. **CalculatePeriodsActivity** - Calculates billing periods
3. **ProcessPeriodsActivity** - Creates invoices for all periods
4. **UpdateSubscriptionPeriodActivity** - Updates subscription to new period
5. **CheckSubscriptionCancellationActivity** - Checks for cancellation
6. **SyncInvoiceToExternalVendorActivity** - Syncs invoices to Stripe, etc.
7. **AttemptPaymentActivity** - Attempts payment collection

### 2.3 Identified Gaps

1. **No trial status filtering**: The cron processes subscriptions with `status = 'active'` but should also process `status = 'trialing'`
2. **No trial period awareness**: Invoice creation doesn't check if the period overlaps with trial
3. **No automatic status transition**: No logic to transition from `trialing` to `active` when trial ends
4. **Billing period calculation ignores trial**: Periods are calculated without considering trial boundaries

---

## 3. Proposed Solution

### 3.1 High-Level Approach

We will implement trial handling at **three strategic points** in the workflow:

1. **Pre-flight check**: Add a new activity to check trial status before processing
2. **Period calculation**: Adjust billing period calculation to account for trial end
3. **Invoice creation**: Skip invoice creation for periods that fall within trial

### 3.2 Design Principles

- ✅ **Fail-safe**: Errors in trial handling should not break existing billing
- ✅ **Explicit**: Trial logic should be clearly separated and testable
- ✅ **Observable**: Add comprehensive logging for trial transitions
- ✅ **Idempotent**: Multiple runs should produce same result
- ✅ **Backward compatible**: Non-trial subscriptions work exactly as before

---

## 4. Detailed Implementation Plan

### 4.1 Phase 1: Add Trial Status Check Activity

**New Activity**: `CheckSubscriptionTrialStatusActivity`

**Location**: `internal/temporal/activities/subscription/update_billing_period_activities.go`

**Purpose**: 
- Check if subscription is in trial
- Handle trial-to-active transition when trial ends
- Determine if billing should be skipped

**Input Model**:
```go
type CheckSubscriptionTrialStatusActivityInput struct {
    SubscriptionID string    `json:"subscription_id"`
    TenantID       string    `json:"tenant_id"`
    EnvironmentID  string    `json:"environment_id"`
    CurrentTime    time.Time `json:"current_time"`
}
```

**Output Model**:
```go
type CheckSubscriptionTrialStatusActivityOutput struct {
    IsInTrial              bool       `json:"is_in_trial"`
    TrialEnded             bool       `json:"trial_ended"`
    TrialEndDate           *time.Time `json:"trial_end_date,omitempty"`
    ShouldSkipBilling      bool       `json:"should_skip_billing"`
    StatusTransitioned     bool       `json:"status_transitioned"` // trialing -> active
    NewCurrentPeriodStart  *time.Time `json:"new_current_period_start,omitempty"`
    NewCurrentPeriodEnd    *time.Time `json:"new_current_period_end,omitempty"`
}
```

**Logic**:
```go
func (s *UpdateBillingPeriodActivities) CheckSubscriptionTrialStatusActivity(
    ctx context.Context,
    input CheckSubscriptionTrialStatusActivityInput,
) (*CheckSubscriptionTrialStatusActivityOutput, error) {
    // 1. Get subscription
    sub, err := s.serviceParams.SubRepo.Get(ctx, input.SubscriptionID)
    
    output := &CheckSubscriptionTrialStatusActivityOutput{
        IsInTrial:         false,
        TrialEnded:        false,
        ShouldSkipBilling: false,
        StatusTransitioned: false,
    }
    
    // 2. Check if subscription has trial period
    if sub.TrialStart == nil || sub.TrialEnd == nil {
        // No trial, proceed with normal billing
        return output, nil
    }
    
    now := input.CurrentTime
    
    // 3. Check trial status
    if now.Before(*sub.TrialEnd) {
        // Still in trial period
        output.IsInTrial = true
        output.TrialEndDate = sub.TrialEnd
        output.ShouldSkipBilling = true
        
        // Ensure subscription status is 'trialing'
        if sub.SubscriptionStatus != types.SubscriptionStatusTrialing {
            sub.SubscriptionStatus = types.SubscriptionStatusTrialing
            if err := s.serviceParams.SubRepo.Update(ctx, sub); err != nil {
                return nil, err
            }
            s.logger.Infow("corrected subscription status to trialing",
                "subscription_id", sub.ID,
                "trial_end", sub.TrialEnd)
        }
        
        return output, nil
    }
    
    // 4. Trial has ended - transition to active
    output.TrialEnded = true
    output.TrialEndDate = sub.TrialEnd
    
    if sub.SubscriptionStatus == types.SubscriptionStatusTrialing {
        // Transition from trialing to active
        sub.SubscriptionStatus = types.SubscriptionStatusActive
        
        // Set billing period to start from trial end date
        // This ensures first invoice is generated from trial end
        sub.CurrentPeriodStart = *sub.TrialEnd
        nextEnd, err := types.NextBillingDate(
            *sub.TrialEnd,
            sub.BillingAnchor,
            sub.BillingPeriodCount,
            sub.BillingPeriod,
            sub.EndDate,
        )
        if err != nil {
            return nil, err
        }
        sub.CurrentPeriodEnd = nextEnd
        
        if err := s.serviceParams.SubRepo.Update(ctx, sub); err != nil {
            return nil, err
        }
        
        output.StatusTransitioned = true
        output.NewCurrentPeriodStart = &sub.CurrentPeriodStart
        output.NewCurrentPeriodEnd = &sub.CurrentPeriodEnd
        
        s.logger.Infow("transitioned subscription from trial to active",
            "subscription_id", sub.ID,
            "trial_end", sub.TrialEnd,
            "new_period_start", sub.CurrentPeriodStart,
            "new_period_end", sub.CurrentPeriodEnd)
    }
    
    // After transition, continue with normal billing
    output.ShouldSkipBilling = false
    return output, nil
}
```

### 4.2 Phase 2: Update Workflow to Check Trial Status

**File**: `internal/temporal/workflows/subscription/process_subscription_billing_period_update_workflow.go`

**Changes**:

1. Add new activity constant:
```go
const (
    // ... existing constants ...
    ActivityCheckSubscriptionTrialStatus = "CheckSubscriptionTrialStatusActivity"
)
```

2. Insert trial check **after** pause check, **before** period calculation:

```go
// ================================================================================
// STEP 1.5: Check Subscription Trial Status
// ================================================================================
logger.Info("Step 1.5: Checking subscription trial status",
    "subscription_id", input.SubscriptionID)

var trialStatusOutput subscriptionModels.CheckSubscriptionTrialStatusActivityOutput
trialStatusInput := subscriptionModels.CheckSubscriptionTrialStatusActivityInput{
    SubscriptionID: input.SubscriptionID,
    TenantID:       input.TenantID,
    EnvironmentID:  input.EnvironmentID,
    CurrentTime:    now,
}

err = workflow.ExecuteActivity(ctx, ActivityCheckSubscriptionTrialStatus, trialStatusInput).Get(ctx, &trialStatusOutput)
if err != nil {
    logger.Error("Failed to check subscription trial status",
        "error", err,
        "subscription_id", input.SubscriptionID)
    errorMsg := err.Error()
    return &subscriptionModels.ProcessSubscriptionUpdateBillingPeriodWorkflowResult{
        Success:     false,
        Error:       &errorMsg,
        CompletedAt: workflow.Now(ctx),
    }, nil
}

// If in trial, skip billing processing
if trialStatusOutput.ShouldSkipBilling {
    logger.Info("Skipping billing period processing - subscription in trial",
        "subscription_id", input.SubscriptionID,
        "is_in_trial", trialStatusOutput.IsInTrial,
        "trial_end_date", trialStatusOutput.TrialEndDate)
    return &subscriptionModels.ProcessSubscriptionUpdateBillingPeriodWorkflowResult{
        Success:     true,
        CompletedAt: workflow.Now(ctx),
    }, nil
}

// Log if trial just ended and status was transitioned
if trialStatusOutput.StatusTransitioned {
    logger.Info("Subscription trial ended - transitioned to active",
        "subscription_id", input.SubscriptionID,
        "trial_end_date", trialStatusOutput.TrialEndDate,
        "new_period_start", trialStatusOutput.NewCurrentPeriodStart,
        "new_period_end", trialStatusOutput.NewCurrentPeriodEnd)
}

// Continue with normal billing flow...
```

### 4.3 Phase 3: Register New Activity

**File**: `internal/temporal/registration.go`

**Changes**:

Add activity to registration list around line 141-149:

```go
case types.TemporalTaskQueueSubscription:
    workflowsList = append(
        workflowsList,
        subscriptionWorkflows.ScheduleSubscriptionUpdateBillingPeriodWorkflow,
        subscriptionWorkflows.ProcessSubscriptionBillingPeriodUpdateWorkflow,
    )
    activitiesList = append(activitiesList,
        subscriptionActivities.ScheduleSubscriptionUpdateBillingPeriod,
        updateBillingPeriodActivities.CheckSubscriptionPauseStatusActivity,
        updateBillingPeriodActivities.CheckSubscriptionTrialStatusActivity, // NEW
        updateBillingPeriodActivities.CalculatePeriodsActivity,
        // ... rest of activities ...
    )
```

### 4.4 Phase 4: Update Scheduling to Include Trialing Subscriptions

**File**: `internal/temporal/activities/subscription/schedule_update_billing_period_activities.go`

**Current Filter** (approximately line 40):
```go
SubscriptionStatus: []types.SubscriptionStatus{types.SubscriptionStatusActive},
```

**Change To**:
```go
SubscriptionStatus: []types.SubscriptionStatus{
    types.SubscriptionStatusActive,
    types.SubscriptionStatusTrialing, // Add trialing subscriptions
},
```

This ensures that subscriptions in trial are also picked up by the cron job so they can transition to active when trial ends.

---

## 5. Edge Cases & Considerations

### 5.1 Trial Period Edge Cases

| Scenario | Current Behavior | New Behavior | Implementation Notes |
|----------|-----------------|--------------|---------------------|
| Trial ends mid-period | Invoices created for full period | Skip trial portion, invoice from trial end | Handle in period calculation |
| Trial end = period end | May create invoice | No invoice during trial, first invoice after | Check in invoice creation |
| Subscription created with past trial dates | Status might be wrong | Correct status immediately | Check in trial status activity |
| Trial dates modified after creation | No automatic correction | Detect and handle in next run | Trial status activity handles it |
| Subscription paused during trial | Both pause and trial active | Trial takes precedence | Check trial before pause |
| Subscription cancelled during trial | Cancellation processed | Skip invoicing, handle cancellation | Check cancellation after trial |

### 5.2 Status Transition Matrix

| Current Status | Trial State | Action | New Status |
|---------------|-------------|--------|------------|
| `trialing` | In trial (`now < TrialEnd`) | Skip billing | `trialing` |
| `trialing` | Trial ended (`now >= TrialEnd`) | Transition + start billing | `active` |
| `active` | Has trial dates but already active | Continue billing | `active` |
| `active` | No trial dates | Continue billing | `active` |
| Any other | In trial | Correct status + skip billing | `trialing` |

### 5.3 Billing Period Adjustment

**Scenario**: Subscription created on Jan 1, trial ends Jan 31, monthly billing

```
Timeline:
  Jan 1 (Start Date) -------- Jan 31 (Trial End) -------- Feb 1 -------- Feb 28
  [         TRIAL PERIOD - NO BILLING           ] [  FIRST BILLING PERIOD  ]

CurrentPeriodStart: Jan 1  -> After transition: Jan 31
CurrentPeriodEnd:   Jan 31 -> After transition: Feb 28 (or Feb 29)
```

**Implementation**: When trial ends, set `CurrentPeriodStart = TrialEnd` and calculate next billing date from there.

---

## 6. Testing Strategy

### 6.1 Unit Tests

**Test File**: `internal/temporal/activities/subscription/update_billing_period_activities_test.go`

**Test Cases**:
1. ✅ Subscription with no trial dates - should return `ShouldSkipBilling = false`
2. ✅ Subscription in trial (before TrialEnd) - should return `ShouldSkipBilling = true`, `IsInTrial = true`
3. ✅ Subscription with trial just ended - should transition status, return `StatusTransitioned = true`
4. ✅ Subscription with trial ended long ago - should handle gracefully
5. ✅ Subscription with invalid trial dates (TrialEnd before TrialStart) - should error
6. ✅ Subscription status correction (active but in trial) - should correct to trialing

### 6.2 Integration Tests

**Test File**: `internal/temporal/workflows/subscription/process_subscription_billing_period_update_workflow_test.go`

**Test Scenarios**:
1. ✅ Full workflow with subscription in trial - should skip billing, no invoices created
2. ✅ Full workflow with trial ending - should transition status and create first invoice
3. ✅ Full workflow with non-trial subscription - should work as before (regression test)
4. ✅ Trial ending + pause scheduled - should handle both correctly
5. ✅ Trial ending + cancellation scheduled - should handle both correctly

### 6.3 Manual Testing Checklist

- [ ] Create subscription with 7-day trial, verify status = `trialing`
- [ ] Wait for cron to run during trial, verify no invoices created
- [ ] Manually trigger workflow for trialing subscription, verify skips billing
- [ ] Simulate time passing trial end, verify transition to `active`
- [ ] Verify first invoice created after trial ends
- [ ] Check Stripe sync for subscriptions with trials
- [ ] Verify metrics/logs show trial transitions

---

## 7. Rollout Plan

### 7.1 Deployment Strategy

**Phase 1: Development & Testing** (Week 1)
- [ ] Implement trial status check activity
- [ ] Update workflow to call new activity
- [ ] Add comprehensive unit tests
- [ ] Add integration tests

**Phase 2: Staging Validation** (Week 1-2)
- [ ] Deploy to staging environment
- [ ] Create test subscriptions with various trial configurations
- [ ] Monitor logs for trial transitions
- [ ] Verify no regressions in non-trial subscriptions

**Phase 3: Production Rollout** (Week 2)
- [ ] Deploy to production with feature flag (if available)
- [ ] Monitor logs and metrics closely
- [ ] Run one-time script to correct status for existing trialing subscriptions
- [ ] Verify trial-to-active transitions working correctly

### 7.2 Monitoring & Observability

**Key Metrics to Track**:
- Number of subscriptions in `trialing` status
- Number of trial-to-active transitions per day
- Invoices created (should decrease if many trials)
- Errors in trial status check activity

**Log Queries**:
```
# Find all trial transitions
"transitioned subscription from trial to active"

# Find subscriptions skipped due to trial
"Skipping billing period processing - subscription in trial"

# Find trial status corrections
"corrected subscription status to trialing"
```

### 7.3 Rollback Plan

If issues arise:
1. **Immediate**: Stop the cron scheduler to prevent further processing
2. **Quick fix**: Revert to previous deployment
3. **Data fix**: Run correction script to fix any incorrectly transitioned subscriptions
4. **Investigation**: Analyze logs to identify root cause

---

## 8. Migration & Data Consistency

### 8.1 Existing Subscriptions

**Concern**: Subscriptions might have trial dates but wrong status

**Solution**: One-time migration script to correct status

```sql
-- Find subscriptions with active trial but wrong status
SELECT id, subscription_status, trial_start, trial_end, current_period_start, current_period_end
FROM subscriptions
WHERE trial_end IS NOT NULL
  AND trial_end > NOW()
  AND subscription_status != 'trialing';

-- Correction will happen automatically on next cron run via CheckSubscriptionTrialStatusActivity
```

### 8.2 Backward Compatibility

✅ **Non-trial subscriptions**: No change in behavior - trial fields are `NULL`, activity returns immediately  
✅ **Existing workflows**: Will continue working - new activity is additive  
✅ **API contracts**: No changes to external APIs  
✅ **Database schema**: No schema changes required  

---

## 9. Alternative Approaches Considered

### 9.1 Option A: Handle Trial in Period Calculation (Rejected)
**Pros**: Centralized logic  
**Cons**: Complex to adjust periods mid-calculation, harder to test, less explicit

### 9.2 Option B: Handle Trial in Invoice Creation (Rejected)
**Pros**: Simple to skip invoice  
**Cons**: Doesn't handle status transitions, periods still calculated incorrectly

### 9.3 Option C: Separate Trial Workflow (Rejected)
**Pros**: Complete separation of concerns  
**Cons**: Duplicates logic, harder to maintain, more complex scheduling

### 9.4 **Selected: Option D - Pre-flight Trial Check Activity** ✅
**Pros**: 
- Explicit and testable
- Handles all trial logic in one place
- Easy to understand and maintain
- Minimal changes to existing workflow
- Clear separation of concerns

**Cons**: 
- Adds one more activity call
- Slight increase in workflow execution time (~100-200ms)

---

## 10. Open Questions

1. **Q**: Should we send events/webhooks when trial ends?  
   **A**: *To be decided - recommend yes for customer notification*

2. **Q**: What happens if trial dates are modified while subscription is active?  
   **A**: *Current approach: Ignore trial if status is already active*

3. **Q**: Should trial period affect proration calculations?  
   **A**: *Recommend: Yes, but handle in separate enhancement*

4. **Q**: Should we support extending trial period after creation?  
   **A**: *Recommend: Yes via API, separate from this implementation*

5. **Q**: What if customer has payment method but subscription is in trial?  
   **A**: *Don't charge until trial ends - payment attempted on first invoice after trial*

---

## 11. Success Criteria

### 11.1 Functional Requirements
- ✅ No invoices generated during trial period
- ✅ Automatic transition from `trialing` to `active` when trial ends
- ✅ First invoice generated correctly after trial ends
- ✅ Trial end date aligns with first billing period start
- ✅ Non-trial subscriptions unaffected (regression test passes)

### 11.2 Non-Functional Requirements
- ✅ Performance: Trial check activity completes in < 500ms
- ✅ Reliability: No failed workflows due to trial handling
- ✅ Observability: All trial events logged with context
- ✅ Maintainability: Code is well-documented and testable

---

## 12. Future Enhancements

1. **Trial Period Modification API**: Allow extending/shortening trial via API
2. **Proration Awareness**: Handle proration correctly when trial ends mid-period
3. **Trial Conversion Analytics**: Track trial-to-paid conversion rates
4. **Custom Trial End Actions**: Webhooks, notifications, custom workflows
5. **Per-Line-Item Trials**: Different trial periods for different products
6. **Trial Credit Application**: Apply credits during trial for better UX

---

## 13. References

- **Current Implementation**: `internal/service/subscription.go` (lines 2138-2534)
- **Trial Fields**: `internal/domain/subscription/model.go` (lines 68-72)
- **Status Types**: `internal/types/subscription.go` (line 31)
- **Temporal Workflow**: `internal/temporal/workflows/subscription/process_subscription_billing_period_update_workflow.go`
- **Temporal Activities**: `internal/temporal/activities/subscription/update_billing_period_activities.go`

---

## Appendix A: Code Flow Diagram

```
┌─────────────────────────────────────────────────────────────────────┐
│  ProcessSubscriptionBillingPeriodUpdateWorkflow                      │
└─────────────────────────────────────────────────────────────────────┘
                                 │
                                 ▼
┌─────────────────────────────────────────────────────────────────────┐
│  Step 1: CheckSubscriptionPauseStatusActivity                        │
│  - Handle pause/resume logic                                         │
└─────────────────────────────────────────────────────────────────────┘
                                 │
                                 ▼
┌─────────────────────────────────────────────────────────────────────┐
│  Step 1.5: CheckSubscriptionTrialStatusActivity  ⭐ NEW              │
│  ┌───────────────────────────────────────────────────────────────┐  │
│  │ Is TrialEnd NULL?                                             │  │
│  │   YES → Return (ShouldSkipBilling = false)                    │  │
│  │   NO  → Continue                                              │  │
│  └───────────────────────────────────────────────────────────────┘  │
│  ┌───────────────────────────────────────────────────────────────┐  │
│  │ Is now < TrialEnd?                                            │  │
│  │   YES → Return (ShouldSkipBilling = true, IsInTrial = true)   │  │
│  │         Ensure status = 'trialing'                            │  │
│  │   NO  → Trial has ended, continue                             │  │
│  └───────────────────────────────────────────────────────────────┘  │
│  ┌───────────────────────────────────────────────────────────────┐  │
│  │ Is status = 'trialing'?                                       │  │
│  │   YES → Transition to 'active'                                │  │
│  │         Set CurrentPeriodStart = TrialEnd                     │  │
│  │         Calculate new CurrentPeriodEnd                        │  │
│  │         Update subscription                                   │  │
│  │         Return (StatusTransitioned = true)                    │  │
│  │   NO  → Return (ShouldSkipBilling = false)                    │  │
│  └───────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────┘
                                 │
                   ┌─────────────┴─────────────┐
                   │                           │
        ShouldSkipBilling?                     │
            YES → Return Success               │
                   │                           │
                   │          NO               │
                   └───────────────────────────┘
                                 │
                                 ▼
┌─────────────────────────────────────────────────────────────────────┐
│  Step 2: CalculatePeriodsActivity                                    │
│  - Calculate billing periods from CurrentPeriodStart                 │
│  - (Now starts from TrialEnd if just transitioned)                   │
└─────────────────────────────────────────────────────────────────────┘
                                 │
                                 ▼
┌─────────────────────────────────────────────────────────────────────┐
│  Step 3: ProcessPeriodsActivity                                      │
│  - Create invoices for each period                                   │
└─────────────────────────────────────────────────────────────────────┘
                                 │
                                 ▼
              (Continue with remaining steps...)
```

---

## Appendix B: Example Scenarios

### Scenario 1: Subscription Created with 7-Day Trial

**Timeline**:
```
Day 0 (Jan 1): Subscription created
  - Status: trialing
  - TrialStart: Jan 1
  - TrialEnd: Jan 8
  - CurrentPeriodStart: Jan 1
  - CurrentPeriodEnd: Jan 31 (monthly billing)
  
Day 1-7: Cron runs daily
  - CheckSubscriptionTrialStatusActivity: IsInTrial = true
  - Workflow returns early, no invoices created
  
Day 8 (Jan 8): Trial ends, cron runs
  - CheckSubscriptionTrialStatusActivity: TrialEnded = true
  - Transition to active status
  - Update CurrentPeriodStart: Jan 8
  - Update CurrentPeriodEnd: Feb 8
  - Continue workflow, create first invoice for Jan 8 - Feb 8
  
Day 9+: Normal billing continues
```

### Scenario 2: Existing Subscription with Expired Trial

**Timeline**:
```
Current State:
  - Status: trialing (incorrect)
  - TrialStart: Dec 1
  - TrialEnd: Dec 8
  - CurrentPeriodStart: Dec 1
  - CurrentPeriodEnd: Dec 31
  - Current Date: Dec 15
  
Next Cron Run (Dec 15):
  - CheckSubscriptionTrialStatusActivity detects trial ended
  - Transition to active
  - Update CurrentPeriodStart: Dec 8
  - Update CurrentPeriodEnd: Jan 8
  - Create invoice for Dec 8 - Jan 8 (includes past period)
  
Future: Normal billing continues
```

---

## Appendix C: Model Definitions Location

Add to: `internal/temporal/models/subscription/process_update_billing_period.go`

```go
// CheckSubscriptionTrialStatusActivityInput represents the input for checking subscription trial status
type CheckSubscriptionTrialStatusActivityInput struct {
    SubscriptionID string    `json:"subscription_id"`
    TenantID       string    `json:"tenant_id"`
    EnvironmentID  string    `json:"environment_id"`
    CurrentTime    time.Time `json:"current_time"`
}

// Validate validates the check subscription trial status activity input
func (i *CheckSubscriptionTrialStatusActivityInput) Validate() error {
    if i.SubscriptionID == "" {
        return ierr.NewError("subscription_id is required").
            WithHint("Subscription ID is required").
            Mark(ierr.ErrValidation)
    }
    if i.TenantID == "" {
        return ierr.NewError("tenant_id is required").
            WithHint("Tenant ID is required").
            Mark(ierr.ErrValidation)
    }
    if i.EnvironmentID == "" {
        return ierr.NewError("environment_id is required").
            WithHint("Environment ID is required").
            Mark(ierr.ErrValidation)
    }
    if i.CurrentTime.IsZero() {
        return ierr.NewError("current_time is required").
            WithHint("Current Time is required").
            Mark(ierr.ErrValidation)
    }
    return nil
}

// CheckSubscriptionTrialStatusActivityOutput represents the output for checking subscription trial status
type CheckSubscriptionTrialStatusActivityOutput struct {
    IsInTrial             bool       `json:"is_in_trial"`
    TrialEnded            bool       `json:"trial_ended"`
    TrialEndDate          *time.Time `json:"trial_end_date,omitempty"`
    ShouldSkipBilling     bool       `json:"should_skip_billing"`
    StatusTransitioned    bool       `json:"status_transitioned"`
    NewCurrentPeriodStart *time.Time `json:"new_current_period_start,omitempty"`
    NewCurrentPeriodEnd   *time.Time `json:"new_current_period_end,omitempty"`
}
```

---

**END OF DOCUMENT**

