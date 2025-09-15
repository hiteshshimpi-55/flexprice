package settings

import (
	"errors"
	"strings"
	"time"

	ierr "github.com/flexprice/flexprice/internal/errors"
	"github.com/flexprice/flexprice/internal/types"
)

// InvoiceConfig represents the configuration for invoice generation and management
type InvoiceConfig struct {
	BaseSettingConfig

	// Invoice number configuration
	Prefix        string                    `json:"prefix"`
	Format        types.InvoiceNumberFormat `json:"format"`
	StartSequence int                       `json:"start_sequence"`
	Timezone      string                    `json:"timezone"`
	Separator     string                    `json:"separator"`
	SuffixLength  int                       `json:"suffix_length"`

	// Due date configuration
	DueDateDays int `json:"due_date_days"`
}

// NewInvoiceConfig creates a new InvoiceConfig with default values
func NewInvoiceConfig() *InvoiceConfig {
	return &InvoiceConfig{
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
	}
}

// GetDefaultValue returns the default values for invoice configuration
func (ic *InvoiceConfig) GetDefaultValue() map[string]interface{} {
	return map[string]interface{}{
		"prefix":         ic.Prefix,
		"format":         string(ic.Format),
		"start_sequence": ic.StartSequence,
		"timezone":       ic.Timezone,
		"separator":      ic.Separator,
		"suffix_length":  ic.SuffixLength,
		"due_date_days":  ic.DueDateDays,
	}
}

// FromJSON populates the InvoiceConfig from JSON data
func (ic *InvoiceConfig) FromJSON(data map[string]interface{}) error {
	if data == nil {
		return ierr.NewError("invoice_config data cannot be nil").
			WithHint("Please provide valid invoice configuration data").
			Mark(ierr.ErrValidation)
	}

	// Handle partial updates - only validate fields that are present
	// This allows for partial updates like only updating due_date_days

	// Validate prefix if present
	if prefix, err := ValidateStringField(data, "prefix", false, 1, 50); err != nil {
		return err
	} else if prefix != "" {
		ic.Prefix = prefix
	}

	// Validate format if present
	if format, err := ValidateEnumField(data, "format", []string{
		string(types.InvoiceNumberFormatYYYYMM),
		string(types.InvoiceNumberFormatYYYYMMDD),
		string(types.InvoiceNumberFormatYYMMDD),
		string(types.InvoiceNumberFormatYY),
		string(types.InvoiceNumberFormatYYYY),
	}, false); err != nil {
		return err
	} else if format != "" {
		ic.Format = types.InvoiceNumberFormat(format)
	}

	// Validate start_sequence if present
	if startSeq, err := ValidateIntField(data, "start_sequence", false, intPtr(0), nil); err != nil {
		return err
	} else if startSeq != 0 || data["start_sequence"] != nil {
		ic.StartSequence = startSeq
	}

	// Validate timezone if present
	if timezone, err := ValidateStringField(data, "timezone", false, 1, 100); err != nil {
		return err
	} else if timezone != "" {
		// Validate timezone by trying to load it
		if err := validateTimezone(timezone); err != nil {
			return ierr.NewErrorf("invalid timezone '%s': %v", timezone, err).
				WithHintf("Please provide a valid timezone identifier").
				Mark(ierr.ErrValidation)
		}
		ic.Timezone = timezone
	} else if data["timezone"] != nil {
		// If timezone field is present but empty, that's an error
		return ierr.NewError("timezone cannot be empty").
			WithHint("Please provide a valid timezone identifier").
			Mark(ierr.ErrValidation)
	}

	// Validate separator if present
	if separator, err := ValidateStringField(data, "separator", false, 0, 10); err != nil {
		return err
	} else if separator != "" || data["separator"] != nil {
		ic.Separator = separator
	}

	// Validate suffix_length if present
	if suffixLength, err := ValidateIntField(data, "suffix_length", false, intPtr(1), intPtr(10)); err != nil {
		return err
	} else if suffixLength != 0 || data["suffix_length"] != nil {
		ic.SuffixLength = suffixLength
	}

	// Validate due_date_days if present
	if dueDateDays, err := ValidateIntField(data, "due_date_days", false, intPtr(0), nil); err != nil {
		return err
	} else if dueDateDays != 0 || data["due_date_days"] != nil {
		ic.DueDateDays = dueDateDays
	}

	return nil
}

// ToJSON converts the InvoiceConfig to JSON format
func (ic *InvoiceConfig) ToJSON() (map[string]interface{}, error) {
	return ConvertToJSON(ic)
}

