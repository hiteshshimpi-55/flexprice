#!/bin/bash

# Test Step 1: Create Master Plan Only
# This script tests only the master plan creation step

set -e

echo "=== Testing Step 1: Create Master Plan ==="

# Required environment variables
export TENANT_ID="your-tenant-id"
export ENVIRONMENT_ID="your-environment-id"
export DRY_RUN_MODE="true"  # Set to "false" for actual execution

# Step control - only run step 1
export STEP_1_CREATE_PLAN="true"
export STEP_2_CREATE_PRICES="false"
export STEP_3_CANCEL_SUBSCRIPTIONS="false"
export STEP_4_CREATE_SUBSCRIPTIONS="false"

echo "Configuration:"
echo "  Tenant ID: $TENANT_ID"
echo "  Environment ID: $ENVIRONMENT_ID"
echo "  Dry Run Mode: $DRY_RUN_MODE"
echo "  Step 1 (Create Plan): $STEP_1_CREATE_PLAN"
echo ""

# Change to the project root directory
cd "$(dirname "$0")/.."

# Run the migration
echo "Running step 1 migration..."
go run ./scripts/main.go -cmd=complete-migration

echo ""
echo "=== Step 1 Test Complete ==="
echo "Check the logs above for the master plan ID if successful."



