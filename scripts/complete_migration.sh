#!/bin/bash

# Complete Migration Script
# This script performs a complete migration:
# 1. Create a master plan
# 2. Add prices to it with group associated  
# 3. Cancel the subscription by modifying current_period_end and end_date with t1 and cancel_at_period_end true
# 4. Create new subscription with master plan with start day as t1 and billing cycle calendar

set -e

# Check if required environment variables are set
if [[ -z "$TENANT_ID" ]]; then
    echo "Error: TENANT_ID environment variable is required"
    exit 1
fi

if [[ -z "$ENVIRONMENT_ID" ]]; then
    echo "Error: ENVIRONMENT_ID environment variable is required"
    exit 1
fi

if [[ -z "$ENTITY_GROUPS" ]]; then
    echo "Error: ENTITY_GROUPS environment variable is required"
    echo "Example: ENTITY_GROUPS='{\"entity_id_1\":\"group_1\",\"entity_id_2\":\"group_2\"}'"
    exit 1
fi

if [[ -z "$T1_TIME" ]]; then
    echo "Error: T1_TIME environment variable is required"
    echo "Example: T1_TIME='2024-01-01T00:00:00Z'"
    exit 1
fi

# Set default for DRY_RUN_MODE if not provided
DRY_RUN_MODE=${DRY_RUN_MODE:-"true"}

echo "Starting Complete Migration..."
echo "Tenant ID: $TENANT_ID"
echo "Environment ID: $ENVIRONMENT_ID"
echo "T1 Time: $T1_TIME"
echo "Dry Run Mode: $DRY_RUN_MODE"
echo "Entity Groups: $ENTITY_GROUPS"

# Build and run the migration
cd "$(dirname "$0")/.."

# Build the migration binary
echo "Building migration binary..."
go build -o /tmp/complete_migration ./scripts/main.go

# Run the migration
echo "Running complete migration..."
/tmp/complete_migration -cmd=complete-migration

echo "Complete migration finished!"

# Clean up
rm -f /tmp/complete_migration
