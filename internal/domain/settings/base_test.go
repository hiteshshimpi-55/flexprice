package settings

import (
	"testing"
)

func TestValidateStringField(t *testing.T) {
	tests := []struct {
		name        string
		data        map[string]interface{}
		fieldName   string
		required    bool
		minLength   int
		maxLength   int
		expected    string
		expectError bool
	}{
		{
			name:        "valid string field",
			data:        map[string]interface{}{"name": "test"},
			fieldName:   "name",
			required:    true,
			minLength:   1,
			maxLength:   10,
			expected:    "test",
			expectError: false,
		},
		{
			name:        "missing required field",
			data:        map[string]interface{}{},
			fieldName:   "name",
			required:    true,
			minLength:   1,
			maxLength:   10,
			expected:    "",
			expectError: true,
		},
		{
			name:        "missing optional field",
			data:        map[string]interface{}{},
			fieldName:   "name",
			required:    false,
			minLength:   1,
			maxLength:   10,
			expected:    "",
			expectError: false,
		},
		{
			name:        "invalid type",
			data:        map[string]interface{}{"name": 123},
			fieldName:   "name",
			required:    true,
			minLength:   1,
			maxLength:   10,
			expected:    "",
			expectError: true,
		},
		{
			name:        "too short",
			data:        map[string]interface{}{"name": ""},
			fieldName:   "name",
			required:    true,
			minLength:   1,
			maxLength:   10,
			expected:    "",
			expectError: true,
		},
		{
			name:        "too long",
			data:        map[string]interface{}{"name": "verylongstring"},
			fieldName:   "name",
			required:    true,
			minLength:   1,
			maxLength:   5,
			expected:    "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ValidateStringField(tt.data, tt.fieldName, tt.required, tt.minLength, tt.maxLength)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("expected %s, got %s", tt.expected, result)
				}
			}
		})
	}
}

func TestValidateIntField(t *testing.T) {
	tests := []struct {
		name        string
		data        map[string]interface{}
		fieldName   string
		required    bool
		minValue    *int
		maxValue    *int
		expected    int
		expectError bool
	}{
		{
			name:        "valid int field",
			data:        map[string]interface{}{"count": 5},
			fieldName:   "count",
			required:    true,
			minValue:    intPtr(0),
			maxValue:    intPtr(10),
			expected:    5,
			expectError: false,
		},
		{
			name:        "valid float64 field",
			data:        map[string]interface{}{"count": 5.0},
			fieldName:   "count",
			required:    true,
			minValue:    intPtr(0),
			maxValue:    intPtr(10),
			expected:    5,
			expectError: false,
		},
		{
			name:        "invalid float64 field (not whole number)",
			data:        map[string]interface{}{"count": 5.5},
			fieldName:   "count",
			required:    true,
			minValue:    intPtr(0),
			maxValue:    intPtr(10),
			expected:    0,
			expectError: true,
		},
		{
			name:        "invalid type",
			data:        map[string]interface{}{"count": "five"},
			fieldName:   "count",
			required:    true,
			minValue:    intPtr(0),
			maxValue:    intPtr(10),
			expected:    0,
			expectError: true,
		},
		{
			name:        "below minimum",
			data:        map[string]interface{}{"count": -1},
			fieldName:   "count",
			required:    true,
			minValue:    intPtr(0),
			maxValue:    intPtr(10),
			expected:    0,
			expectError: true,
		},
		{
			name:        "above maximum",
			data:        map[string]interface{}{"count": 15},
			fieldName:   "count",
			required:    true,
			minValue:    intPtr(0),
			maxValue:    intPtr(10),
			expected:    0,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ValidateIntField(tt.data, tt.fieldName, tt.required, tt.minValue, tt.maxValue)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("expected %d, got %d", tt.expected, result)
				}
			}
		})
	}
}

