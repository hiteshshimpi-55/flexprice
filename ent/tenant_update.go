// Code generated by ent, DO NOT EDIT.

package ent

import (
	"context"
	"errors"
	"fmt"
	"time"

	"entgo.io/ent/dialect/sql"
	"entgo.io/ent/dialect/sql/sqlgraph"
	"entgo.io/ent/schema/field"
	"github.com/flexprice/flexprice/ent/predicate"
	"github.com/flexprice/flexprice/ent/schema"
	"github.com/flexprice/flexprice/ent/tenant"
)

// TenantUpdate is the builder for updating Tenant entities.
type TenantUpdate struct {
	config
	hooks    []Hook
	mutation *TenantMutation
}

// Where appends a list predicates to the TenantUpdate builder.
func (tu *TenantUpdate) Where(ps ...predicate.Tenant) *TenantUpdate {
	tu.mutation.Where(ps...)
	return tu
}

// SetName sets the "name" field.
func (tu *TenantUpdate) SetName(s string) *TenantUpdate {
	tu.mutation.SetName(s)
	return tu
}

// SetNillableName sets the "name" field if the given value is not nil.
func (tu *TenantUpdate) SetNillableName(s *string) *TenantUpdate {
	if s != nil {
		tu.SetName(*s)
	}
	return tu
}

// SetStatus sets the "status" field.
func (tu *TenantUpdate) SetStatus(s string) *TenantUpdate {
	tu.mutation.SetStatus(s)
	return tu
}

// SetNillableStatus sets the "status" field if the given value is not nil.
func (tu *TenantUpdate) SetNillableStatus(s *string) *TenantUpdate {
	if s != nil {
		tu.SetStatus(*s)
	}
	return tu
}

// SetUpdatedAt sets the "updated_at" field.
func (tu *TenantUpdate) SetUpdatedAt(t time.Time) *TenantUpdate {
	tu.mutation.SetUpdatedAt(t)
	return tu
}

// SetBillingDetails sets the "billing_details" field.
func (tu *TenantUpdate) SetBillingDetails(sbd schema.TenantBillingDetails) *TenantUpdate {
	tu.mutation.SetBillingDetails(sbd)
	return tu
}

// SetNillableBillingDetails sets the "billing_details" field if the given value is not nil.
func (tu *TenantUpdate) SetNillableBillingDetails(sbd *schema.TenantBillingDetails) *TenantUpdate {
	if sbd != nil {
		tu.SetBillingDetails(*sbd)
	}
	return tu
}

// ClearBillingDetails clears the value of the "billing_details" field.
func (tu *TenantUpdate) ClearBillingDetails() *TenantUpdate {
	tu.mutation.ClearBillingDetails()
	return tu
}

// SetMetadata sets the "metadata" field.
func (tu *TenantUpdate) SetMetadata(m map[string]string) *TenantUpdate {
	tu.mutation.SetMetadata(m)
	return tu
}

// ClearMetadata clears the value of the "metadata" field.
func (tu *TenantUpdate) ClearMetadata() *TenantUpdate {
	tu.mutation.ClearMetadata()
	return tu
}

// Mutation returns the TenantMutation object of the builder.
func (tu *TenantUpdate) Mutation() *TenantMutation {
	return tu.mutation
}

// Save executes the query and returns the number of nodes affected by the update operation.
func (tu *TenantUpdate) Save(ctx context.Context) (int, error) {
	tu.defaults()
	return withHooks(ctx, tu.sqlSave, tu.mutation, tu.hooks)
}

// SaveX is like Save, but panics if an error occurs.
func (tu *TenantUpdate) SaveX(ctx context.Context) int {
	affected, err := tu.Save(ctx)
	if err != nil {
		panic(err)
	}
	return affected
}

// Exec executes the query.
func (tu *TenantUpdate) Exec(ctx context.Context) error {
	_, err := tu.Save(ctx)
	return err
}

// ExecX is like Exec, but panics if an error occurs.
func (tu *TenantUpdate) ExecX(ctx context.Context) {
	if err := tu.Exec(ctx); err != nil {
		panic(err)
	}
}

// defaults sets the default values of the builder before save.
func (tu *TenantUpdate) defaults() {
	if _, ok := tu.mutation.UpdatedAt(); !ok {
		v := tenant.UpdateDefaultUpdatedAt()
		tu.mutation.SetUpdatedAt(v)
	}
}

// check runs all checks and user-defined validators on the builder.
func (tu *TenantUpdate) check() error {
	if v, ok := tu.mutation.Name(); ok {
		if err := tenant.NameValidator(v); err != nil {
			return &ValidationError{Name: "name", err: fmt.Errorf(`ent: validator failed for field "Tenant.name": %w`, err)}
		}
	}
	return nil
}

