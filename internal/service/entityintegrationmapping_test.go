package service

import (
	"context"
	"testing"

	"github.com/flexprice/flexprice/internal/api/dto"
	"github.com/flexprice/flexprice/internal/testutil"
	"github.com/flexprice/flexprice/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type EntityIntegrationMappingServiceSuite struct {
	testutil.BaseServiceTestSuite
	service EntityIntegrationMappingService
}

func TestEntityIntegrationMappingService(t *testing.T) {
	suite.Run(t, new(EntityIntegrationMappingServiceSuite))
}

func (s *EntityIntegrationMappingServiceSuite) SetupTest() {
	s.BaseServiceTestSuite.SetupTest()
	s.setupService()
}

func (s *EntityIntegrationMappingServiceSuite) setupService() {
	stores := s.GetStores()
	s.service = NewEntityIntegrationMappingService(ServiceParams{
		Logger:                       s.GetLogger(),
		Config:                       s.GetConfig(),
		DB:                           s.GetDB(),
		EntityIntegrationMappingRepo: stores.EntityIntegrationMappingRepo,
		WebhookPublisher:             s.GetWebhookPublisher(),
	})
}

func (s *EntityIntegrationMappingServiceSuite) TestCreateEntityIntegrationMapping() {
	// Test data
	req := dto.CreateEntityIntegrationMappingRequest{
		EntityID:         "cust_123",
		EntityType:       "customer",
		ProviderType:     "stripe",
		ProviderEntityID: "cus_stripe_456",
		Metadata: map[string]interface{}{
			"stripe_customer_email": "test@example.com",
		},
	}

	// Execute
	ctx := types.SetTenantID(context.Background(), "test_tenant")
	ctx = types.SetEnvironmentID(ctx, "test_env")
	ctx = types.SetUserID(ctx, "test_user")

	resp, err := s.service.CreateEntityIntegrationMapping(ctx, req)

	// Assert
	require.NoError(s.T(), err)
	assert.NotNil(s.T(), resp)
	assert.Equal(s.T(), req.EntityID, resp.EntityID)
	assert.Equal(s.T(), types.IntegrationEntityType(req.EntityType), resp.EntityType)
	assert.Equal(s.T(), req.ProviderType, resp.ProviderType)
	assert.Equal(s.T(), req.ProviderEntityID, resp.ProviderEntityID)
	// Note: resp.Metadata should NOT exist anymore - this verifies our security fix!
	assert.NotEmpty(s.T(), resp.ID)
	assert.Equal(s.T(), "test_tenant", resp.TenantID)
	assert.Equal(s.T(), "test_env", resp.EnvironmentID)
}

func (s *EntityIntegrationMappingServiceSuite) TestGetEntityIntegrationMapping() {
	// Create a mapping first
	req := dto.CreateEntityIntegrationMappingRequest{
		EntityID:         "cust_123",
		EntityType:       "customer",
		ProviderType:     "stripe",
		ProviderEntityID: "cus_stripe_456",
	}

	ctx := types.SetTenantID(context.Background(), "test_tenant")
	ctx = types.SetEnvironmentID(ctx, "test_env")
	ctx = types.SetUserID(ctx, "test_user")

	created, err := s.service.CreateEntityIntegrationMapping(ctx, req)
	require.NoError(s.T(), err)

	// Get the mapping
	resp, err := s.service.GetEntityIntegrationMapping(ctx, created.ID)

	// Assert
	require.NoError(s.T(), err)
	assert.NotNil(s.T(), resp)
	assert.Equal(s.T(), created.ID, resp.ID)
	assert.Equal(s.T(), req.EntityID, resp.EntityID)
	assert.Equal(s.T(), types.IntegrationEntityType(req.EntityType), resp.EntityType)
	assert.Equal(s.T(), req.ProviderType, resp.ProviderType)
	assert.Equal(s.T(), req.ProviderEntityID, resp.ProviderEntityID)
}

func (s *EntityIntegrationMappingServiceSuite) TestGetByEntityAndProvider() {
	// Create a mapping first
	req := dto.CreateEntityIntegrationMappingRequest{
		EntityID:         "cust_123",
		EntityType:       "customer",
		ProviderType:     "stripe",
		ProviderEntityID: "cus_stripe_456",
	}

	ctx := types.SetTenantID(context.Background(), "test_tenant")
	ctx = types.SetEnvironmentID(ctx, "test_env")
	ctx = types.SetUserID(ctx, "test_user")

	_, err := s.service.CreateEntityIntegrationMapping(ctx, req)
	require.NoError(s.T(), err)

	// Get by entity and provider using plural filters
	listResp, err := s.service.GetEntityIntegrationMappings(ctx, &types.EntityIntegrationMappingFilter{
		QueryFilter:   types.NewDefaultQueryFilter(),
		EntityID:      "cust_123",
		EntityType:    types.IntegrationEntityType("customer"),
		ProviderTypes: []string{"stripe"},
	})

	// Assert
	require.NoError(s.T(), err)
	require.NotNil(s.T(), listResp)
	require.GreaterOrEqual(s.T(), len(listResp.Items), 1)
	first := listResp.Items[0]
	assert.Equal(s.T(), "cust_123", first.EntityID)
	assert.Equal(s.T(), types.IntegrationEntityType("customer"), first.EntityType)
	assert.Equal(s.T(), "stripe", first.ProviderType)
	assert.Equal(s.T(), "cus_stripe_456", first.ProviderEntityID)
}

func (s *EntityIntegrationMappingServiceSuite) TestGetByProviderEntity() {
	// Create a mapping first
	req := dto.CreateEntityIntegrationMappingRequest{
		EntityID:         "cust_123",
		EntityType:       "customer",
		ProviderType:     "stripe",
		ProviderEntityID: "cus_stripe_456",
	}

	ctx := types.SetTenantID(context.Background(), "test_tenant")
	ctx = types.SetEnvironmentID(ctx, "test_env")
	ctx = types.SetUserID(ctx, "test_user")

	_, err := s.service.CreateEntityIntegrationMapping(ctx, req)
	require.NoError(s.T(), err)

	// Get by provider entity using plural filters
	listResp, err := s.service.GetEntityIntegrationMappings(ctx, &types.EntityIntegrationMappingFilter{
		QueryFilter:       types.NewDefaultQueryFilter(),
		ProviderTypes:     []string{"stripe"},
		ProviderEntityIDs: []string{"cus_stripe_456"},
	})

	// Assert
	require.NoError(s.T(), err)
	require.NotNil(s.T(), listResp)
	require.GreaterOrEqual(s.T(), len(listResp.Items), 1)
	first := listResp.Items[0]
	assert.Equal(s.T(), "cust_123", first.EntityID)
	assert.Equal(s.T(), types.IntegrationEntityType("customer"), first.EntityType)
	assert.Equal(s.T(), "stripe", first.ProviderType)
	assert.Equal(s.T(), "cus_stripe_456", first.ProviderEntityID)
}
