package service

import (
	"testing"

	"github.com/flexprice/flexprice/internal/testutil"
	"github.com/stretchr/testify/suite"
)

type IntegrationServiceSuite struct {
	testutil.BaseServiceTestSuite
	service IntegrationService
}

func TestIntegrationService(t *testing.T) {
	suite.Run(t, new(IntegrationServiceSuite))
}

func (s *IntegrationServiceSuite) SetupTest() {
	s.BaseServiceTestSuite.SetupTest()
	s.service = NewIntegrationService(ServiceParams{
		Logger:                       s.GetLogger(),
		Config:                       s.GetConfig(),
		DB:                           s.GetDB(),
		CustomerRepo:                 s.GetStores().CustomerRepo,
		ConnectionRepo:               s.GetStores().ConnectionRepo,
		EntityIntegrationMappingRepo: s.GetStores().EntityIntegrationMappingRepo,
		EventPublisher:               s.GetPublisher(),
		WebhookPublisher:             s.GetWebhookPublisher(),
	})
}

func (s *IntegrationServiceSuite) TestIntegrationServiceCreation() {
	// Simple test to verify the service can be created
	s.NotNil(s.service)
}
