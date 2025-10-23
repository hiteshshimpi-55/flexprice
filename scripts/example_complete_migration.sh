#!/bin/bash

# Example Complete Migration Script
# This script demonstrates how to run the complete migration with example data

set -e

echo "=== FlexPrice Complete Migration Example ==="
echo ""

# Example configuration - REPLACE WITH YOUR ACTUAL VALUES
export TENANT_ID="example_tenant_123"
export ENVIRONMENT_ID="example_env_456"
export T1_TIME="2024-12-01T00:00:00Z"

# Example entity group mappings - REPLACE WITH YOUR ACTUAL MAPPINGS
# This maps entity IDs to group IDs for price grouping
export ENTITY_GROUPS='{
  "price_entity_001": "group_premium",
  "price_entity_002": "group_standard", 
  "price_entity_003": "group_premium",
  "price_entity_004": "group_basic",
  "price_entity_005": "group_standard"
}'

# Set to dry run mode for safety - change to "false" when ready to execute
export DRY_RUN_MODE="true"

echo "Configuration:"
echo "  Tenant ID: $TENANT_ID"
echo "  Environment ID: $ENVIRONMENT_ID"
echo "  T1 Time: $T1_TIME"
echo "  Dry Run Mode: $DRY_RUN_MODE"
echo "  Entity Groups: $ENTITY_GROUPS"
echo ""

# Confirmation prompt
if [ "$DRY_RUN_MODE" = "false" ]; then
    echo "⚠️  WARNING: This will make actual changes to the database!"
    echo "   - Create a new master plan"
    echo "   - Create new prices for the master plan with group associations (preserves originals)"
    echo "   - Cancel existing subscriptions at T1 time"
    echo "   - Create new subscriptions with master plan and calendar billing"
    echo ""
    read -p "Are you sure you want to proceed? (yes/no): " confirm
    
    if [ "$confirm" != "yes" ]; then
        echo "Migration cancelled."
        exit 0
    fi
else
    echo "🔍 Running in DRY RUN mode - no actual changes will be made"
    echo ""
fi

# Run the migration
echo "Starting complete migration..."
echo ""

# Change to the project root directory
cd "$(dirname "$0")/.."

# Execute the migration
./scripts/complete_migration.sh

echo ""
echo "=== Migration Complete ==="

if [ "$DRY_RUN_MODE" = "true" ]; then
    echo ""
    echo "📋 Next Steps:"
    echo "1. Review the dry run output above"
    echo "2. Verify all entity IDs and group mappings are correct"
    echo "3. Set DRY_RUN_MODE=\"false\" to execute the actual migration"
    echo "4. Re-run this script to perform the migration"
fi
