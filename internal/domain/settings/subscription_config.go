package settings

import (
	ierr "github.com/flexprice/flexprice/internal/errors"
)

// SubscriptionConfig represents the configuration for subscription auto-cancellation
type SubscriptionConfig struct {
	BaseSettingConfig

	GracePeriodDays         int  `json:"grace_period_days"`
	AutoCancellationEnabled bool `json:"auto_cancellation_enabled"`
}

// NewSubscriptionConfig creates a new SubscriptionConfig with default values
func NewSubscriptionConfig() *SubscriptionConfig {
	return &SubscriptionConfig{
		BaseSettingConfig: BaseSettingConfig{
			Key:         "subscription_config",
			Description: "Configuration for subscription auto-cancellation (grace period and enabled flag)",
		},
		GracePeriodDays:         3,
		AutoCancellationEnabled: false,
	}
}

// GetDefaultValue returns the default values for subscription configuration
func (sc *SubscriptionConfig) GetDefaultValue() map[string]interface{} {
	return map[string]interface{}{
		"grace_period_days":         sc.GracePeriodDays,
		"auto_cancellation_enabled": sc.AutoCancellationEnabled,
	}
}

// FromJSON populates the SubscriptionConfig from JSON data
func (sc *SubscriptionConfig) FromJSON(data map[string]interface{}) error {
	if data == nil {
		return ierr.NewError("subscription_config data cannot be nil").
			WithHint("Please provide valid subscription configuration data").
			Mark(ierr.ErrValidation)
	}

	// Validate grace_period_days if present
	if gracePeriodDays, err := ValidateIntField(data, "grace_period_days", false, intPtr(1), nil); err != nil {
		return err
	} else if gracePeriodDays != 0 || data["grace_period_days"] != nil {
		sc.GracePeriodDays = gracePeriodDays
	}

	// Validate auto_cancellation_enabled if present
	if autoCancellationEnabled, err := ValidateBoolField(data, "auto_cancellation_enabled", false); err != nil {
		return err
	} else if data["auto_cancellation_enabled"] != nil {
		sc.AutoCancellationEnabled = autoCancellationEnabled
	}

	return nil
}

// ToJSON converts the SubscriptionConfig to JSON format
func (sc *SubscriptionConfig) ToJSON() (map[string]interface{}, error) {
	return ConvertToJSON(sc)
}

// Validate validates the SubscriptionConfig
func (sc *SubscriptionConfig) Validate() error {
	// Validate grace_period_days
	if sc.GracePeriodDays < 1 {
		return ierr.NewError("subscription_config: 'grace_period_days' must be greater than or equal to 1").
			WithHint("Please provide a valid grace period in days").
			Mark(ierr.ErrValidation)
	}

	// AutoCancellationEnabled is a boolean, so no additional validation needed
	return nil
}