func TestValidateBoolField(t *testing.T) {
	tests := []struct {
		name        string
		data        map[string]interface{}
		fieldName   string
		required    bool
		expected    bool
		expectError bool
	}{
		{
			name:        "valid true bool field",
			data:        map[string]interface{}{"enabled": true},
			fieldName:   "enabled",
			required:    true,
			expected:    true,
			expectError: false,
		},
		{
			name:        "valid false bool field",
			data:        map[string]interface{}{"enabled": false},
			fieldName:   "enabled",
			required:    true,
			expected:    false,
			expectError: false,
		},
		{
			name:        "missing required field",
			data:        map[string]interface{}{},
			fieldName:   "enabled",
			required:    true,
			expected:    false,
			expectError: true,
		},
		{
			name:        "missing optional field",
			data:        map[string]interface{}{},
			fieldName:   "enabled",
			required:    false,
			expected:    false,
			expectError: false,
		},
		{
			name:        "invalid type",
			data:        map[string]interface{}{"enabled": "true"},
			fieldName:   "enabled",
			required:    true,
			expected:    false,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ValidateBoolField(tt.data, tt.fieldName, tt.required)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("expected %t, got %t", tt.expected, result)
				}
			}
		})
	}
}

func TestValidateEnumField(t *testing.T) {
	allowedValues := []string{"option1", "option2", "option3"}

	tests := []struct {
		name        string
		data        map[string]interface{}
		fieldName   string
		required    bool
		expected    string
		expectError bool
	}{
		{
			name:        "valid enum field",
			data:        map[string]interface{}{"type": "option1"},
			fieldName:   "type",
			required:    true,
			expected:    "option1",
			expectError: false,
		},
		{
			name:        "missing required field",
			data:        map[string]interface{}{},
			fieldName:   "type",
			required:    true,
			expected:    "",
			expectError: true,
		},
		{
			name:        "missing optional field",
			data:        map[string]interface{}{},
			fieldName:   "type",
			required:    false,
			expected:    "",
			expectError: false,
		},
		{
			name:        "invalid value",
			data:        map[string]interface{}{"type": "invalid"},
			fieldName:   "type",
			required:    true,
			expected:    "",
			expectError: true,
		},
		{
			name:        "invalid type",
			data:        map[string]interface{}{"type": 123},
			fieldName:   "type",
			required:    true,
			expected:    "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ValidateEnumField(tt.data, tt.fieldName, allowedValues, tt.required)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("expected %s, got %s", tt.expected, result)
				}
			}
		})
	}
}

func TestConvertToJSON(t *testing.T) {
	type TestStruct struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	testStruct := TestStruct{
		Name:  "test",
		Value: 42,
	}

	result, err := ConvertToJSON(testStruct)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result["name"] != "test" {
		t.Errorf("expected name to be 'test', got %v", result["name"])
	}

	if result["value"] != float64(42) { // JSON numbers are float64
		t.Errorf("expected value to be 42, got %v", result["value"])
	}
}

func TestValidateJSONField(t *testing.T) {
	tests := []struct {
		name        string
		data        map[string]interface{}
		fieldName   string
		target      interface{}
		expectError bool
	}{
		{
			name:        "valid field",
			data:        map[string]interface{}{"count": 42},
			fieldName:   "count",
			target:      new(int),
			expectError: false,
		},
		{
			name:        "missing field",
			data:        map[string]interface{}{},
			fieldName:   "count",
			target:      new(int),
			expectError: true,
		},
		{
			name:        "invalid JSON",
			data:        map[string]interface{}{"count": func() {}}, // functions can't be marshaled
			fieldName:   "count",
			target:      new(int),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateJSONField(tt.data, tt.fieldName, tt.target)

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

func TestIntPtr(t *testing.T) {
	ptr := intPtr(42)
	if ptr == nil {
		t.Error("expected non-nil pointer")
	}
	if *ptr != 42 {
		t.Errorf("expected 42, got %d", *ptr)
	}
}