func (tu *TenantUpdate) sqlSave(ctx context.Context) (n int, err error) {
	if err := tu.check(); err != nil {
		return n, err
	}
	_spec := sqlgraph.NewUpdateSpec(tenant.Table, tenant.Columns, sqlgraph.NewFieldSpec(tenant.FieldID, field.TypeString))
	if ps := tu.mutation.predicates; len(ps) > 0 {
		_spec.Predicate = func(selector *sql.Selector) {
			for i := range ps {
				ps[i](selector)
			}
		}
	}
	if value, ok := tu.mutation.Name(); ok {
		_spec.SetField(tenant.FieldName, field.TypeString, value)
	}
	if value, ok := tu.mutation.Status(); ok {
		_spec.SetField(tenant.FieldStatus, field.TypeString, value)
	}
	if value, ok := tu.mutation.UpdatedAt(); ok {
		_spec.SetField(tenant.FieldUpdatedAt, field.TypeTime, value)
	}
	if value, ok := tu.mutation.BillingDetails(); ok {
		_spec.SetField(tenant.FieldBillingDetails, field.TypeJSON, value)
	}
	if tu.mutation.BillingDetailsCleared() {
		_spec.ClearField(tenant.FieldBillingDetails, field.TypeJSON)
	}
	if value, ok := tu.mutation.Metadata(); ok {
		_spec.SetField(tenant.FieldMetadata, field.TypeJSON, value)
	}
	if tu.mutation.MetadataCleared() {
		_spec.ClearField(tenant.FieldMetadata, field.TypeJSON)
	}
	if n, err = sqlgraph.UpdateNodes(ctx, tu.driver, _spec); err != nil {
		if _, ok := err.(*sqlgraph.NotFoundError); ok {
			err = &NotFoundError{tenant.Label}
		} else if sqlgraph.IsConstraintError(err) {
			err = &ConstraintError{msg: err.Error(), wrap: err}
		}
		return 0, err
	}
	tu.mutation.done = true
	return n, nil
}

// TenantUpdateOne is the builder for updating a single Tenant entity.
type TenantUpdateOne struct {
	config
	fields   []string
	hooks    []Hook
	mutation *TenantMutation
}

// SetName sets the "name" field.
func (tuo *TenantUpdateOne) SetName(s string) *TenantUpdateOne {
	tuo.mutation.SetName(s)
	return tuo
}

// SetNillableName sets the "name" field if the given value is not nil.
func (tuo *TenantUpdateOne) SetNillableName(s *string) *TenantUpdateOne {
	if s != nil {
		tuo.SetName(*s)
	}
	return tuo
}

// SetStatus sets the "status" field.
func (tuo *TenantUpdateOne) SetStatus(s string) *TenantUpdateOne {
	tuo.mutation.SetStatus(s)
	return tuo
}

// SetNillableStatus sets the "status" field if the given value is not nil.
func (tuo *TenantUpdateOne) SetNillableStatus(s *string) *TenantUpdateOne {
	if s != nil {
		tuo.SetStatus(*s)
	}
	return tuo
}

// SetUpdatedAt sets the "updated_at" field.
func (tuo *TenantUpdateOne) SetUpdatedAt(t time.Time) *TenantUpdateOne {
	tuo.mutation.SetUpdatedAt(t)
	return tuo
}

// SetBillingDetails sets the "billing_details" field.
func (tuo *TenantUpdateOne) SetBillingDetails(sbd schema.TenantBillingDetails) *TenantUpdateOne {
	tuo.mutation.SetBillingDetails(sbd)
	return tuo
}

// SetNillableBillingDetails sets the "billing_details" field if the given value is not nil.
func (tuo *TenantUpdateOne) SetNillableBillingDetails(sbd *schema.TenantBillingDetails) *TenantUpdateOne {
	if sbd != nil {
		tuo.SetBillingDetails(*sbd)
	}
	return tuo
}

// ClearBillingDetails clears the value of the "billing_details" field.
func (tuo *TenantUpdateOne) ClearBillingDetails() *TenantUpdateOne {
	tuo.mutation.ClearBillingDetails()
	return tuo
}

// SetMetadata sets the "metadata" field.
func (tuo *TenantUpdateOne) SetMetadata(m map[string]string) *TenantUpdateOne {
	tuo.mutation.SetMetadata(m)
	return tuo
}

// ClearMetadata clears the value of the "metadata" field.
func (tuo *TenantUpdateOne) ClearMetadata() *TenantUpdateOne {
	tuo.mutation.ClearMetadata()
	return tuo
}

// Mutation returns the TenantMutation object of the builder.
func (tuo *TenantUpdateOne) Mutation() *TenantMutation {
	return tuo.mutation
}

// Where appends a list predicates to the TenantUpdate builder.
func (tuo *TenantUpdateOne) Where(ps ...predicate.Tenant) *TenantUpdateOne {
	tuo.mutation.Where(ps...)
	return tuo
}

