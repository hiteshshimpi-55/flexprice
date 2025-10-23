#!/bin/bash

# Test Step 3: Cancel Subscriptions Only
# This script tests only the subscription cancellation step

set -e

echo "=== Testing Step 3: Cancel Subscriptions at T1 ==="

# Required environment variables
export TENANT_ID="your-tenant-id"
export ENVIRONMENT_ID="your-environment-id"
export DRY_RUN_MODE="true"  # Set to "false" for actual execution

# T1 time for cancellation
export T1_TIME="2024-12-01T00:00:00Z"

# Step control - only run step 3
export STEP_1_CREATE_PLAN="false"
export STEP_2_CREATE_PRICES="false"
export STEP_3_CANCEL_SUBSCRIPTIONS="true"
export STEP_4_CREATE_SUBSCRIPTIONS="false"

echo "Configuration:"
echo "  Tenant ID: $TENANT_ID"
echo "  Environment ID: $ENVIRONMENT_ID"
echo "  T1 Time: $T1_TIME"
echo "  Dry Run Mode: $DRY_RUN_MODE"
echo "  Step 3 (Cancel Subscriptions): $STEP_3_CANCEL_SUBSCRIPTIONS"
echo ""

# Change to the project root directory
cd "$(dirname "$0")/.."

# Run the migration
echo "Running step 3 migration..."
go run ./scripts/main.go -cmd=complete-migration

echo ""
echo "=== Step 3 Test Complete ==="
echo "Check the logs above for the number of subscriptions canceled."



