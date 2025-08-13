package types

// SettingsFilter represents the filter criteria for querying settings
type SettingsFilter struct {
	*QueryFilter
	*TimeRangeFilter

	// Filters allows complex filtering based on multiple fields
	Filters []*FilterCondition `json:"filters,omitempty" form:"filters" validate:"omitempty"`
	Sort    []*SortCondition   `json:"sort,omitempty" form:"sort" validate:"omitempty"`

	// Specific filters
	Keys       []string `json:"keys,omitempty" form:"keys" validate:"omitempty"`
	Key        string   `json:"key,omitempty" form:"key" validate:"omitempty"`
	KeyPattern string   `json:"key_pattern,omitempty" form:"key_pattern" validate:"omitempty"`
	SettingIDs []string `json:"setting_ids,omitempty" form:"setting_ids" validate:"omitempty"`
}

// NewSettingsFilter creates a new SettingsFilter with default values
func NewSettingsFilter() *SettingsFilter {
	return &SettingsFilter{
		QueryFilter:     NewDefaultQueryFilter(),
		TimeRangeFilter: NewDefaultTimeRangeFilter(),
	}
}

// GetLimit implements BaseFilter interface
func (f *SettingsFilter) GetLimit() int {
	if f.QueryFilter == nil {
		return NewDefaultQueryFilter().GetLimit()
	}
	return f.QueryFilter.GetLimit()
}

// GetOffset implements BaseFilter interface
func (f *SettingsFilter) GetOffset() int {
	if f.QueryFilter == nil {
		return NewDefaultQueryFilter().GetOffset()
	}
	return f.QueryFilter.GetOffset()
}

// GetStatus implements BaseFilter interface
func (f *SettingsFilter) GetStatus() string {
	if f.QueryFilter == nil {
		return NewDefaultQueryFilter().GetStatus()
	}
	return f.QueryFilter.GetStatus()
}

// GetSortBy implements BaseFilter interface
func (f *SettingsFilter) GetSortBy() string {
	if f.QueryFilter == nil {
		return NewDefaultQueryFilter().GetSortBy()
	}
	return f.QueryFilter.GetSortBy()
}

// GetSortOrder implements BaseFilter interface
func (f *SettingsFilter) GetSortOrder() string {
	if f.QueryFilter == nil {
		return NewDefaultQueryFilter().GetSortOrder()
	}
	return f.QueryFilter.GetSortOrder()
}

// GetExpand implements BaseFilter interface
func (f *SettingsFilter) GetExpand() Expand {
	if f.QueryFilter == nil {
		return NewDefaultQueryFilter().GetExpand()
	}
	return f.QueryFilter.GetExpand()
}

// IsUnlimited implements BaseFilter interface
func (f *SettingsFilter) IsUnlimited() bool {
	if f.QueryFilter == nil {
		return NewDefaultQueryFilter().IsUnlimited()
	}
	return f.QueryFilter.IsUnlimited()
}
