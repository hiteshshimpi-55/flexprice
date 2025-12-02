package subscription

import (
	"testing"
	"time"

	"github.com/flexprice/flexprice/internal/domain/customer"
	"github.com/flexprice/flexprice/internal/domain/plan"
	"github.com/flexprice/flexprice/internal/domain/subscription"
	"github.com/flexprice/flexprice/internal/service"
	subscriptionModels "github.com/flexprice/flexprice/internal/temporal/models/subscription"
	"github.com/flexprice/flexprice/internal/testutil"
	"github.com/flexprice/flexprice/internal/types"
	"github.com/stretchr/testify/suite"
)

// UpdateBillingPeriodTrialTestSuite tests trial status functionality in update billing period activities
type UpdateBillingPeriodTrialTestSuite struct {
	testutil.BaseServiceTestSuite
	activities *UpdateBillingPeriodActivities
	testData   struct {
		customer                     *customer.Customer
		plan                         *plan.Plan
		subscriptionInTrial          *subscription.Subscription
		subscriptionTrialEnded       *subscription.Subscription
		subscriptionNoTrial          *subscription.Subscription
		subscriptionIncorrectStatus  *subscription.Subscription
		subscriptionTrialExpiredLong *subscription.Subscription
	}
}

func TestUpdateBillingPeriodTrialActivities(t *testing.T) {
	suite.Run(t, new(UpdateBillingPeriodTrialTestSuite))
}

func (s *UpdateBillingPeriodTrialTestSuite) SetupSuite() {
	s.BaseServiceTestSuite.SetupSuite()
}

func (s *UpdateBillingPeriodTrialTestSuite) SetupTest() {
	s.BaseServiceTestSuite.SetupTest()

	// Initialize service params
	serviceParams := service.ServiceParams{
		Logger:           s.GetLogger(),
		Config:           s.GetConfig(),
		DB:               s.GetDB(),
		SubRepo:          s.GetStores().SubscriptionRepo,
		PlanRepo:         s.GetStores().PlanRepo,
		PriceRepo:        s.GetStores().PriceRepo,
		EventRepo:        s.GetStores().EventRepo,
		MeterRepo:        s.GetStores().MeterRepo,
		CustomerRepo:     s.GetStores().CustomerRepo,
		InvoiceRepo:      s.GetStores().InvoiceRepo,
		EntitlementRepo:  s.GetStores().EntitlementRepo,
		EnvironmentRepo:  s.GetStores().EnvironmentRepo,
		FeatureRepo:      s.GetStores().FeatureRepo,
		TenantRepo:       s.GetStores().TenantRepo,
		UserRepo:         s.GetStores().UserRepo,
		AuthRepo:         s.GetStores().AuthRepo,
		WalletRepo:       s.GetStores().WalletRepo,
		PaymentRepo:      s.GetStores().PaymentRepo,
		EventPublisher:   s.GetPublisher(),
		WebhookPublisher: s.GetWebhookPublisher(),
	}

	subscriptionService := service.NewSubscriptionService(serviceParams)

	// Initialize activities
	s.activities = NewUpdateBillingPeriodActivities(
		subscriptionService,
		serviceParams,
		s.GetLogger(),
	)

	s.setupTrialTestData()
}

func (s *UpdateBillingPeriodTrialTestSuite) TearDownTest() {
	s.BaseServiceTestSuite.TearDownTest()
}

