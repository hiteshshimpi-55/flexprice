#!/bin/bash

# Test Step 4: Create New Subscriptions Only
# This script tests only the new subscription creation step

set -e

echo "=== Testing Step 4: Create New Subscriptions with Master Plan ==="

# Required environment variables
export TENANT_ID="your-tenant-id"
export ENVIRONMENT_ID="your-environment-id"
export DRY_RUN_MODE="true"  # Set to "false" for actual execution

# Master plan ID from step 1 (replace with actual ID)
export MASTER_PLAN_ID="your-master-plan-id-from-step-1"

# T1 time for new subscription start
export T1_TIME="2024-12-01T00:00:00Z"

# Step control - only run step 4
export STEP_1_CREATE_PLAN="false"
export STEP_2_CREATE_PRICES="false"
export STEP_3_CANCEL_SUBSCRIPTIONS="false"
export STEP_4_CREATE_SUBSCRIPTIONS="true"

echo "Configuration:"
echo "  Tenant ID: $TENANT_ID"
echo "  Environment ID: $ENVIRONMENT_ID"
echo "  Master Plan ID: $MASTER_PLAN_ID"
echo "  T1 Time: $T1_TIME"
echo "  Dry Run Mode: $DRY_RUN_MODE"
echo "  Step 4 (Create Subscriptions): $STEP_4_CREATE_SUBSCRIPTIONS"
echo ""

# Change to the project root directory
cd "$(dirname "$0")/.."

# Run the migration
echo "Running step 4 migration..."
go run ./scripts/main.go -cmd=complete-migration

echo ""
echo "=== Step 4 Test Complete ==="
echo "Check the logs above for the number of new subscriptions created."



