package v1

import (
	"net/http"

	"github.com/flexprice/flexprice/internal/api/dto"
	"github.com/flexprice/flexprice/internal/logger"
	"github.com/flexprice/flexprice/internal/service"
	"github.com/flexprice/flexprice/internal/types"
	"github.com/gin-gonic/gin"
)

// IntegrationHandler handles integration API requests
type IntegrationHandler struct {
	IntegrationService service.IntegrationService
	logger             *logger.Logger
}

// NewIntegrationHandler creates a new integration handler
func NewIntegrationHandler(
	integrationService service.IntegrationService,
	logger *logger.Logger,
) *IntegrationHandler {
	return &IntegrationHandler{
		IntegrationService: integrationService,
		logger:             logger,
	}
}

// SyncEntityToProviders godoc
// @Summary Sync entity to all available providers
// @Description Sync an entity to all available payment providers for the current tenant
// @Tags Integration
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param entity_type path string true "Entity type (e.g., customer, invoice, tax)"
// @Param entity_id path string true "Entity ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} errors.ErrorResponse
// @Failure 401 {object} errors.ErrorResponse
// @Failure 404 {object} errors.ErrorResponse
// @Failure 500 {object} errors.ErrorResponse
// @Router /integration/sync/{entity_type}/{entity_id} [post]
func (h *IntegrationHandler) SyncEntityToProviders(c *gin.Context) {
	entityTypeStr := c.Param("entity_type")
	entityID := c.Param("entity_id")

	if entityTypeStr == "" || entityID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Entity type and entity ID are required",
		})
		return
	}

	// Convert string to enum and validate
	entityType := types.IntegrationEntityType(entityTypeStr)
	if err := entityType.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid entity type",
		})
		return
	}

	ctx := c.Request.Context()
	err := h.IntegrationService.SyncEntityToProviders(ctx, entityType, entityID)
	if err != nil {
		h.logger.Errorw("failed to sync entity to providers",
			"entity_type", entityType,
			"entity_id", entityID,
			"error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to sync entity to providers",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "Entity sync initiated successfully",
		"entity_type": entityType,
		"entity_id":   entityID,
	})
}

// GetAvailableProviders godoc
// @Summary Get available providers
// @Description Get all available payment providers for the current tenant
// @Tags Integration
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {object} dto.ListConnectionsResponse
// @Failure 401 {object} errors.ErrorResponse
// @Failure 500 {object} errors.ErrorResponse
// @Router /integration/providers [get]
func (h *IntegrationHandler) GetAvailableProviders(c *gin.Context) {
	ctx := c.Request.Context()
	providers, err := h.IntegrationService.GetAvailableProviders(ctx)
	if err != nil {
		h.logger.Errorw("failed to get available providers", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get available providers",
		})
		return
	}

	// Convert to response format
	connections := make([]dto.ConnectionResponse, 0, len(providers))
	for _, provider := range providers {
		connections = append(connections, dto.ConnectionResponse{
			ID:            provider.ID,
			Name:          provider.Name,
			ProviderType:  provider.ProviderType,
			EnvironmentID: provider.EnvironmentID,
			TenantID:      provider.TenantID,
			Status:        provider.Status,
			CreatedAt:     provider.CreatedAt.Format("2006-01-02T15:04:05Z"),
			UpdatedAt:     provider.UpdatedAt.Format("2006-01-02T15:04:05Z"),
			CreatedBy:     provider.CreatedBy,
			UpdatedBy:     provider.UpdatedBy,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"providers": connections,
		"total":     len(connections),
	})
}