func (s *UpdateBillingPeriodTrialTestSuite) setupTrialTestData() {
	ctx := s.GetContext()
	now := time.Now().UTC()

	// Create test customer
	s.testData.customer = &customer.Customer{
		ID:         "cust_trial_123",
		ExternalID: "ext_cust_trial_123",
		Name:       "Test Trial Customer",
		Email:      "test_trial@example.com",
		BaseModel:  types.GetDefaultBaseModel(ctx),
	}
	s.NoError(s.GetStores().CustomerRepo.Create(ctx, s.testData.customer))

	// Create test plan
	s.testData.plan = &plan.Plan{
		ID:          "plan_trial_123",
		Name:        "Test Trial Plan",
		Description: "Test Trial Plan Description",
		BaseModel:   types.GetDefaultBaseModel(ctx),
	}
	s.NoError(s.GetStores().PlanRepo.Create(ctx, s.testData.plan))

	// Subscription 1: In trial (trial ends in 7 days)
	trialStart := now
	trialEnd := now.Add(7 * 24 * time.Hour)
	s.testData.subscriptionInTrial = &subscription.Subscription{
		ID:                 "sub_in_trial",
		CustomerID:         s.testData.customer.ID,
		PlanID:             s.testData.plan.ID,
		SubscriptionStatus: types.SubscriptionStatusTrialing,
		Currency:           "usd",
		BillingAnchor:      now,
		BillingCycle:       types.BillingCycleAnniversary,
		StartDate:          now,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.Add(30 * 24 * time.Hour),
		BillingCadence:     types.BILLING_CADENCE_RECURRING,
		BillingPeriod:      types.BILLING_PERIOD_MONTHLY,
		BillingPeriodCount: 1,
		TrialStart:         &trialStart,
		TrialEnd:           &trialEnd,
		BaseModel:          types.GetDefaultBaseModel(ctx),
		CollectionMethod:   string(types.CollectionMethodChargeAutomatically),
		CancelAtPeriodEnd:  false,
		PauseStatus:        types.PauseStatusNone,
	}
	s.NoError(s.GetStores().SubscriptionRepo.Create(ctx, s.testData.subscriptionInTrial))

	// Subscription 2: Trial just ended (trial ended 1 hour ago)
	trialStart2 := now.Add(-8 * 24 * time.Hour)
	trialEnd2 := now.Add(-1 * time.Hour)
	s.testData.subscriptionTrialEnded = &subscription.Subscription{
		ID:                 "sub_trial_ended",
		CustomerID:         s.testData.customer.ID,
		PlanID:             s.testData.plan.ID,
		SubscriptionStatus: types.SubscriptionStatusTrialing,
		Currency:           "usd",
		BillingAnchor:      now.Add(-8 * 24 * time.Hour),
		BillingCycle:       types.BillingCycleAnniversary,
		StartDate:          now.Add(-8 * 24 * time.Hour),
		CurrentPeriodStart: now.Add(-8 * 24 * time.Hour),
		CurrentPeriodEnd:   now.Add(22 * 24 * time.Hour),
		BillingCadence:     types.BILLING_CADENCE_RECURRING,
		BillingPeriod:      types.BILLING_PERIOD_MONTHLY,
		BillingPeriodCount: 1,
		TrialStart:         &trialStart2,
		TrialEnd:           &trialEnd2,
		BaseModel:          types.GetDefaultBaseModel(ctx),
		CollectionMethod:   string(types.CollectionMethodChargeAutomatically),
		CancelAtPeriodEnd:  false,
		PauseStatus:        types.PauseStatusNone,
	}
	s.NoError(s.GetStores().SubscriptionRepo.Create(ctx, s.testData.subscriptionTrialEnded))

	// Subscription 3: No trial
	s.testData.subscriptionNoTrial = &subscription.Subscription{
		ID:                 "sub_no_trial",
		CustomerID:         s.testData.customer.ID,
		PlanID:             s.testData.plan.ID,
		SubscriptionStatus: types.SubscriptionStatusActive,
		Currency:           "usd",
		BillingAnchor:      now,
		BillingCycle:       types.BillingCycleAnniversary,
		StartDate:          now,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.Add(30 * 24 * time.Hour),
		BillingCadence:     types.BILLING_CADENCE_RECURRING,
		BillingPeriod:      types.BILLING_PERIOD_MONTHLY,
		BillingPeriodCount: 1,
		BaseModel:          types.GetDefaultBaseModel(ctx),
		CollectionMethod:   string(types.CollectionMethodChargeAutomatically),
		CancelAtPeriodEnd:  false,
		PauseStatus:        types.PauseStatusNone,
	}
	s.NoError(s.GetStores().SubscriptionRepo.Create(ctx, s.testData.subscriptionNoTrial))

	// Subscription 4: Has trial but status is incorrect (active instead of trialing)
	trialStart4 := now
	trialEnd4 := now.Add(7 * 24 * time.Hour)
	s.testData.subscriptionIncorrectStatus = &subscription.Subscription{
		ID:                 "sub_incorrect_status",
		CustomerID:         s.testData.customer.ID,
		PlanID:             s.testData.plan.ID,
		SubscriptionStatus: types.SubscriptionStatusActive, // Wrong status
		Currency:           "usd",
		BillingAnchor:      now,
		BillingCycle:       types.BillingCycleAnniversary,
		StartDate:          now,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.Add(30 * 24 * time.Hour),
		BillingCadence:     types.BILLING_CADENCE_RECURRING,
		BillingPeriod:      types.BILLING_PERIOD_MONTHLY,
		BillingPeriodCount: 1,
		TrialStart:         &trialStart4,
		TrialEnd:           &trialEnd4,
		BaseModel:          types.GetDefaultBaseModel(ctx),
		CollectionMethod:   string(types.CollectionMethodChargeAutomatically),
		CancelAtPeriodEnd:  false,
		PauseStatus:        types.PauseStatusNone,
	}
	s.NoError(s.GetStores().SubscriptionRepo.Create(ctx, s.testData.subscriptionIncorrectStatus))

	// Subscription 5: Trial ended long ago (30 days ago)
	trialStart5 := now.Add(-37 * 24 * time.Hour)
	trialEnd5 := now.Add(-30 * 24 * time.Hour)
	s.testData.subscriptionTrialExpiredLong = &subscription.Subscription{
		ID:                 "sub_trial_expired_long",
		CustomerID:         s.testData.customer.ID,
		PlanID:             s.testData.plan.ID,
		SubscriptionStatus: types.SubscriptionStatusTrialing, // Status not updated
		Currency:           "usd",
		BillingAnchor:      now.Add(-37 * 24 * time.Hour),
		BillingCycle:       types.BillingCycleAnniversary,
		StartDate:          now.Add(-37 * 24 * time.Hour),
		CurrentPeriodStart: now.Add(-37 * 24 * time.Hour),
		CurrentPeriodEnd:   now.Add(-7 * 24 * time.Hour),
		BillingCadence:     types.BILLING_CADENCE_RECURRING,
		BillingPeriod:      types.BILLING_PERIOD_MONTHLY,
		BillingPeriodCount: 1,
		TrialStart:         &trialStart5,
		TrialEnd:           &trialEnd5,
		BaseModel:          types.GetDefaultBaseModel(ctx),
		CollectionMethod:   string(types.CollectionMethodChargeAutomatically),
		CancelAtPeriodEnd:  false,
		PauseStatus:        types.PauseStatusNone,
	}
	s.NoError(s.GetStores().SubscriptionRepo.Create(ctx, s.testData.subscriptionTrialExpiredLong))
}