// Select allows selecting one or more fields (columns) of the returned entity.
// The default is selecting all fields defined in the entity schema.
func (tuo *TenantUpdateOne) Select(field string, fields ...string) *TenantUpdateOne {
	tuo.fields = append([]string{field}, fields...)
	return tuo
}

// Save executes the query and returns the updated Tenant entity.
func (tuo *TenantUpdateOne) Save(ctx context.Context) (*Tenant, error) {
	tuo.defaults()
	return withHooks(ctx, tuo.sqlSave, tuo.mutation, tuo.hooks)
}

// SaveX is like Save, but panics if an error occurs.
func (tuo *TenantUpdateOne) SaveX(ctx context.Context) *Tenant {
	node, err := tuo.Save(ctx)
	if err != nil {
		panic(err)
	}
	return node
}

// Exec executes the query on the entity.
func (tuo *TenantUpdateOne) Exec(ctx context.Context) error {
	_, err := tuo.Save(ctx)
	return err
}

// ExecX is like Exec, but panics if an error occurs.
func (tuo *TenantUpdateOne) ExecX(ctx context.Context) {
	if err := tuo.Exec(ctx); err != nil {
		panic(err)
	}
}

// defaults sets the default values of the builder before save.
func (tuo *TenantUpdateOne) defaults() {
	if _, ok := tuo.mutation.UpdatedAt(); !ok {
		v := tenant.UpdateDefaultUpdatedAt()
		tuo.mutation.SetUpdatedAt(v)
	}
}

// check runs all checks and user-defined validators on the builder.
func (tuo *TenantUpdateOne) check() error {
	if v, ok := tuo.mutation.Name(); ok {
		if err := tenant.NameValidator(v); err != nil {
			return &ValidationError{Name: "name", err: fmt.Errorf(`ent: validator failed for field "Tenant.name": %w`, err)}
		}
	}
	return nil
}

func (tuo *TenantUpdateOne) sqlSave(ctx context.Context) (_node *Tenant, err error) {
	if err := tuo.check(); err != nil {
		return _node, err
	}
	_spec := sqlgraph.NewUpdateSpec(tenant.Table, tenant.Columns, sqlgraph.NewFieldSpec(tenant.FieldID, field.TypeString))
	id, ok := tuo.mutation.ID()
	if !ok {
		return nil, &ValidationError{Name: "id", err: errors.New(`ent: missing "Tenant.id" for update`)}
	}
	_spec.Node.ID.Value = id
	if fields := tuo.fields; len(fields) > 0 {
		_spec.Node.Columns = make([]string, 0, len(fields))
		_spec.Node.Columns = append(_spec.Node.Columns, tenant.FieldID)
		for _, f := range fields {
			if !tenant.ValidColumn(f) {
				return nil, &ValidationError{Name: f, err: fmt.Errorf("ent: invalid field %q for query", f)}
			}
			if f != tenant.FieldID {
				_spec.Node.Columns = append(_spec.Node.Columns, f)
			}
		}
	}
	if ps := tuo.mutation.predicates; len(ps) > 0 {
		_spec.Predicate = func(selector *sql.Selector) {
			for i := range ps {
				ps[i](selector)
			}
		}
	}
	if value, ok := tuo.mutation.Name(); ok {
		_spec.SetField(tenant.FieldName, field.TypeString, value)
	}
	if value, ok := tuo.mutation.Status(); ok {
		_spec.SetField(tenant.FieldStatus, field.TypeString, value)
	}
	if value, ok := tuo.mutation.UpdatedAt(); ok {
		_spec.SetField(tenant.FieldUpdatedAt, field.TypeTime, value)
	}
	if value, ok := tuo.mutation.BillingDetails(); ok {
		_spec.SetField(tenant.FieldBillingDetails, field.TypeJSON, value)
	}
	if tuo.mutation.BillingDetailsCleared() {
		_spec.ClearField(tenant.FieldBillingDetails, field.TypeJSON)
	}
	if value, ok := tuo.mutation.Metadata(); ok {
		_spec.SetField(tenant.FieldMetadata, field.TypeJSON, value)
	}
	if tuo.mutation.MetadataCleared() {
		_spec.ClearField(tenant.FieldMetadata, field.TypeJSON)
	}
	_node = &Tenant{config: tuo.config}
	_spec.Assign = _node.assignValues
	_spec.ScanValues = _node.scanValues
	if err = sqlgraph.UpdateNode(ctx, tuo.driver, _spec); err != nil {
		if _, ok := err.(*sqlgraph.NotFoundError); ok {
			err = &NotFoundError{tenant.Label}
		} else if sqlgraph.IsConstraintError(err) {
			err = &ConstraintError{msg: err.Error(), wrap: err}
		}
		return nil, err
	}
	tuo.mutation.done = true
	return _node, nil
}
