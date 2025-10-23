# Step-by-Step Migration Testing Guide

This guide explains how to test the migration process step by step using individual flags for easy testing and validation.

## Overview

The complete migration has been divided into 4 testable steps:

1. **Step 1**: Create Master Plan (`STEP_1_CREATE_PLAN=true`)
2. **Step 2**: Create Prices for Master Plan (`STEP_2_CREATE_PRICES=true`)
3. **Step 3**: Cancel Existing Subscriptions (`STEP_3_CANCEL_SUBSCRIPTIONS=true`)
4. **Step 4**: Create New Subscriptions (`STEP_4_CREATE_SUBSCRIPTIONS=true`)

## Environment Variables

### Required for All Steps
- `TENANT_ID` - Your tenant identifier
- `ENVIRONMENT_ID` - Your environment identifier
- `DRY_RUN_MODE` - Set to "true" for testing, "false" for execution

### Step-Specific Variables
- `ENTITY_GROUPS` - Required for Step 2 (JSON mapping of entity IDs to group IDs)
- `T1_TIME` - Required for Steps 3 & 4 (ISO 8601 format: "2024-01-01T00:00:00Z")
- `MASTER_PLAN_ID` - Required for Steps 2 & 4 when not running Step 1

### Step Control Flags
- `STEP_1_CREATE_PLAN` - Set to "true" to run step 1
- `STEP_2_CREATE_PRICES` - Set to "true" to run step 2
- `STEP_3_CANCEL_SUBSCRIPTIONS` - Set to "true" to run step 3
- `STEP_4_CREATE_SUBSCRIPTIONS` - Set to "true" to run step 4

## Testing Approach

### 1. Test Each Step Individually

#### Step 1: Create Master Plan
```bash
./scripts/test_step_1_create_plan.sh
```

**What it does:**
- Creates a new master plan named "Master"
- Returns the master plan ID for use in subsequent steps

**Expected output:**
- Master plan creation log
- Master plan ID in the response

**Update the script with your values:**
```bash
export TENANT_ID="your-actual-tenant-id"
export ENVIRONMENT_ID="your-actual-environment-id"
```

#### Step 2: Create Prices for Master Plan
```bash
./scripts/test_step_2_create_prices.sh
```

**What it does:**
- Queries existing prices based on entity IDs
- Creates new prices for the master plan
- Associates group IDs with the new prices

**Prerequisites:**
- Master plan ID from Step 1
- Entity group mappings

**Update the script with your values:**
```bash
export MASTER_PLAN_ID="master-plan-id-from-step-1"
export ENTITY_GROUPS='{"your_entity_1":"group_a","your_entity_2":"group_b"}'
```

#### Step 3: Cancel Existing Subscriptions
```bash
./scripts/test_step_3_cancel_subscriptions.sh
```

**What it does:**
- Finds all active subscriptions
- Modifies current_period_end and end_date to T1 time
- Sets cancel_at_period_end to true

**Prerequisites:**
- T1 time for cancellation

**Update the script with your values:**
```bash
export T1_TIME="2024-12-01T00:00:00Z"
```

#### Step 4: Create New Subscriptions
```bash
./scripts/test_step_4_create_subscriptions.sh
```

**What it does:**
- Creates new subscriptions for each existing customer
- Uses the master plan
- Sets start date to T1 time
- Uses calendar billing cycle

**Prerequisites:**
- Master plan ID from Step 1
- T1 time for new subscription start

**Update the script with your values:**
```bash
export MASTER_PLAN_ID="master-plan-id-from-step-1"
export T1_TIME="2024-12-01T00:00:00Z"
```

### 2. Test All Steps Together
```bash
./scripts/test_all_steps.sh
```

**What it does:**
- Runs all 4 steps in sequence
- Equivalent to the original complete migration

## Testing Workflow

### Recommended Testing Sequence

