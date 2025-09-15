package settings

import (
	"encoding/json"

	ierr "github.com/flexprice/flexprice/internal/errors"
)

// SettingConfig defines the interface that all setting configurations must implement
type SettingConfig interface {
	// Validate validates the setting configuration
	Validate() error

	// ToJSON converts the setting to JSON format for storage
	ToJSON() (map[string]interface{}, error)

	// FromJSON populates the setting from JSON data
	FromJSON(data map[string]interface{}) error

	// GetKey returns the setting key identifier
	GetKey() string

	// GetDefaultValue returns the default values for this setting
	GetDefaultValue() map[string]interface{}

	// GetDescription returns a human-readable description of this setting
	GetDescription() string
}

// BaseSettingConfig provides common functionality for setting configurations
type BaseSettingConfig struct {
	Key         string
	Description string
}

// GetKey returns the setting key
func (b *BaseSettingConfig) GetKey() string {
	return b.Key
}

// GetDescription returns the setting description
func (b *BaseSettingConfig) GetDescription() string {
	return b.Description
}

// ValidateJSONField validates a JSON field and converts it to the target type
func ValidateJSONField(data map[string]interface{}, fieldName string, target interface{}) error {
	value, exists := data[fieldName]
	if !exists {
		return ierr.NewErrorf("field '%s' is required", fieldName).
			WithHintf("Please provide a value for %s", fieldName).
			Mark(ierr.ErrValidation)
	}

	// Convert to JSON and back to ensure proper type conversion
	jsonBytes, err := json.Marshal(value)
	if err != nil {
		return ierr.WithError(err).
			WithHintf("failed to marshal field '%s'", fieldName).
			Mark(ierr.ErrValidation)
	}

	err = json.Unmarshal(jsonBytes, target)
	if err != nil {
		return ierr.WithError(err).
			WithHintf("failed to unmarshal field '%s' to target type", fieldName).
			Mark(ierr.ErrValidation)
	}

	return nil
}

// ValidateOptionalJSONField validates an optional JSON field and converts it to the target type
func ValidateOptionalJSONField(data map[string]interface{}, fieldName string, target interface{}) error {
	value, exists := data[fieldName]
	if !exists {
		return nil // Field is optional
	}

	// Convert to JSON and back to ensure proper type conversion
	jsonBytes, err := json.Marshal(value)
	if err != nil {
		return ierr.WithError(err).
			WithHintf("failed to marshal field '%s'", fieldName).
			Mark(ierr.ErrValidation)
	}

	err = json.Unmarshal(jsonBytes, target)
	if err != nil {
		return ierr.WithError(err).
			WithHintf("failed to unmarshal field '%s' to target type", fieldName).
			Mark(ierr.ErrValidation)
	}

	return nil
}

// ConvertToJSON converts a struct to JSON map
func ConvertToJSON(v interface{}) (map[string]interface{}, error) {
	jsonBytes, err := json.Marshal(v)
	if err != nil {
		return nil, ierr.WithError(err).
			WithHint("failed to marshal struct to JSON").
			Mark(ierr.ErrValidation)
	}

	var result map[string]interface{}
	err = json.Unmarshal(jsonBytes, &result)
	if err != nil {
		return nil, ierr.WithError(err).
			WithHint("failed to unmarshal JSON to map").
			Mark(ierr.ErrValidation)
	}

	return result, nil
}

// ValidateStringField validates a string field with optional constraints
func ValidateStringField(data map[string]interface{}, fieldName string, required bool, minLength, maxLength int) (string, error) {
	value, exists := data[fieldName]
	if !exists {
		if required {
			return "", ierr.NewErrorf("field '%s' is required", fieldName).
				WithHintf("Please provide a value for %s", fieldName).
				Mark(ierr.ErrValidation)
		}
		return "", nil
	}

	str, ok := value.(string)
	if !ok {
		return "", ierr.NewErrorf("field '%s' must be a string, got %T", fieldName, value).
			WithHintf("Please provide a string value for %s", fieldName).
			Mark(ierr.ErrValidation)
	}

	if minLength > 0 && len(str) < minLength {
		return "", ierr.NewErrorf("field '%s' must be at least %d characters long", fieldName, minLength).
			WithHintf("Please provide a longer value for %s", fieldName).
			Mark(ierr.ErrValidation)
	}

	if maxLength > 0 && len(str) > maxLength {
		return "", ierr.NewErrorf("field '%s' must be at most %d characters long", fieldName, maxLength).
			WithHintf("Please provide a shorter value for %s", fieldName).
			Mark(ierr.ErrValidation)
	}

	return str, nil
}

