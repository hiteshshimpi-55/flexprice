# Migration Scripts Documentation

This document describes the migration scripts available for performing plan grouping and subscription migration operations.

## Available Migration Scripts

### 1. Complete Migration (`complete-migration`)

This is the comprehensive migration script that performs all four required steps in sequence:

1. **Create a master plan**
2. **Add prices to it with group associated**
3. **Cancel existing subscriptions by modifying current_period_end and end_date with t1 and cancel_at_period_end true**
4. **Create new subscriptions with master plan with start day as t1 and billing cycle calendar**

#### Usage

```bash
# Set required environment variables
export TENANT_ID="your-tenant-id"
export ENVIRONMENT_ID="your-environment-id"
export T1_TIME="2024-01-01T00:00:00Z"
export ENTITY_GROUPS='{"entity_id_1":"group_1","entity_id_2":"group_2"}'
export DRY_RUN_MODE="true"  # Set to "false" for actual execution

# Run the complete migration
./scripts/complete_migration.sh
```

#### Environment Variables

- **TENANT_ID** (required): The tenant identifier
- **ENVIRONMENT_ID** (required): The environment identifier  
- **T1_TIME** (required): The cutoff time for subscription cancellation and new subscription start (ISO 8601 format)
- **ENTITY_GROUPS** (required): JSON mapping of entity IDs to group IDs
- **DRY_RUN_MODE** (optional): Set to "true" for dry run, "false" for actual execution (default: "true")

#### Example

```bash
export TENANT_ID="tenant_123"
export ENVIRONMENT_ID="env_456"
export T1_TIME="2024-12-01T00:00:00Z"
export ENTITY_GROUPS='{"price_entity_1":"group_a","price_entity_2":"group_b","price_entity_3":"group_a"}'
export DRY_RUN_MODE="false"

./scripts/complete_migration.sh
```

### 2. Plan Grouping Only (`plan-grouping`)

This script only performs the plan and price grouping operations:

1. Create a master plan
2. Add prices to it with group associations

#### Usage

```bash
# Using the main script runner
go run ./scripts/main.go -cmd=plan-grouping \
  -tenant-id="your-tenant-id" \
  -environment-id="your-environment-id"

# Or set environment variables
export TENANT_ID="your-tenant-id"
export ENVIRONMENT_ID="your-environment-id"
export ENTITY_GROUPS='{"entity_id_1":"group_1","entity_id_2":"group_2"}'
export DRY_RUN_MODE="true"

go run ./scripts/main.go -cmd=plan-grouping
```

### 3. Subscription Migration Only (`subscription-migration`)

This script only performs subscription migration operations:

1. Cancel existing subscriptions at T1
2. Create new subscriptions with master plan

#### Usage

```bash
# Using the main script runner
go run ./scripts/main.go -cmd=subscription-migration \
  -tenant-id="your-tenant-id" \
  -environment-id="your-environment-id"

# Or set environment variables
export TENANT_ID="your-tenant-id"
export ENVIRONMENT_ID="your-environment-id"
export T1_TIME="2024-01-01T00:00:00Z"
export MASTER_PLAN_ID="master-plan-id"
export DRY_RUN_MODE="true"

go run ./scripts/main.go -cmd=subscription-migration
```

## Migration Process Details

### Step 1: Master Plan Creation

- Creates a new plan named "Master"
- Sets lookup key as "master"
- Adds metadata indicating it was created by the migration script

### Step 2: Price Grouping

- Queries all active prices for the specified entity IDs
- **Creates new prices** based on existing prices and associates them with the master plan
- Sets entity_type to PLAN and entity_id to master plan ID for new prices
- Links new prices to original prices via ParentPriceID and metadata
- Adds group_id to new price metadata based on the original entity group mappings
- **Preserves original prices unchanged**

### Step 3: Subscription Cancellation

- Fetches all active subscriptions for the tenant/environment
- Modifies each subscription to:
  - Set `current_period_end` to T1 time
  - Set `end_date` to T1 time
  - Set `cancel_at_period_end` to true
  - Set `cancel_at` to T1 time

### Step 4: New Subscription Creation

- Creates new subscriptions for each cancelled subscription
- Uses the master plan ID
- Sets start date to T1 time
- Uses calendar billing cycle
- Preserves original billing period and cadence
- Adds migration metadata

## Key Benefits

### Price Safety
- **Original prices are preserved**: The migration creates new prices instead of modifying existing ones
- **Full traceability**: New prices link back to original prices via ParentPriceID and metadata
- **Rollback capability**: Original prices remain intact for potential rollback scenarios
- **No data loss**: All original pricing information is maintained

### Migration Safety
- **Non-destructive approach**: Only creates new entities, doesn't modify existing pricing
- **Clear audit trail**: Metadata tracks the migration process and original relationships
- **Incremental rollout**: Can test with specific entity groups before full migration

## Safety Features

### Dry Run Mode

All scripts support dry run mode (enabled by default). In dry run mode:

- No actual database changes are made
- All operations are logged with "DRY RUN:" prefix
- You can verify the migration plan before execution

### Logging

All scripts provide comprehensive logging:

- Info level: Major operations and progress
- Debug level: Detailed operation information
- Error level: Failures and issues

### Error Handling

- Scripts fail fast on errors
- Detailed error messages with context
- Database transactions ensure consistency

## Prerequisites

1. **Database Access**: Scripts need access to the PostgreSQL database
2. **Configuration**: Proper configuration file setup
3. **Permissions**: Appropriate permissions for the tenant/environment
4. **Entity Mappings**: Valid entity group mappings in JSON format

## Troubleshooting

### Common Issues

1. **Invalid T1_TIME format**: Ensure time is in ISO 8601 format (e.g., "2024-01-01T00:00:00Z")
2. **Invalid ENTITY_GROUPS JSON**: Validate JSON syntax and ensure all entity IDs exist
3. **Missing permissions**: Verify tenant/environment access
4. **Database connectivity**: Check database configuration and network access

### Validation Steps

Before running the migration:

1. **Verify entity IDs exist**:
   ```bash
   # Check if entity IDs in your mapping actually exist in the database
   ```

2. **Test with dry run**:
   ```bash
   export DRY_RUN_MODE="true"
   ./scripts/complete_migration.sh
   ```

3. **Check logs**: Review all log output for warnings or issues

### Recovery

If migration fails partway through:

1. **Check logs** to identify the failure point
2. **Verify database state** - some operations may have completed
3. **Fix the underlying issue**
4. **Re-run with appropriate environment variables**

The scripts are designed to be idempotent where possible, but manual cleanup may be required in some failure scenarios.

## Support

For issues or questions:

1. Check the logs for detailed error messages
2. Verify all environment variables are set correctly
3. Ensure database connectivity and permissions
4. Test with dry run mode first
