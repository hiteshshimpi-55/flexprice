#!/bin/bash

# Example script to run the plan grouping functionality
# This script demonstrates how to use the plan_grouping.go script

# Set required environment variables
export TENANT_ID="your_tenant_id_here"
export ENVIRONMENT_ID="your_environment_id_here"
export DRY_RUN_MODE="true"  # Set to "false" for actual execution

# Define entity group mappings as JSON
# This matches the format: {"entity_id": "group_id", ...}
export ENTITY_GROUPS='{
  "Serverless": "group_private",
  "Dedicated": "group_public", 
  "Shared": "group_hybrid"
}'

echo "Starting plan grouping process..."
echo "Tenant ID: $TENANT_ID"
echo "Environment ID: $ENVIRONMENT_ID"
echo "Dry Run Mode: $DRY_RUN_MODE"
echo "Entity Groups: $ENTITY_GROUPS"
echo ""

# Run the plan grouping script
# Note: You'll need to compile and run the Go script
# go run scripts/internal/plan_grouping.go

echo "To run this script:"
echo "1. Update the TENANT_ID and ENVIRONMENT_ID above"
echo "2. Modify the ENTITY_GROUPS JSON array as needed"
echo "3. Set DRY_RUN_MODE to 'false' when ready for actual execution"
echo "4. Compile and run: go run scripts/internal/plan_grouping.go"
echo ""
echo "The script will:"
echo "1. Create a Master plan"
echo "2. Query prices based on the provided entity_ids"
echo "3. Associate prices with the Master plan"
echo "4. Associate group_id with prices based on entity_id mapping"
echo ""
echo "Example usage:"
echo "ENTITY_GROUPS='{\"entity1\": \"group1\", \"entity2\": \"group2\"}'"