1. **Start with Dry Run Mode**
   ```bash
   export DRY_RUN_MODE="true"
   ```

2. **Test Step 1 First**
   ```bash
   ./scripts/test_step_1_create_plan.sh
   ```
   - Verify master plan creation logs
   - Note the master plan ID from logs

3. **Test Step 2 with Real Entity IDs**
   ```bash
   # Update the script with actual entity IDs and master plan ID
   ./scripts/test_step_2_create_prices.sh
   ```
   - Verify price creation logs
   - Check that group associations are correct

4. **Test Step 3 Independently**
   ```bash
   ./scripts/test_step_3_cancel_subscriptions.sh
   ```
   - Verify subscription cancellation logs
   - Check that T1 time is applied correctly

5. **Test Step 4 Independently**
   ```bash
   ./scripts/test_step_4_create_subscriptions.sh
   ```
   - Verify new subscription creation logs
   - Check calendar billing cycle is applied

6. **Test All Steps Together**
   ```bash
   ./scripts/test_all_steps.sh
   ```
   - Verify complete flow works end-to-end

7. **Execute with Real Data**
   ```bash
   export DRY_RUN_MODE="false"
   # Run your chosen step(s)
   ```

## Validation Points

### After Step 1
- [ ] Master plan created successfully
- [ ] Master plan ID returned
- [ ] Plan metadata includes migration tracking

### After Step 2
- [ ] New prices created for master plan
- [ ] Original prices preserved unchanged
- [ ] Group IDs correctly associated via metadata
- [ ] Parent-child relationships established

### After Step 3
- [ ] Existing subscriptions have updated period end
- [ ] End date set to T1 time
- [ ] Cancel at period end flag set to true
- [ ] Original subscription data preserved

### After Step 4
- [ ] New subscriptions created for each customer
- [ ] Master plan associated with new subscriptions
- [ ] Calendar billing cycle applied
- [ ] Start date set to T1 time
- [ ] Migration metadata included

## Troubleshooting

### Common Issues

1. **Missing Master Plan ID**
   - Run Step 1 first or provide `MASTER_PLAN_ID`

2. **Invalid Entity Groups JSON**
   - Validate JSON syntax: `echo $ENTITY_GROUPS | jq .`

3. **Invalid T1 Time Format**
   - Use ISO 8601 format: "2024-01-01T00:00:00Z"

4. **No Active Subscriptions Found**
   - Verify tenant/environment has active subscriptions

### Debugging Tips

1. **Check Logs Carefully**
   - Each step logs its progress with clear markers
   - Look for "Step X completed" messages

2. **Verify Prerequisites**
   - Each step validates required environment variables
   - Error messages indicate missing requirements

3. **Use Dry Run Mode**
   - Always test with `DRY_RUN_MODE="true"` first
   - Verify expected behavior before real execution

4. **Test Steps in Order**
   - Some steps depend on outputs from previous steps
   - Follow the recommended testing sequence

## Advanced Usage

### Running Multiple Steps
```bash
export STEP_1_CREATE_PLAN="true"
export STEP_2_CREATE_PRICES="true"
export STEP_3_CANCEL_SUBSCRIPTIONS="false"
export STEP_4_CREATE_SUBSCRIPTIONS="false"

go run ./scripts/main.go -cmd=complete-migration
```

### Using Existing Master Plan
```bash
export STEP_1_CREATE_PLAN="false"
export STEP_2_CREATE_PRICES="true"
export MASTER_PLAN_ID="existing-master-plan-id"

go run ./scripts/main.go -cmd=complete-migration
```

### Production Execution
```bash
export DRY_RUN_MODE="false"
export STEP_1_CREATE_PLAN="true"  # Only run the step you want

go run ./scripts/main.go -cmd=complete-migration
```

This step-by-step approach allows you to:
- Test each component individually
- Validate outputs before proceeding
- Debug issues in isolation
- Execute migration in phases
- Rollback if needed (original data preserved)