// Validate validates the InvoiceConfig
func (ic *InvoiceConfig) Validate() error {
	// Validate prefix
	if strings.TrimSpace(ic.Prefix) == "" {
		return ierr.NewError("invoice_config: 'prefix' cannot be empty").
			WithHint("Please provide a valid invoice number prefix").
			Mark(ierr.ErrValidation)
	}

	// Validate format
	validFormats := []types.InvoiceNumberFormat{
		types.InvoiceNumberFormatYYYYMM,
		types.InvoiceNumberFormatYYYYMMDD,
		types.InvoiceNumberFormatYYMMDD,
		types.InvoiceNumberFormatYY,
		types.InvoiceNumberFormatYYYY,
	}

	valid := false
	for _, validFormat := range validFormats {
		if ic.Format == validFormat {
			valid = true
			break
		}
	}
	if !valid {
		return ierr.NewErrorf("invoice_config: 'format' must be one of %v, got %s", validFormats, ic.Format).
			WithHintf("Please provide a valid invoice number format").
			Mark(ierr.ErrValidation)
	}

	// Validate start_sequence
	if ic.StartSequence < 0 {
		return ierr.NewError("invoice_config: 'start_sequence' must be greater than or equal to 0").
			WithHint("Please provide a valid start sequence number").
			Mark(ierr.ErrValidation)
	}

	// Validate timezone
	if strings.TrimSpace(ic.Timezone) == "" {
		return ierr.NewError("invoice_config: 'timezone' cannot be empty").
			WithHint("Please provide a valid timezone").
			Mark(ierr.ErrValidation)
	}

	// Validate timezone by trying to load it
	if err := validateTimezone(ic.Timezone); err != nil {
		return ierr.NewErrorf("invoice_config: invalid timezone '%s': %v", ic.Timezone, err).
			WithHintf("Please provide a valid timezone identifier").
			Mark(ierr.ErrValidation)
	}

	// Validate suffix_length
	if ic.SuffixLength < 1 || ic.SuffixLength > 10 {
		return ierr.NewError("invoice_config: 'suffix_length' must be between 1 and 10").
			WithHint("Please provide a suffix length between 1 and 10").
			Mark(ierr.ErrValidation)
	}

	// Validate due_date_days
	if ic.DueDateDays < 0 {
		return ierr.NewError("invoice_config: 'due_date_days' must be greater than or equal to 0").
			WithHint("Please provide a valid due date days value").
			Mark(ierr.ErrValidation)
	}

	return nil
}

// validateTimezone validates a timezone by converting abbreviations and checking with time.LoadLocation
func validateTimezone(timezone string) error {
	if timezone == "" {
		return errors.New("timezone cannot be empty")
	}
	resolvedTimezone := resolveTimezone(timezone)
	_, err := time.LoadLocation(resolvedTimezone)
	return err
}

// resolveTimezone converts timezone abbreviation to IANA identifier or returns the input if it's already valid
func resolveTimezone(timezone string) string {
	// First check if it's a known abbreviation
	if ianaName, exists := timezoneAbbreviationMap[strings.ToUpper(timezone)]; exists {
		return ianaName
	}

	// If not an abbreviation, return as-is (might be IANA name already)
	return timezone
}

// timezoneAbbreviationMap maps common three-letter timezone abbreviations to IANA timezone identifiers
var timezoneAbbreviationMap = map[string]string{
	// Indian Standard Time
	"IST": "Asia/Kolkata",

	// US Timezones
	"EST":  "America/New_York",    // Eastern Standard Time
	"CST":  "America/Chicago",     // Central Standard Time
	"MST":  "America/Denver",      // Mountain Standard Time
	"PST":  "America/Los_Angeles", // Pacific Standard Time
	"HST":  "Pacific/Honolulu",    // Hawaii Standard Time
	"AKST": "America/Anchorage",   // Alaska Standard Time

	// European Timezones
	"GMT": "Europe/London", // Greenwich Mean Time
	"CET": "Europe/Berlin", // Central European Time
	"EET": "Europe/Athens", // Eastern European Time
	"WET": "Europe/Lisbon", // Western European Time
	"BST": "Europe/London", // British Summer Time

	// Asia Pacific
	"JST":  "Asia/Tokyo",       // Japan Standard Time
	"KST":  "Asia/Seoul",       // Korea Standard Time
	"CCT":  "Asia/Shanghai",    // China Coast Time (avoiding CST conflict)
	"AEST": "Australia/Sydney", // Australian Eastern Standard Time
	"AWST": "Australia/Perth",  // Australian Western Standard Time

	// Others
	"MSK": "Europe/Moscow",  // Moscow Standard Time
	"CAT": "Africa/Harare",  // Central Africa Time
	"EAT": "Africa/Nairobi", // East Africa Time
	"WAT": "Africa/Lagos",   // West Africa Time
}
