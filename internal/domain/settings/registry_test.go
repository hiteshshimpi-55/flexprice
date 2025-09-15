package settings

import (
	"errors"
	"testing"
)

func TestGetSettingConfig(t *testing.T) {
	tests := []struct {
		name        string
		key         string
		expectError bool
	}{
		{
			name:        "valid invoice config key",
			key:         "invoice_config",
			expectError: false,
		},
		{
			name:        "valid subscription config key",
			key:         "subscription_config",
			expectError: false,
		},
		{
			name:        "invalid key",
			key:         "invalid_key",
			expectError: true,
		},
		{
			name:        "empty key",
			key:         "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := GetSettingConfig(tt.key)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				if config != nil {
					t.Errorf("expected nil config but got %v", config)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if config == nil {
					t.Errorf("expected non-nil config")
				}
				if config.GetKey() != tt.key {
					t.Errorf("expected key %s, got %s", tt.key, config.GetKey())
				}
			}
		})
	}
}

func TestGetAvailableKeys(t *testing.T) {
	keys := GetAvailableKeys()

	expectedKeys := []string{"invoice_config", "subscription_config"}
	if len(keys) != len(expectedKeys) {
		t.Errorf("expected %d keys, got %d", len(expectedKeys), len(keys))
	}

	for _, expectedKey := range expectedKeys {
		found := false
		for _, key := range keys {
			if key == expectedKey {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected key %s not found in available keys", expectedKey)
		}
	}
}

func TestIsValidSettingKey(t *testing.T) {
	tests := []struct {
		key      string
		expected bool
	}{
		{"invoice_config", true},
		{"subscription_config", true},
		{"invalid_key", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			result := IsValidSettingKey(tt.key)
			if result != tt.expected {
				t.Errorf("expected %t, got %t", tt.expected, result)
			}
		})
	}
}

func TestValidateSettingValue(t *testing.T) {
	tests := []struct {
		name        string
		key         string
		value       map[string]interface{}
		expectError bool
	}{
		{
			name: "valid invoice config",
			key:  "invoice_config",
			value: map[string]interface{}{
				"prefix":         "INV",
				"format":         "YYYYMM",
				"start_sequence": 1,
				"timezone":       "UTC",
				"separator":      "-",
				"suffix_length":  5,
				"due_date_days":  1,
			},
			expectError: false,
		},
		{
			name: "valid subscription config",
			key:  "subscription_config",
			value: map[string]interface{}{
				"grace_period_days":         3,
				"auto_cancellation_enabled": true,
			},
			expectError: false,
		},
		{
			name: "invalid key",
			key:  "invalid_key",
			value: map[string]interface{}{
				"some_field": "some_value",
			},
			expectError: true,
		},
		{
			name: "invalid invoice config - missing required field",
			key:  "invoice_config",
			value: map[string]interface{}{
				"prefix": "INV",
				// missing other required fields
			},
			expectError: true,
		},
		{
			name: "invalid subscription config - invalid grace period",
			key:  "subscription_config",
			value: map[string]interface{}{
				"grace_period_days":         0, // invalid: must be >= 1
				"auto_cancellation_enabled": true,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSettingValue(tt.key, tt.value)

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

func TestGetDefaultSettingValue(t *testing.T) {
	tests := []struct {
		name        string
		key         string
		expectError bool
	}{
		{
			name:        "valid invoice config key",
			key:         "invoice_config",
			expectError: false,
		},
		{
			name:        "valid subscription config key",
			key:         "subscription_config",
			expectError: false,
		},
		{
			name:        "invalid key",
			key:         "invalid_key",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, err := GetDefaultSettingValue(tt.key)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				if value != nil {
					t.Errorf("expected nil value but got %v", value)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if value == nil {
					t.Errorf("expected non-nil value")
				}
			}
		})
	}
}

func TestConvertSettingToJSON(t *testing.T) {
	tests := []struct {
		name        string
		key         string
		value       map[string]interface{}
		expectError bool
	}{
		{
			name: "valid invoice config",
			key:  "invoice_config",
			value: map[string]interface{}{
				"prefix":         "INV",
				"format":         "YYYYMM",
				"start_sequence": 1,
				"timezone":       "UTC",
				"separator":      "-",
				"suffix_length":  5,
				"due_date_days":  1,
			},
			expectError: false,
		},
		{
			name: "valid subscription config",
			key:  "subscription_config",
			value: map[string]interface{}{
				"grace_period_days":         3,
				"auto_cancellation_enabled": true,
			},
			expectError: false,
		},
		{
			name: "invalid key",
			key:  "invalid_key",
			value: map[string]interface{}{
				"some_field": "some_value",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ConvertSettingToJSON(tt.key, tt.value)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				if result != nil {
					t.Errorf("expected nil result but got %v", result)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result == nil {
					t.Errorf("expected non-nil result")
				}
			}
		})
	}
}

func TestGetAllSettingConfigs(t *testing.T) {
	configs := GetAllSettingConfigs()

	expectedKeys := []string{"invoice_config", "subscription_config"}
	if len(configs) != len(expectedKeys) {
		t.Errorf("expected %d configs, got %d", len(expectedKeys), len(configs))
	}

	for _, expectedKey := range expectedKeys {
		config, exists := configs[expectedKey]
		if !exists {
			t.Errorf("expected config for key %s not found", expectedKey)
		}
		if config.GetKey() != expectedKey {
			t.Errorf("expected key %s, got %s", expectedKey, config.GetKey())
		}
	}
}

// TestConfig implements SettingConfig for testing
type TestConfig struct {
	BaseSettingConfig
	Value string `json:"value"`
}

func (tc *TestConfig) Validate() error {
	if tc.Value == "" {
		return errors.New("value is required")
	}
	return nil
}

func (tc *TestConfig) ToJSON() (map[string]interface{}, error) {
	return ConvertToJSON(tc)
}

func (tc *TestConfig) FromJSON(data map[string]interface{}) error {
	if data == nil {
		return errors.New("data cannot be nil")
	}

	value, err := ValidateStringField(data, "value", true, 1, 100)
	if err != nil {
		return err
	}
	tc.Value = value
	return nil
}

func (tc *TestConfig) GetDefaultValue() map[string]interface{} {
	return map[string]interface{}{
		"value": "default_value",
	}
}

func TestRegisterSettingConfig(t *testing.T) {
	// Create a test config
	testConfig := &TestConfig{
		BaseSettingConfig: BaseSettingConfig{
			Key:         "test_config",
			Description: "Test configuration",
		},
		Value: "test_value",
	}

	// Test successful registration
	err := RegisterSettingConfig("test_config", testConfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Verify it was registered
	config, err := GetSettingConfig("test_config")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if config.GetKey() != "test_config" {
		t.Errorf("expected key 'test_config', got %s", config.GetKey())
	}

	// Test registration with empty key
	err = RegisterSettingConfig("", testConfig)
	if err == nil {
		t.Errorf("expected error for empty key")
	}

	// Test registration with nil config
	err = RegisterSettingConfig("test_config2", nil)
	if err == nil {
		t.Errorf("expected error for nil config")
	}

	// Test registration with key mismatch
	wrongConfig := &TestConfig{
		BaseSettingConfig: BaseSettingConfig{
			Key:         "different_key",
			Description: "Different key",
		},
		Value: "wrong_value",
	}
	err = RegisterSettingConfig("test_config3", wrongConfig)
	if err == nil {
		t.Errorf("expected error for key mismatch")
	}
}
