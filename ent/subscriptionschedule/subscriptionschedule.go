// Code generated by ent, DO NOT EDIT.

package subscriptionschedule

import (
	"time"

	"entgo.io/ent/dialect/sql"
	"entgo.io/ent/dialect/sql/sqlgraph"
	"github.com/flexprice/flexprice/internal/types"
)

const (
	// Label holds the string label denoting the subscriptionschedule type in the database.
	Label = "subscription_schedule"
	// FieldID holds the string denoting the id field in the database.
	FieldID = "id"
	// FieldTenantID holds the string denoting the tenant_id field in the database.
	FieldTenantID = "tenant_id"
	// FieldStatus holds the string denoting the status field in the database.
	FieldStatus = "status"
	// FieldCreatedAt holds the string denoting the created_at field in the database.
	FieldCreatedAt = "created_at"
	// FieldUpdatedAt holds the string denoting the updated_at field in the database.
	FieldUpdatedAt = "updated_at"
	// FieldCreatedBy holds the string denoting the created_by field in the database.
	FieldCreatedBy = "created_by"
	// FieldUpdatedBy holds the string denoting the updated_by field in the database.
	FieldUpdatedBy = "updated_by"
	// FieldEnvironmentID holds the string denoting the environment_id field in the database.
	FieldEnvironmentID = "environment_id"
	// FieldSubscriptionID holds the string denoting the subscription_id field in the database.
	FieldSubscriptionID = "subscription_id"
	// FieldScheduleStatus holds the string denoting the schedule_status field in the database.
	FieldScheduleStatus = "schedule_status"
	// FieldCurrentPhaseIndex holds the string denoting the current_phase_index field in the database.
	FieldCurrentPhaseIndex = "current_phase_index"
	// FieldEndBehavior holds the string denoting the end_behavior field in the database.
	FieldEndBehavior = "end_behavior"
	// FieldStartDate holds the string denoting the start_date field in the database.
	FieldStartDate = "start_date"
	// FieldMetadata holds the string denoting the metadata field in the database.
	FieldMetadata = "metadata"
	// EdgePhases holds the string denoting the phases edge name in mutations.
	EdgePhases = "phases"
	// EdgeSubscription holds the string denoting the subscription edge name in mutations.
	EdgeSubscription = "subscription"
	// Table holds the table name of the subscriptionschedule in the database.
	Table = "subscription_schedules"
	// PhasesTable is the table that holds the phases relation/edge.
	PhasesTable = "subscription_schedule_phases"
	// PhasesInverseTable is the table name for the SubscriptionSchedulePhase entity.
	// It exists in this package in order to avoid circular dependency with the "subscriptionschedulephase" package.
	PhasesInverseTable = "subscription_schedule_phases"
	// PhasesColumn is the table column denoting the phases relation/edge.
	PhasesColumn = "schedule_id"
	// SubscriptionTable is the table that holds the subscription relation/edge.
	SubscriptionTable = "subscription_schedules"
	// SubscriptionInverseTable is the table name for the Subscription entity.
	// It exists in this package in order to avoid circular dependency with the "subscription" package.
	SubscriptionInverseTable = "subscriptions"
	// SubscriptionColumn is the table column denoting the subscription relation/edge.
	SubscriptionColumn = "subscription_id"
)

// Columns holds all SQL columns for subscriptionschedule fields.
var Columns = []string{
	FieldID,
	FieldTenantID,
	FieldStatus,
	FieldCreatedAt,
	FieldUpdatedAt,
	FieldCreatedBy,
	FieldUpdatedBy,
	FieldEnvironmentID,
	FieldSubscriptionID,
	FieldScheduleStatus,
	FieldCurrentPhaseIndex,
	FieldEndBehavior,
	FieldStartDate,
	FieldMetadata,
}

// ValidColumn reports if the column name is valid (part of the table columns).
func ValidColumn(column string) bool {
	for i := range Columns {
		if column == Columns[i] {
			return true
		}
	}
	return false
}

var (
	// TenantIDValidator is a validator for the "tenant_id" field. It is called by the builders before save.
	TenantIDValidator func(string) error
	// DefaultStatus holds the default value on creation for the "status" field.
	DefaultStatus string
	// DefaultCreatedAt holds the default value on creation for the "created_at" field.
	DefaultCreatedAt func() time.Time
	// DefaultUpdatedAt holds the default value on creation for the "updated_at" field.
	DefaultUpdatedAt func() time.Time
	// UpdateDefaultUpdatedAt holds the default value on update for the "updated_at" field.
	UpdateDefaultUpdatedAt func() time.Time
	// DefaultEnvironmentID holds the default value on creation for the "environment_id" field.
	DefaultEnvironmentID string
	// SubscriptionIDValidator is a validator for the "subscription_id" field. It is called by the builders before save.
	SubscriptionIDValidator func(string) error
	// DefaultScheduleStatus holds the default value on creation for the "schedule_status" field.
	DefaultScheduleStatus types.SubscriptionScheduleStatus
	// DefaultCurrentPhaseIndex holds the default value on creation for the "current_phase_index" field.
	DefaultCurrentPhaseIndex int
	// DefaultEndBehavior holds the default value on creation for the "end_behavior" field.
	DefaultEndBehavior types.ScheduleEndBehavior
	// IDValidator is a validator for the "id" field. It is called by the builders before save.
	IDValidator func(string) error
)

