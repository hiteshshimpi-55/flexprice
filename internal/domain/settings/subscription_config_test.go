package settings

import (
	"testing"
)

func TestSubscriptionConfig_GetKey(t *testing.T) {
	config := NewSubscriptionConfig()
	if config.GetKey() != "subscription_config" {
		t.Errorf("expected key 'subscription_config', got %s", config.GetKey())
	}
}

func TestSubscriptionConfig_GetDescription(t *testing.T) {
	config := NewSubscriptionConfig()
	expected := "Configuration for subscription auto-cancellation (grace period and enabled flag)"
	if config.GetDescription() != expected {
		t.Errorf("expected description '%s', got '%s'", expected, config.GetDescription())
	}
}

func TestSubscriptionConfig_GetDefaultValue(t *testing.T) {
	config := NewSubscriptionConfig()
	defaultValue := config.GetDefaultValue()

	expected := map[string]interface{}{
		"grace_period_days":         3,
		"auto_cancellation_enabled": false,
	}

	for key, expectedValue := range expected {
		if defaultValue[key] != expectedValue {
			t.Errorf("expected %s to be %v, got %v", key, expectedValue, defaultValue[key])
		}
	}
}

func TestSubscriptionConfig_FromJSON(t *testing.T) {
	tests := []struct {
		name        string
		data        map[string]interface{}
		expectError bool
		expected    *SubscriptionConfig
	}{
		{
			name: "valid full config",
			data: map[string]interface{}{
				"grace_period_days":         7,
				"auto_cancellation_enabled": true,
			},
			expectError: false,
			expected: &SubscriptionConfig{
				BaseSettingConfig: BaseSettingConfig{
					Key:         "subscription_config",
					Description: "Configuration for subscription auto-cancellation (grace period and enabled flag)",
				},
				GracePeriodDays:         7,
				AutoCancellationEnabled: true,
			},
		},
		{
			name: "partial update - only grace_period_days",
			data: map[string]interface{}{
				"grace_period_days": 14,
			},
			expectError: false,
			expected: &SubscriptionConfig{
				BaseSettingConfig: BaseSettingConfig{
					Key:         "subscription_config",
					Description: "Configuration for subscription auto-cancellation (grace period and enabled flag)",
				},
				GracePeriodDays:         14,    // updated
				AutoCancellationEnabled: false, // default
			},
		},
		{
			name: "partial update - only auto_cancellation_enabled",
			data: map[string]interface{}{
				"auto_cancellation_enabled": true,
			},
			expectError: false,
			expected: &SubscriptionConfig{
				BaseSettingConfig: BaseSettingConfig{
					Key:         "subscription_config",
					Description: "Configuration for subscription auto-cancellation (grace period and enabled flag)",
				},
				GracePeriodDays:         3,    // default
				AutoCancellationEnabled: true, // updated
			},
		},
		{
			name: "invalid grace_period_days - too low",
			data: map[string]interface{}{
				"grace_period_days": 0,
			},
			expectError: true,
		},
		{
			name: "invalid grace_period_days - negative",
			data: map[string]interface{}{
				"grace_period_days": -1,
			},
			expectError: true,
		},
		{
			name: "invalid auto_cancellation_enabled - wrong type",
			data: map[string]interface{}{
				"auto_cancellation_enabled": "true",
			},
			expectError: true,
		},
		{
			name: "invalid auto_cancellation_enabled - wrong type number",
			data: map[string]interface{}{
				"auto_cancellation_enabled": 1,
			},
			expectError: true,
		},
		{
			name:        "nil data",
			data:        nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := NewSubscriptionConfig()
			err := config.FromJSON(tt.data)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if tt.expected != nil {
					if config.GracePeriodDays != tt.expected.GracePeriodDays {
						t.Errorf("expected grace_period_days %d, got %d", tt.expected.GracePeriodDays, config.GracePeriodDays)
					}
					if config.AutoCancellationEnabled != tt.expected.AutoCancellationEnabled {
						t.Errorf("expected auto_cancellation_enabled %t, got %t", tt.expected.AutoCancellationEnabled, config.AutoCancellationEnabled)
					}
				}
			}
		})
	}
}

func TestSubscriptionConfig_ToJSON(t *testing.T) {
	config := NewSubscriptionConfig()
	json, err := config.ToJSON()

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	expected := map[string]interface{}{
		"grace_period_days":         3,
		"auto_cancellation_enabled": false,
	}

	for key, expectedValue := range expected {
		actualValue := json[key]
		// Handle type conversion for numeric values
		if key == "grace_period_days" {
			if int(actualValue.(float64)) != expectedValue {
				t.Errorf("expected %s to be %v, got %v", key, expectedValue, actualValue)
			}
		} else if actualValue != expectedValue {
			t.Errorf("expected %s to be %v, got %v", key, expectedValue, actualValue)
		}
	}
}

func TestSubscriptionConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      *SubscriptionConfig
		expectError bool
	}{
		{
			name: "valid config",
			config: &SubscriptionConfig{
				BaseSettingConfig: BaseSettingConfig{
					Key:         "subscription_config",
					Description: "Configuration for subscription auto-cancellation (grace period and enabled flag)",
				},
				GracePeriodDays:         3,
				AutoCancellationEnabled: false,
			},
			expectError: false,
		},
		{
			name: "valid config with auto cancellation enabled",
			config: &SubscriptionConfig{
				BaseSettingConfig: BaseSettingConfig{
					Key:         "subscription_config",
					Description: "Configuration for subscription auto-cancellation (grace period and enabled flag)",
				},
				GracePeriodDays:         7,
				AutoCancellationEnabled: true,
			},
			expectError: false,
		},
		{
			name: "grace period too low",
			config: &SubscriptionConfig{
				BaseSettingConfig: BaseSettingConfig{
					Key:         "subscription_config",
					Description: "Configuration for subscription auto-cancellation (grace period and enabled flag)",
				},
				GracePeriodDays:         0,
				AutoCancellationEnabled: false,
			},
			expectError: true,
		},
		{
			name: "grace period negative",
			config: &SubscriptionConfig{
				BaseSettingConfig: BaseSettingConfig{
					Key:         "subscription_config",
					Description: "Configuration for subscription auto-cancellation (grace period and enabled flag)",
				},
				GracePeriodDays:         -1,
				AutoCancellationEnabled: false,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestNewSubscriptionConfig(t *testing.T) {
	config := NewSubscriptionConfig()

	if config.GetKey() != "subscription_config" {
		t.Errorf("expected key 'subscription_config', got %s", config.GetKey())
	}

	expectedDescription := "Configuration for subscription auto-cancellation (grace period and enabled flag)"
	if config.GetDescription() != expectedDescription {
		t.Errorf("expected description '%s', got '%s'", expectedDescription, config.GetDescription())
	}

	if config.GracePeriodDays != 3 {
		t.Errorf("expected grace_period_days 3, got %d", config.GracePeriodDays)
	}

	if config.AutoCancellationEnabled != false {
		t.Errorf("expected auto_cancellation_enabled false, got %t", config.AutoCancellationEnabled)
	}
}
