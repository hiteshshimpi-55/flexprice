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

// @Summary Create a setting
// @Description Create a new setting for the tenant and environment
// @Tags Settings
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param setting body dto.CreateSettingRequest true "Setting"
// @Success 201 {object} dto.SettingResponse
// @Failure 400 {object} ierr.ErrorResponse
// @Failure 409 {object} ierr.ErrorResponse
// @Failure 500 {object} ierr.ErrorResponse
// @Router /settings [post]
func (h *SettingsHandler) CreateSetting(c *gin.Context) {
	var req dto.CreateSettingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(ierr.WithError(err).
			WithHint("Invalid request format").
			Mark(ierr.ErrValidation))
		return
	}

	resp, err := h.service.CreateSetting(c.Request.Context(), &req)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusCreated, resp)
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
// @Router /settings/key/{key} [get]
func (h *SettingsHandler) GetSettingByKey(c *gin.Context) {
	key := c.Param("key")

	resp, err := h.service.GetSetting(c.Request.Context(), key)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// @Summary Get a setting by ID
// @Description Get a setting by its unique ID
// @Tags Settings
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param id path string true "Setting ID"
// @Success 200 {object} dto.SettingResponse
// @Failure 404 {object} ierr.ErrorResponse
// @Failure 500 {object} ierr.ErrorResponse
// @Router /settings/{id} [get]
func (h *SettingsHandler) GetSetting(c *gin.Context) {
	id := c.Param("id")

	resp, err := h.service.GetSettingByID(c.Request.Context(), id)
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
// @Param id path string true "Setting ID"
// @Param setting body dto.UpdateSettingRequest true "Setting update data"
// @Success 200 {object} dto.SettingResponse
// @Failure 400 {object} ierr.ErrorResponse
// @Failure 404 {object} ierr.ErrorResponse
// @Failure 500 {object} ierr.ErrorResponse
// @Router /settings/{id} [put]
func (h *SettingsHandler) UpdateSetting(c *gin.Context) {
	id := c.Param("id")

	var req dto.UpdateSettingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(ierr.WithError(err).
			WithHint("Invalid request format").
			Mark(ierr.ErrValidation))
		return
	}

	req.ID = id

	resp, err := h.service.UpdateSetting(c.Request.Context(), &req)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// @Summary Delete a setting
// @Description Delete a setting by ID (soft delete)
// @Tags Settings
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param id path string true "Setting ID"
// @Success 204 "No Content"
// @Failure 404 {object} ierr.ErrorResponse
// @Failure 500 {object} ierr.ErrorResponse
// @Router /settings/{id} [delete]
func (h *SettingsHandler) DeleteSetting(c *gin.Context) {
	id := c.Param("id")

	err := h.service.DeleteSetting(c.Request.Context(), id)
	if err != nil {
		c.Error(err)
		return
	}

	c.Status(http.StatusNoContent)
}

// @Summary Update a setting
// @Description Update an existing setting
// @Tags Settings
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param id path string true "Setting ID"
// @Param setting body dto.UpdateSettingRequest true "Setting update data"
// @Success 200 {object} dto.SettingResponse
// @Failure 400 {object} ierr.ErrorResponse
// @Failure 404 {object} ierr.ErrorResponse
// @Failure 500 {object} ierr.ErrorResponse
// @Router /settings/{id} [put]
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