// OrderOption defines the ordering options for the SubscriptionSchedule queries.
type OrderOption func(*sql.Selector)

// ByID orders the results by the id field.
func ByID(opts ...sql.OrderTermOption) OrderOption {
	return sql.OrderByField(FieldID, opts...).ToFunc()
}

// ByTenantID orders the results by the tenant_id field.
func ByTenantID(opts ...sql.OrderTermOption) OrderOption {
	return sql.OrderByField(FieldTenantID, opts...).ToFunc()
}

// ByStatus orders the results by the status field.
func ByStatus(opts ...sql.OrderTermOption) OrderOption {
	return sql.OrderByField(FieldStatus, opts...).ToFunc()
}

// ByCreatedAt orders the results by the created_at field.
func ByCreatedAt(opts ...sql.OrderTermOption) OrderOption {
	return sql.OrderByField(FieldCreatedAt, opts...).ToFunc()
}

// ByUpdatedAt orders the results by the updated_at field.
func ByUpdatedAt(opts ...sql.OrderTermOption) OrderOption {
	return sql.OrderByField(FieldUpdatedAt, opts...).ToFunc()
}

// ByCreatedBy orders the results by the created_by field.
func ByCreatedBy(opts ...sql.OrderTermOption) OrderOption {
	return sql.OrderByField(FieldCreatedBy, opts...).ToFunc()
}

// ByUpdatedBy orders the results by the updated_by field.
func ByUpdatedBy(opts ...sql.OrderTermOption) OrderOption {
	return sql.OrderByField(FieldUpdatedBy, opts...).ToFunc()
}

// ByEnvironmentID orders the results by the environment_id field.
func ByEnvironmentID(opts ...sql.OrderTermOption) OrderOption {
	return sql.OrderByField(FieldEnvironmentID, opts...).ToFunc()
}

// BySubscriptionID orders the results by the subscription_id field.
func BySubscriptionID(opts ...sql.OrderTermOption) OrderOption {
	return sql.OrderByField(FieldSubscriptionID, opts...).ToFunc()
}

// ByScheduleStatus orders the results by the schedule_status field.
func ByScheduleStatus(opts ...sql.OrderTermOption) OrderOption {
	return sql.OrderByField(FieldScheduleStatus, opts...).ToFunc()
}

// ByCurrentPhaseIndex orders the results by the current_phase_index field.
func ByCurrentPhaseIndex(opts ...sql.OrderTermOption) OrderOption {
	return sql.OrderByField(FieldCurrentPhaseIndex, opts...).ToFunc()
}

// ByEndBehavior orders the results by the end_behavior field.
func ByEndBehavior(opts ...sql.OrderTermOption) OrderOption {
	return sql.OrderByField(FieldEndBehavior, opts...).ToFunc()
}

// ByStartDate orders the results by the start_date field.
func ByStartDate(opts ...sql.OrderTermOption) OrderOption {
	return sql.OrderByField(FieldStartDate, opts...).ToFunc()
}

// ByPhasesCount orders the results by phases count.
func ByPhasesCount(opts ...sql.OrderTermOption) OrderOption {
	return func(s *sql.Selector) {
		sqlgraph.OrderByNeighborsCount(s, newPhasesStep(), opts...)
	}
}

// ByPhases orders the results by phases terms.
func ByPhases(term sql.OrderTerm, terms ...sql.OrderTerm) OrderOption {
	return func(s *sql.Selector) {
		sqlgraph.OrderByNeighborTerms(s, newPhasesStep(), append([]sql.OrderTerm{term}, terms...)...)
	}
}

// BySubscriptionField orders the results by subscription field.
func BySubscriptionField(field string, opts ...sql.OrderTermOption) OrderOption {
	return func(s *sql.Selector) {
		sqlgraph.OrderByNeighborTerms(s, newSubscriptionStep(), sql.OrderByField(field, opts...))
	}
}
func newPhasesStep() *sqlgraph.Step {
	return sqlgraph.NewStep(
		sqlgraph.From(Table, FieldID),
		sqlgraph.To(PhasesInverseTable, FieldID),
		sqlgraph.Edge(sqlgraph.O2M, false, PhasesTable, PhasesColumn),
	)
}
func newSubscriptionStep() *sqlgraph.Step {
	return sqlgraph.NewStep(
		sqlgraph.From(Table, FieldID),
		sqlgraph.To(SubscriptionInverseTable, FieldID),
		sqlgraph.Edge(sqlgraph.O2O, true, SubscriptionTable, SubscriptionColumn),
	)
}