// Test 1: Subscription with no trial dates should proceed with normal billing
func (s *UpdateBillingPeriodTrialTestSuite) TestCheckTrialStatus_NoTrial() {
	ctx := s.GetContext()
	now := time.Now().UTC()

	input := subscriptionModels.CheckSubscriptionTrialStatusActivityInput{
		SubscriptionID: s.testData.subscriptionNoTrial.ID,
		TenantID:       s.testData.subscriptionNoTrial.TenantID,
		EnvironmentID:  s.testData.subscriptionNoTrial.EnvironmentID,
		CurrentTime:    now,
	}

	output, err := s.activities.CheckSubscriptionTrialStatusActivity(ctx, input)
	s.NoError(err)
	s.NotNil(output)

	// Should not skip billing for non-trial subscriptions
	s.False(output.ShouldSkipBilling)
	s.False(output.IsInTrial)
	s.False(output.TrialEnded)
	s.False(output.StatusTransitioned)
	s.Nil(output.TrialEndDate)
}

// Test 2: Subscription in trial should skip billing
func (s *UpdateBillingPeriodTrialTestSuite) TestCheckTrialStatus_InTrial() {
	ctx := s.GetContext()
	now := time.Now().UTC()

	input := subscriptionModels.CheckSubscriptionTrialStatusActivityInput{
		SubscriptionID: s.testData.subscriptionInTrial.ID,
		TenantID:       s.testData.subscriptionInTrial.TenantID,
		EnvironmentID:  s.testData.subscriptionInTrial.EnvironmentID,
		CurrentTime:    now,
	}

	output, err := s.activities.CheckSubscriptionTrialStatusActivity(ctx, input)
	s.NoError(err)
	s.NotNil(output)

	// Should skip billing for subscriptions in trial
	s.True(output.ShouldSkipBilling)
	s.True(output.IsInTrial)
	s.False(output.TrialEnded)
	s.False(output.StatusTransitioned)
	s.NotNil(output.TrialEndDate)

	// Verify subscription status is still trialing
	sub, err := s.GetStores().SubscriptionRepo.Get(ctx, s.testData.subscriptionInTrial.ID)
	s.NoError(err)
	s.Equal(types.SubscriptionStatusTrialing, sub.SubscriptionStatus)
}