// ValidateIntField validates an integer field with optional constraints
func ValidateIntField(data map[string]interface{}, fieldName string, required bool, minValue, maxValue *int) (int, error) {
	value, exists := data[fieldName]
	if !exists {
		if required {
			return 0, ierr.NewErrorf("field '%s' is required", fieldName).
				WithHintf("Please provide a value for %s", fieldName).
				Mark(ierr.ErrValidation)
		}
		return 0, nil
	}

	var intValue int
	switch v := value.(type) {
	case int:
		intValue = v
	case float64:
		if v != float64(int(v)) {
			return 0, ierr.NewErrorf("field '%s' must be a whole number", fieldName).
				WithHintf("Please provide a whole number for %s", fieldName).
				Mark(ierr.ErrValidation)
		}
		intValue = int(v)
	default:
		return 0, ierr.NewErrorf("field '%s' must be an integer, got %T", fieldName, value).
			WithHintf("Please provide an integer value for %s", fieldName).
			Mark(ierr.ErrValidation)
	}

	if minValue != nil && intValue < *minValue {
		return 0, ierr.NewErrorf("field '%s' must be at least %d", fieldName, *minValue).
			WithHintf("Please provide a value >= %d for %s", *minValue, fieldName).
			Mark(ierr.ErrValidation)
	}

	if maxValue != nil && intValue > *maxValue {
		return 0, ierr.NewErrorf("field '%s' must be at most %d", fieldName, *maxValue).
			WithHintf("Please provide a value <= %d for %s", *maxValue, fieldName).
			Mark(ierr.ErrValidation)
	}

	return intValue, nil
}

// ValidateBoolField validates a boolean field
func ValidateBoolField(data map[string]interface{}, fieldName string, required bool) (bool, error) {
	value, exists := data[fieldName]
	if !exists {
		if required {
			return false, ierr.NewErrorf("field '%s' is required", fieldName).
				WithHintf("Please provide a value for %s", fieldName).
				Mark(ierr.ErrValidation)
		}
		return false, nil
	}

	boolValue, ok := value.(bool)
	if !ok {
		return false, ierr.NewErrorf("field '%s' must be a boolean, got %T", fieldName, value).
			WithHintf("Please provide a boolean value for %s", fieldName).
			Mark(ierr.ErrValidation)
	}

	return boolValue, nil
}

// ValidateEnumField validates a field against allowed enum values
func ValidateEnumField(data map[string]interface{}, fieldName string, allowedValues []string, required bool) (string, error) {
	value, exists := data[fieldName]
	if !exists {
		if required {
			return "", ierr.NewErrorf("field '%s' is required", fieldName).
				WithHintf("Please provide a value for %s", fieldName).
				Mark(ierr.ErrValidation)
		}
		return "", nil
	}

	str, ok := value.(string)
	if !ok {
		return "", ierr.NewErrorf("field '%s' must be a string, got %T", fieldName, value).
			WithHintf("Please provide a string value for %s", fieldName).
			Mark(ierr.ErrValidation)
	}

	for _, allowed := range allowedValues {
		if str == allowed {
			return str, nil
		}
	}

	return "", ierr.NewErrorf("field '%s' must be one of %v, got '%s'", fieldName, allowedValues, str).
		WithHintf("Please provide one of the allowed values: %v", allowedValues).
		Mark(ierr.ErrValidation)
}

// Helper function to create int pointers
func intPtr(i int) *int {
	return &i
}
