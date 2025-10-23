#!/bin/bash

# Example script to run the subscription migration functionality
# This script demonstrates how to use the subscription_migration.go script

# Set required environment variables
export TENANT_ID="your_tenant_id_here"
export ENVIRONMENT_ID="your_environment_id_here"
export DRY_RUN_MODE="true"  # Set to "false" for actual execution
export T1_TIME="2025-02-01T00:00:00Z"  # Migration time in RFC3339 format
export MASTER_PLAN_ID="your_master_plan_id_here"

echo "Starting subscription migration process..."
echo "Tenant ID: $TENANT_ID"
echo "Environment ID: $ENVIRONMENT_ID"
echo "Dry Run Mode: $DRY_RUN_MODE"
echo "T1 Time (Migration Time): $T1_TIME"
echo "Master Plan ID: $MASTER_PLAN_ID"
echo ""

# Run the subscription migration script
# Note: You'll need to compile and run the Go script
# go run scripts/internal/subscription_migration.go

echo "To run this script:"
echo "1. Update the TENANT_ID and ENVIRONMENT_ID above"
echo "2. Set the T1_TIME to your desired migration time (RFC3339 format)"
echo "3. Set the MASTER_PLAN_ID to your Master plan ID"
echo "4. Set DRY_RUN_MODE to 'false' when ready for actual execution"
echo "5. Compile and run: go run scripts/internal/subscription_migration.go"
echo ""
echo "The script will:"
echo "1. Fetch all active subscriptions for the tenant/environment"
echo "2. Cancel existing subscriptions at T1 time (set cancel_at_period_end=true, cancel_at=T1)"
echo "3. Create new subscriptions with Master plan starting at T1 with calendar billing"
echo ""
echo "Example T1_TIME formats:"
echo "  T1_TIME='2025-02-01T00:00:00Z'        # UTC time"
echo "  T1_TIME='2025-02-01T09:00:00+09:00'   # JST time"
echo "  T1_TIME='2025-02-01T00:00:00-05:00'   # EST time"