// Test 3: Subscription with trial just ended should transition to active
func (s *UpdateBillingPeriodTrialTestSuite) TestCheckTrialStatus_TrialEnded() {
	ctx := s.GetContext()
	now := time.Now().UTC()

	input := subscriptionModels.CheckSubscriptionTrialStatusActivityInput{
		SubscriptionID: s.testData.subscriptionTrialEnded.ID,
		TenantID:       s.testData.subscriptionTrialEnded.TenantID,
		EnvironmentID:  s.testData.subscriptionTrialEnded.EnvironmentID,
		CurrentTime:    now,
	}

	output, err := s.activities.CheckSubscriptionTrialStatusActivity(ctx, input)
	s.NoError(err)
	s.NotNil(output)

	// Should not skip billing after trial ends
	s.False(output.ShouldSkipBilling)
	s.False(output.IsInTrial)
	s.True(output.TrialEnded)
	s.True(output.StatusTransitioned)
	s.NotNil(output.TrialEndDate)
	s.NotNil(output.NewCurrentPeriodStart)
	s.NotNil(output.NewCurrentPeriodEnd)

	// Verify subscription status transitioned to active
	sub, err := s.GetStores().SubscriptionRepo.Get(ctx, s.testData.subscriptionTrialEnded.ID)
	s.NoError(err)
	s.Equal(types.SubscriptionStatusActive, sub.SubscriptionStatus)

	// Verify billing period was adjusted to start from trial end
	s.Equal(*s.testData.subscriptionTrialEnded.TrialEnd, sub.CurrentPeriodStart)
	s.True(sub.CurrentPeriodEnd.After(sub.CurrentPeriodStart))
}

