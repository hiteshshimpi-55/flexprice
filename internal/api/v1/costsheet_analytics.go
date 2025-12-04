package v1

import (
	"net/http"

	"github.com/flexprice/flexprice/internal/api/dto"
	"github.com/flexprice/flexprice/internal/config"
	ierr "github.com/flexprice/flexprice/internal/errors"
	"github.com/flexprice/flexprice/internal/interfaces"
	"github.com/flexprice/flexprice/internal/logger"
	"github.com/gin-gonic/gin"
)

type RevenueAnalyticsHandler struct {
	revenueAnalyticsService interfaces.RevenueAnalyticsService
	config                  *config.Configuration
	Logger                  *logger.Logger
}

func NewRevenueAnalyticsHandler(
	revenueAnalyticsService interfaces.RevenueAnalyticsService,
	config *config.Configuration,
	logger *logger.Logger,
) *RevenueAnalyticsHandler {
	return &RevenueAnalyticsHandler{
		revenueAnalyticsService: revenueAnalyticsService,
		config:                  config,
		Logger:                  logger,
	}
}

// GetCombinedAnalytics retrieves combined cost and revenue analytics with derived metrics
// @Summary Get combined revenue and cost analytics
// @Description Retrieve combined analytics with ROI, margin, and detailed breakdowns. If start_time and end_time are not provided, defaults to last 7 days.
// @Tags Costs
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param request body dto.GetCostAnalyticsRequest true "Combined analytics request (start_time/end_time optional - defaults to last 7 days)"
// @Success 200 {object} dto.GetDetailedCostAnalyticsResponse
// @Failure 400 {object} ierr.ErrorResponse
// @Failure 500 {object} ierr.ErrorResponse
// @Router /costs/analytics [post]
func (h *RevenueAnalyticsHandler) GetDetailedCostAnalytics(c *gin.Context) {
	ctx := c.Request.Context()

	var req dto.GetCostAnalyticsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(ierr.WithError(err).
			WithHint("Please check the request payload").
			Mark(ierr.ErrValidation))
		return
	}

	response, err := h.revenueAnalyticsService.GetDetailedCostAnalytics(ctx, &req)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, response)
}

// GetRevenueTimeSeries retrieves aggregated revenue per time window
// @Summary Get aggregated revenue time series
// @Description Retrieve total revenue aggregated per time window (HOUR, DAY, WEEK, MONTH). Returns a single time series with revenue summed across all features.
// @Tags Revenue
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param request body dto.GetRevenueTimeSeriesRequest true "Revenue time series request"
// @Success 200 {object} dto.GetRevenueTimeSeriesResponse
// @Failure 400 {object} ierr.ErrorResponse
// @Failure 500 {object} ierr.ErrorResponse
// @Router /revenue/timeseries [post]
func (h *RevenueAnalyticsHandler) GetRevenueTimeSeries(c *gin.Context) {
	ctx := c.Request.Context()

	var req dto.GetRevenueTimeSeriesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(ierr.WithError(err).
			WithHint("Please check the request payload").
			Mark(ierr.ErrValidation))
		return
	}

	response, err := h.revenueAnalyticsService.GetRevenueTimeSeries(ctx, &req)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, response)
}
