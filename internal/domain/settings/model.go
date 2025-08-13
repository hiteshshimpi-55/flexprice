package settings

import (
	"encoding/json"

	"github.com/flexprice/flexprice/ent"
	ierr "github.com/flexprice/flexprice/internal/errors"
	"github.com/flexprice/flexprice/internal/types"
)

// Setting represents a tenant and environment specific configuration setting
type Setting struct {
	// ID is the unique identifier for the setting
	ID string `json:"id"`

	// Key is the setting key
	Key string `json:"key"`

	// Value is the JSON value of the setting
	Value map[string]interface{} `json:"value"`

	// EnvironmentID is the environment identifier for the setting
	EnvironmentID string `json:"environment_id"`

	types.BaseModel
}

// FromEnt converts an ent setting to a domain setting
func FromEnt(s *ent.Settings) *Setting {
	if s == nil {
		return nil
	}

	// Convert the JSON value from map[string]string to map[string]interface{}
	value := make(map[string]interface{})
	if s.Value != nil {
		for k, v := range s.Value {
			// Try to unmarshal as JSON, fallback to string
			var jsonValue interface{}
			if err := json.Unmarshal([]byte(v), &jsonValue); err != nil {
				jsonValue = v // Fallback to string value
			}
			value[k] = jsonValue
		}
	}

	return &Setting{
		ID:            s.ID,
		Key:           s.Key,
		Value:         value,
		EnvironmentID: s.EnvironmentID,
		BaseModel: types.BaseModel{
			TenantID:  s.TenantID,
			Status:    types.Status(s.Status),
			CreatedAt: s.CreatedAt,
			UpdatedAt: s.UpdatedAt,
			CreatedBy: s.CreatedBy,
			UpdatedBy: s.UpdatedBy,
		},
	}
}

// FromEntList converts a list of ent settings to domain settings
func FromEntList(settings []*ent.Settings) []*Setting {
	if settings == nil {
		return nil
	}

	result := make([]*Setting, len(settings))
	for i, s := range settings {
		result[i] = FromEnt(s)
	}

	return result
}

// ToEntValue converts the domain value to ent-compatible format
func (s *Setting) ToEntValue() (map[string]string, error) {
	if s.Value == nil {
		return nil, nil
	}

	entValue := make(map[string]string)
	for k, v := range s.Value {
		// Convert interface{} to JSON string
		jsonBytes, err := json.Marshal(v)
		if err != nil {
			return nil, ierr.WithError(err).
				WithHintf("failed to marshal value for key '%s'", k).
				Mark(ierr.ErrValidation)
		}
		entValue[k] = string(jsonBytes)
	}

	return entValue, nil
}

// GetValue retrieves a value by key and unmarshals it into the target
func (s *Setting) GetValue(key string, target interface{}) error {
	if s.Value == nil {
		return ierr.NewErrorf("no value found for key '%s'", key).
			Mark(ierr.ErrNotFound)
	}

	value, exists := s.Value[key]
	if !exists {
		return ierr.NewErrorf("key '%s' not found in setting", key).
			Mark(ierr.ErrNotFound)
	}

	// Marshal and unmarshal to convert interface{} to target type
	jsonBytes, err := json.Marshal(value)
	if err != nil {
		return ierr.WithError(err).
			WithHintf("failed to marshal value for key '%s'", key).
			Mark(ierr.ErrValidation)
	}

	err = json.Unmarshal(jsonBytes, target)
	if err != nil {
		return ierr.WithError(err).
			WithHintf("failed to unmarshal value for key '%s'", key).
			Mark(ierr.ErrValidation)
	}

	return nil
}

// SetValue sets a value for a specific key
func (s *Setting) SetValue(key string, value interface{}) {
	if s.Value == nil {
		s.Value = make(map[string]interface{})
	}
	s.Value[key] = value
}

// Validate validates the setting
func (s *Setting) Validate() error {
	if s.Key == "" {
		return ierr.NewError("setting key is required").
			Mark(ierr.ErrValidation)
	}

	return nil
}
