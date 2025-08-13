package v1

import (
	"net/http"

	"github.com/flexprice/flexprice/internal/api/dto"
	ierr "github.com/flexprice/flexprice/internal/errors"
	"github.com/flexprice/flexprice/internal/logger"
	"github.com/flexprice/flexprice/internal/service"
	"github.com/gin-gonic/gin"
)

type SettingsHandler struct {
	service service.SettingsService
	log     *logger.Logger
}

func NewSettingsHandler(
	service service.SettingsService,
	log *logger.Logger,
) *SettingsHandler {
	return &SettingsHandler{
		service: service,
		log:     log,
	}
}

// @Summary Get a setting by key
// @Description Get a setting by key
// @Tags Settings
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param key path string true "Setting Key"
// @Success 200 {object} dto.SettingResponse
// @Failure 404 {object} ierr.ErrorResponse
// @Failure 500 {object} ierr.ErrorResponse
// @Router /settings/{key} [get]
func (h *SettingsHandler) GetSettingByKey(c *gin.Context) {
	key := c.Param("key")

	resp, err := h.service.GetSettingByKey(c.Request.Context(), key)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// @Summary Update a setting
// @Description Update an existing setting
// @Tags Settings
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param key path string true "Setting Key"
// @Param setting body dto.UpdateSettingRequest true "Setting update data"
// @Success 200 {object} dto.SettingResponse
// @Failure 400 {object} ierr.ErrorResponse
// @Failure 404 {object} ierr.ErrorResponse
// @Failure 500 {object} ierr.ErrorResponse
// @Router /settings/{key} [put]
func (h *SettingsHandler) UpdateSettingByKey(c *gin.Context) {
	key := c.Param("key")

	var req dto.UpdateSettingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(ierr.WithError(err).
			WithHint("Invalid request format").
			Mark(ierr.ErrValidation))
		return
	}

	resp, err := h.service.UpdateSettingByKey(c.Request.Context(), key, &req)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// @Summary Delete a setting
// @Description Delete an existing setting
// @Tags Settings
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param key path string true "Setting Key"
// @Param setting body dto.UpdateSettingRequest true "Setting update data"
// @Success 200 {object} dto.SettingResponse
// @Failure 400 {object} ierr.ErrorResponse
// @Failure 404 {object} ierr.ErrorResponse
// @Failure 500 {object} ierr.ErrorResponse
// @Router /settings/{key} [delete]
func (h *SettingsHandler) DeleteSettingByKey(c *gin.Context) {
	key := c.Param("key")

	err := h.service.DeleteSettingByKey(c.Request.Context(), key)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Setting deleted successfully"})
}