// Test 4: Subscription with incorrect status should be corrected
func (s *UpdateBillingPeriodTrialTestSuite) TestCheckTrialStatus_CorrectStatus() {
	ctx := s.GetContext()
	now := time.Now().UTC()

	input := subscriptionModels.CheckSubscriptionTrialStatusActivityInput{
		SubscriptionID: s.testData.subscriptionIncorrectStatus.ID,
		TenantID:       s.testData.subscriptionIncorrectStatus.TenantID,
		EnvironmentID:  s.testData.subscriptionIncorrectStatus.EnvironmentID,
		CurrentTime:    now,
	}

	output, err := s.activities.CheckSubscriptionTrialStatusActivity(ctx, input)
	s.NoError(err)
	s.NotNil(output)

	// Should skip billing and correct status
	s.True(output.ShouldSkipBilling)
	s.True(output.IsInTrial)
	s.False(output.TrialEnded)
	s.False(output.StatusTransitioned) // Not transitioned because it was corrected to trialing

	// Verify subscription status was corrected to trialing
	sub, err := s.GetStores().SubscriptionRepo.Get(ctx, s.testData.subscriptionIncorrectStatus.ID)
	s.NoError(err)
	s.Equal(types.SubscriptionStatusTrialing, sub.SubscriptionStatus)
}

// Test 5: Subscription with trial expired long ago should transition
func (s *UpdateBillingPeriodTrialTestSuite) TestCheckTrialStatus_TrialExpiredLongAgo() {
	ctx := s.GetContext()
	now := time.Now().UTC()

	input := subscriptionModels.CheckSubscriptionTrialStatusActivityInput{
		SubscriptionID: s.testData.subscriptionTrialExpiredLong.ID,
		TenantID:       s.testData.subscriptionTrialExpiredLong.TenantID,
		EnvironmentID:  s.testData.subscriptionTrialExpiredLong.EnvironmentID,
		CurrentTime:    now,
	}

	output, err := s.activities.CheckSubscriptionTrialStatusActivity(ctx, input)
	s.NoError(err)
	s.NotNil(output)

	// Should transition and continue with billing
	s.False(output.ShouldSkipBilling)
	s.False(output.IsInTrial)
	s.True(output.TrialEnded)
	s.True(output.StatusTransitioned)

	// Verify subscription transitioned to active
	sub, err := s.GetStores().SubscriptionRepo.Get(ctx, s.testData.subscriptionTrialExpiredLong.ID)
	s.NoError(err)
	s.Equal(types.SubscriptionStatusActive, sub.SubscriptionStatus)

	// Verify billing period starts from trial end date
	s.Equal(*s.testData.subscriptionTrialExpiredLong.TrialEnd, sub.CurrentPeriodStart)
}

// Test 6: Invalid input validation
func (s *UpdateBillingPeriodTrialTestSuite) TestCheckTrialStatus_InvalidInput() {
	ctx := s.GetContext()

	// Missing subscription ID
	input := subscriptionModels.CheckSubscriptionTrialStatusActivityInput{
		SubscriptionID: "",
		TenantID:       "tenant_123",
		EnvironmentID:  "env_123",
		CurrentTime:    time.Now().UTC(),
	}

	_, err := s.activities.CheckSubscriptionTrialStatusActivity(ctx, input)
	s.Error(err)

	// Missing current time
	input2 := subscriptionModels.CheckSubscriptionTrialStatusActivityInput{
		SubscriptionID: "sub_123",
		TenantID:       "tenant_123",
		EnvironmentID:  "env_123",
		CurrentTime:    time.Time{},
	}

	_, err2 := s.activities.CheckSubscriptionTrialStatusActivity(ctx, input2)
	s.Error(err2)
}

// Test 7: Non-existent subscription
func (s *UpdateBillingPeriodTrialTestSuite) TestCheckTrialStatus_NonExistentSubscription() {
	ctx := s.GetContext()
	now := time.Now().UTC()

	baseModel := types.GetDefaultBaseModel(ctx)
	envID := types.GetEnvironmentID(ctx)

	input := subscriptionModels.CheckSubscriptionTrialStatusActivityInput{
		SubscriptionID: "non_existent_sub",
		TenantID:       baseModel.TenantID,
		EnvironmentID:  envID,
		CurrentTime:    now,
	}

	_, err := s.activities.CheckSubscriptionTrialStatusActivity(ctx, input)
	s.Error(err)
}
