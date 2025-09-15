package settings

import (
	ierr "github.com/flexprice/flexprice/internal/errors"
)

// settingConfigs registry maps setting keys to their configuration handlers
var settingConfigs = map[string]SettingConfig{
	"invoice_config":      NewInvoiceConfig(),
	"subscription_config": NewSubscriptionConfig(),
}

// GetSettingConfig returns the configuration handler for a given setting key
func GetSettingConfig(key string) (SettingConfig, error) {
	config, exists := settingConfigs[key]
	if !exists {
		return nil, ierr.NewErrorf("unknown setting key: %s", key).
			WithHintf("Unknown setting key: %s. Available keys: %v", key, GetAvailableKeys()).
			Mark(ierr.ErrValidation)
	}
	return config, nil
}

// GetAvailableKeys returns all available setting keys
func GetAvailableKeys() []string {
	keys := make([]string, 0, len(settingConfigs))
	for key := range settingConfigs {
		keys = append(keys, key)
	}
	return keys
}

// RegisterSettingConfig registers a new setting configuration
// This should be called during application initialization
func RegisterSettingConfig(key string, config SettingConfig) error {
	if key == "" {
		return ierr.NewError("setting key cannot be empty").
			WithHint("Please provide a non-empty setting key").
			Mark(ierr.ErrValidation)
	}

	if config == nil {
		return ierr.NewError("setting config cannot be nil").
			WithHint("Please provide a valid setting configuration").
			Mark(ierr.ErrValidation)
	}

	// Validate that the config's key matches the provided key
	if config.GetKey() != key {
		return ierr.NewErrorf("setting config key mismatch: expected '%s', got '%s'", key, config.GetKey()).
			WithHint("The setting config's key must match the registration key").
			Mark(ierr.ErrValidation)
	}

	settingConfigs[key] = config
	return nil
}

// ValidateSettingValue validates a setting value using the appropriate configuration handler
func ValidateSettingValue(key string, value map[string]interface{}) error {
	_, err := GetSettingConfig(key)
	if err != nil {
		return err
	}

	// Create a temporary instance to avoid modifying the registry
	tempConfig := createConfigInstance(key)
	if tempConfig == nil {
		return ierr.NewErrorf("failed to create config instance for key: %s", key).
			WithHint("Please check if the setting configuration is properly registered").
			Mark(ierr.ErrInternal)
	}

	// Populate from JSON
	if err := tempConfig.FromJSON(value); err != nil {
		return err
	}

	// Validate the populated configuration
	return tempConfig.Validate()
}

// createConfigInstance creates a new instance of the configuration for the given key
func createConfigInstance(key string) SettingConfig {
	switch key {
	case "invoice_config":
		return &InvoiceConfig{}
	case "subscription_config":
		return &SubscriptionConfig{}
	default:
		return nil
	}
}

// GetDefaultSettingValue returns the default value for a setting key
func GetDefaultSettingValue(key string) (map[string]interface{}, error) {
	config, err := GetSettingConfig(key)
	if err != nil {
		return nil, err
	}

	return config.GetDefaultValue(), nil
}

// ConvertSettingToJSON converts a setting value to JSON using the appropriate handler
func ConvertSettingToJSON(key string, value map[string]interface{}) (map[string]interface{}, error) {
	_, err := GetSettingConfig(key)
	if err != nil {
		return nil, err
	}

	// Create a temporary instance
	tempConfig := createConfigInstance(key)
	if tempConfig == nil {
		return nil, ierr.NewErrorf("failed to create config instance for key: %s", key).
			WithHint("Please check if the setting configuration is properly registered").
			Mark(ierr.ErrInternal)
	}

	// Populate from JSON
	if err := tempConfig.FromJSON(value); err != nil {
		return nil, err
	}

	// Convert back to JSON
	return tempConfig.ToJSON()
}

// IsValidSettingKey checks if a setting key is valid
func IsValidSettingKey(key string) bool {
	_, exists := settingConfigs[key]
	return exists
}

// GetAllSettingConfigs returns all registered setting configurations
func GetAllSettingConfigs() map[string]SettingConfig {
	// Return a copy to prevent external modification
	result := make(map[string]SettingConfig)
	for key, config := range settingConfigs {
		result[key] = config
	}
	return result
}
