#!/bin/bash

# Test All Steps: Complete Migration
# This script runs all migration steps in sequence

set -e

echo "=== Testing All Steps: Complete Migration ==="

# Required environment variables
export TENANT_ID="your-tenant-id"
export ENVIRONMENT_ID="your-environment-id"
export DRY_RUN_MODE="true"  # Set to "false" for actual execution

# T1 time for cancellation and new subscription start
export T1_TIME="2024-12-01T00:00:00Z"

# Entity group mappings
export ENTITY_GROUPS='{"entity_id_1":"group_premium","entity_id_2":"group_standard","entity_id_3":"group_basic"}'

# Step control - run all steps (this is the default behavior)
export STEP_1_CREATE_PLAN="true"
export STEP_2_CREATE_PRICES="true"
export STEP_3_CANCEL_SUBSCRIPTIONS="true"
export STEP_4_CREATE_SUBSCRIPTIONS="true"

echo "Configuration:"
echo "  Tenant ID: $TENANT_ID"
echo "  Environment ID: $ENVIRONMENT_ID"
echo "  T1 Time: $T1_TIME"
echo "  Dry Run Mode: $DRY_RUN_MODE"
echo "  Entity Groups: $ENTITY_GROUPS"
echo "  All Steps: Enabled"
echo ""

# Change to the project root directory
cd "$(dirname "$0")/.."

# Run the migration
echo "Running complete migration (all steps)..."
go run ./scripts/main.go -cmd=complete-migration

echo ""
echo "=== Complete Migration Test Complete ==="
echo "All steps have been executed. Check the logs above for details."



