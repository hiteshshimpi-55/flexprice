#!/bin/bash

# Test Step 2: Create Prices for Master Plan Only
# This script tests only the price creation step

set -e

echo "=== Testing Step 2: Create Prices for Master Plan ==="

# Required environment variables
export TENANT_ID="your-tenant-id"
export ENVIRONMENT_ID="your-environment-id"
export DRY_RUN_MODE="true"  # Set to "false" for actual execution

# Master plan ID from step 1 (replace with actual ID)
export MASTER_PLAN_ID="your-master-plan-id-from-step-1"

# Entity group mappings
export ENTITY_GROUPS='{"entity_id_1":"group_premium","entity_id_2":"group_standard","entity_id_3":"group_basic"}'

# Step control - only run step 2
export STEP_1_CREATE_PLAN="false"
export STEP_2_CREATE_PRICES="true"
export STEP_3_CANCEL_SUBSCRIPTIONS="false"
export STEP_4_CREATE_SUBSCRIPTIONS="false"

echo "Configuration:"
echo "  Tenant ID: $TENANT_ID"
echo "  Environment ID: $ENVIRONMENT_ID"
echo "  Master Plan ID: $MASTER_PLAN_ID"
echo "  Dry Run Mode: $DRY_RUN_MODE"
echo "  Step 2 (Create Prices): $STEP_2_CREATE_PRICES"
echo "  Entity Groups: $ENTITY_GROUPS"
echo ""

# Change to the project root directory
cd "$(dirname "$0")/.."

# Run the migration
echo "Running step 2 migration..."
go run ./scripts/main.go -cmd=complete-migration

echo ""
echo "=== Step 2 Test Complete ==="
echo "Check the logs above for the number of prices created."



