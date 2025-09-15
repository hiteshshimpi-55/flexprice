package settings

import (
	"testing"

	"github.com/flexprice/flexprice/internal/types"
)

func TestInvoiceConfig_GetKey(t *testing.T) {
	config := NewInvoiceConfig()
	if config.GetKey() != "invoice_config" {
		t.Errorf("expected key 'invoice_config', got %s", config.GetKey())
	}
}

func TestInvoiceConfig_GetDescription(t *testing.T) {
	config := NewInvoiceConfig()
	expected := "Configuration for invoice generation and management"
	if config.GetDescription() != expected {
		t.Errorf("expected description '%s', got '%s'", expected, config.GetDescription())
	}
}

func TestInvoiceConfig_GetDefaultValue(t *testing.T) {
	config := NewInvoiceConfig()
	defaultValue := config.GetDefaultValue()

	expected := map[string]interface{}{
		"prefix":         "INV",
		"format":         "YYYYMM",
		"start_sequence": 1,
		"timezone":       "UTC",
		"separator":      "-",
		"suffix_length":  5,
		"due_date_days":  1,
	}

	for key, expectedValue := range expected {
		if defaultValue[key] != expectedValue {
			t.Errorf("expected %s to be %v, got %v", key, expectedValue, defaultValue[key])
		}
	}
}

func TestInvoiceConfig_FromJSON(t *testing.T) {
	tests := []struct {
		name        string
		data        map[string]interface{}
		expectError bool
		expected    *InvoiceConfig
	}{
		{
			name: "valid full config",
			data: map[string]interface{}{
				"prefix":         "TEST",
				"format":         "YYYYMMDD",
				"start_sequence": 100,
				"timezone":       "America/New_York",
				"separator":      "_",
				"suffix_length":  8,
				"due_date_days":  7,
			},
			expectError: false,
			expected: &InvoiceConfig{
				BaseSettingConfig: BaseSettingConfig{
					Key:         "invoice_config",
					Description: "Configuration for invoice generation and management",
				},
				Prefix:        "TEST",
				Format:        types.InvoiceNumberFormatYYYYMMDD,
				StartSequence: 100,
				Timezone:      "America/New_York",
				Separator:     "_",
				SuffixLength:  8,
				DueDateDays:   7,
			},
		},
		{
			name: "partial update - only due_date_days",
			data: map[string]interface{}{
				"due_date_days": 14,
			},
			expectError: false,
			expected: &InvoiceConfig{
				BaseSettingConfig: BaseSettingConfig{
					Key:         "invoice_config",
					Description: "Configuration for invoice generation and management",
				},
				Prefix:        "INV",                           // default
				Format:        types.InvoiceNumberFormatYYYYMM, // default
				StartSequence: 1,                               // default
				Timezone:      "UTC",                           // default
				Separator:     "-",                             // default
				SuffixLength:  5,                               // default
				DueDateDays:   14,                              // updated
			},
		},
		{
			name: "invalid format",
			data: map[string]interface{}{
				"format": "INVALID_FORMAT",
			},
			expectError: true,
		},
		{
			name: "invalid suffix_length - too high",
			data: map[string]interface{}{
				"suffix_length": 15,
			},
			expectError: true,
		},
		{
			name: "invalid suffix_length - too low",
			data: map[string]interface{}{
				"suffix_length": 0,
			},
			expectError: true,
		},
		{
			name: "invalid due_date_days - negative",
			data: map[string]interface{}{
				"due_date_days": -1,
			},
			expectError: true,
		},
		{
			name: "invalid timezone",
			data: map[string]interface{}{
				"timezone": "Invalid/Timezone",
			},
			expectError: true,
		},
		{
			name: "empty timezone",
			data: map[string]interface{}{
				"timezone": "",
			},
			expectError: true, // Empty timezone should be invalid
		},
		{
			name:        "nil data",
			data:        nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := NewInvoiceConfig()
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
					if config.Prefix != tt.expected.Prefix {
						t.Errorf("expected prefix %s, got %s", tt.expected.Prefix, config.Prefix)
					}
					if config.Format != tt.expected.Format {
						t.Errorf("expected format %s, got %s", tt.expected.Format, config.Format)
					}
					if config.StartSequence != tt.expected.StartSequence {
						t.Errorf("expected start_sequence %d, got %d", tt.expected.StartSequence, config.StartSequence)
					}
					if config.Timezone != tt.expected.Timezone {
						t.Errorf("expected timezone %s, got %s", tt.expected.Timezone, config.Timezone)
					}
					if config.Separator != tt.expected.Separator {
						t.Errorf("expected separator %s, got %s", tt.expected.Separator, config.Separator)
					}
					if config.SuffixLength != tt.expected.SuffixLength {
						t.Errorf("expected suffix_length %d, got %d", tt.expected.SuffixLength, config.SuffixLength)
					}
					if config.DueDateDays != tt.expected.DueDateDays {
						t.Errorf("expected due_date_days %d, got %d", tt.expected.DueDateDays, config.DueDateDays)
					}
				}
			}
		})
	}
}

func TestInvoiceConfig_ToJSON(t *testing.T) {
	config := NewInvoiceConfig()
	json, err := config.ToJSON()

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	expected := map[string]interface{}{
		"prefix":         "INV",
		"format":         "YYYYMM",
		"start_sequence": 1,
		"timezone":       "UTC",
		"separator":      "-",
		"suffix_length":  5,
		"due_date_days":  1,
	}

	for key, expectedValue := range expected {
		actualValue := json[key]
		// Handle type conversion for numeric values
		if key == "start_sequence" || key == "suffix_length" || key == "due_date_days" {
			if int(actualValue.(float64)) != expectedValue {
				t.Errorf("expected %s to be %v, got %v", key, expectedValue, actualValue)
			}
		} else if actualValue != expectedValue {
			t.Errorf("expected %s to be %v, got %v", key, expectedValue, actualValue)
		}
	}
}

func TestInvoiceConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      *InvoiceConfig
		expectError bool
	}{
		{
			name: "valid config",
			config: &InvoiceConfig{
				BaseSettingConfig: BaseSettingConfig{
					Key:         "invoice_config",
					Description: "Configuration for invoice generation and management",
				},
				Prefix:        "INV",
				Format:        types.InvoiceNumberFormatYYYYMM,
				StartSequence: 1,
				Timezone:      "UTC",
				Separator:     "-",
				SuffixLength:  5,
				DueDateDays:   1,
			},
			expectError: false,
		},
		{
			name: "empty prefix",
			config: &InvoiceConfig{
				BaseSettingConfig: BaseSettingConfig{
					Key:         "invoice_config",
					Description: "Configuration for invoice generation and management",
				},
				Prefix:        "",
				Format:        types.InvoiceNumberFormatYYYYMM,
				StartSequence: 1,
				Timezone:      "UTC",
				Separator:     "-",
				SuffixLength:  5,
				DueDateDays:   1,
			},
			expectError: true,
		},
		{
			name: "invalid format",
			config: &InvoiceConfig{
				BaseSettingConfig: BaseSettingConfig{
					Key:         "invoice_config",
					Description: "Configuration for invoice generation and management",
				},
				Prefix:        "INV",
				Format:        "INVALID_FORMAT",
				StartSequence: 1,
				Timezone:      "UTC",
				Separator:     "-",
				SuffixLength:  5,
				DueDateDays:   1,
			},
			expectError: true,
		},
		{
			name: "negative start sequence",
			config: &InvoiceConfig{
				BaseSettingConfig: BaseSettingConfig{
					Key:         "invoice_config",
					Description: "Configuration for invoice generation and management",
				},
				Prefix:        "INV",
				Format:        types.InvoiceNumberFormatYYYYMM,
				StartSequence: -1,
				Timezone:      "UTC",
				Separator:     "-",
				SuffixLength:  5,
				DueDateDays:   1,
			},
			expectError: true,
		},
		{
			name: "empty timezone",
			config: &InvoiceConfig{
				BaseSettingConfig: BaseSettingConfig{
					Key:         "invoice_config",
					Description: "Configuration for invoice generation and management",
				},
				Prefix:        "INV",
				Format:        types.InvoiceNumberFormatYYYYMM,
				StartSequence: 1,
				Timezone:      "",
				Separator:     "-",
				SuffixLength:  5,
				DueDateDays:   1,
			},
			expectError: true,
		},
		{
			name: "invalid timezone",
			config: &InvoiceConfig{
				BaseSettingConfig: BaseSettingConfig{
					Key:         "invoice_config",
					Description: "Configuration for invoice generation and management",
				},
				Prefix:        "INV",
				Format:        types.InvoiceNumberFormatYYYYMM,
				StartSequence: 1,
				Timezone:      "Invalid/Timezone",
				Separator:     "-",
				SuffixLength:  5,
				DueDateDays:   1,
			},
			expectError: true,
		},
		{
			name: "suffix length too low",
			config: &InvoiceConfig{
				BaseSettingConfig: BaseSettingConfig{
					Key:         "invoice_config",
					Description: "Configuration for invoice generation and management",
				},
				Prefix:        "INV",
				Format:        types.InvoiceNumberFormatYYYYMM,
				StartSequence: 1,
				Timezone:      "UTC",
				Separator:     "-",
				SuffixLength:  0,
				DueDateDays:   1,
			},
			expectError: true,
		},
		{
			name: "suffix length too high",
			config: &InvoiceConfig{
				BaseSettingConfig: BaseSettingConfig{
					Key:         "invoice_config",
					Description: "Configuration for invoice generation and management",
				},
				Prefix:        "INV",
				Format:        types.InvoiceNumberFormatYYYYMM,
				StartSequence: 1,
				Timezone:      "UTC",
				Separator:     "-",
				SuffixLength:  15,
				DueDateDays:   1,
			},
			expectError: true,
		},
		{
			name: "negative due date days",
			config: &InvoiceConfig{
				BaseSettingConfig: BaseSettingConfig{
					Key:         "invoice_config",
					Description: "Configuration for invoice generation and management",
				},
				Prefix:        "INV",
				Format:        types.InvoiceNumberFormatYYYYMM,
				StartSequence: 1,
				Timezone:      "UTC",
				Separator:     "-",
				SuffixLength:  5,
				DueDateDays:   -1,
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

func TestValidateTimezone(t *testing.T) {
	tests := []struct {
		timezone    string
		expectError bool
	}{
		{"UTC", false},
		{"America/New_York", false},
		{"Europe/London", false},
		{"Asia/Tokyo", false},
		{"IST", false}, // abbreviation
		{"EST", false}, // abbreviation
		{"Invalid/Timezone", true},
		{"", true}, // Empty string should be invalid
	}

	for _, tt := range tests {
		t.Run(tt.timezone, func(t *testing.T) {
			err := validateTimezone(tt.timezone)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error for timezone %s but got none", tt.timezone)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error for timezone %s: %v", tt.timezone, err)
				}
			}
		})
	}
}

func TestResolveTimezone(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"UTC", "UTC"},
		{"America/New_York", "America/New_York"},
		{"IST", "Asia/Kolkata"},
		{"EST", "America/New_York"},
		{"CST", "America/Chicago"},
		{"MST", "America/Denver"},
		{"PST", "America/Los_Angeles"},
		{"GMT", "Europe/London"},
		{"JST", "Asia/Tokyo"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := resolveTimezone(tt.input)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}
