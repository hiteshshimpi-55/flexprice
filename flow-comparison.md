# Event Processing Flow - Visual Comparison

## Top-Down Approach (Current - Recommended ✅)

```
┌─────────────────────────────────────────────────────────────────┐
│ Event Arrives                                                   │
│ { customer: "cust_123", event: "api_call", properties: {...} } │
└─────────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────────┐
│ Query 1: Get Customer                                           │
│ SELECT * FROM customers WHERE external_id = 'cust_123'         │
│ Result: 1 customer                                              │
└─────────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────────┐
│ Query 2: Get Active Subscriptions (with line items)            │
│ SELECT * FROM subscriptions                                     │
│ JOIN subscription_line_items ON subscription_id                │
│ WHERE customer_id = 'cust_123_internal_id'                     │
│   AND status IN ('active', 'trialing')                         │
│                                                                  │
│ Result: 2 subscriptions, 5 line items (plan + addons)          │
│   ✅ Line items include BOTH plan prices AND addon prices       │
└─────────────────────────────────────────────────────────────────┘
                            ↓
              ┌─────────────┴─────────────┐
              │ In-memory processing      │
              │ Extract unique price IDs: │
              │ [price_1, price_2, ...]   │
              └─────────────┬─────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────────┐
│ Query 3: Bulk Fetch Prices                                      │
│ SELECT * FROM prices                                            │
│ WHERE id IN ('price_1', 'price_2', ...)                        │
│   AND status = 'published'                                      │
│                                                                  │
│ Result: 5 prices (only those used by THIS customer)            │
└─────────────────────────────────────────────────────────────────┘
                            ↓
              ┌─────────────┴─────────────┐
              │ In-memory processing      │
              │ Extract unique meter IDs: │
              │ [meter_1, meter_2, ...]   │
              └─────────────┬─────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────────┐
│ Query 4: Bulk Fetch Meters                                      │
│ SELECT * FROM meters                                            │
│ WHERE id IN ('meter_1', 'meter_2', ...)                        │
│                                                                  │
│ Result: 3 meters (only those needed)                            │
└─────────────────────────────────────────────────────────────────┘
                            ↓
              ┌─────────────┴─────────────┐
              │ Match event to meters     │
              │ Check: event_name         │
              │ Check: filters            │
              └─────────────┬─────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────────┐
│ Query 5: Bulk Fetch Features                                    │
│ SELECT * FROM features                                          │
│ WHERE meter_id IN ('meter_1', 'meter_2')                       │
│                                                                  │
│ Result: 3 features (only those matched)                         │
└─────────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────────┐
│ Create Feature Usage Records                                    │
│ - 2 records created (one per matched meter per subscription)   │
│ - Includes subscription_id, line_item_id, price_id, etc.       │
└─────────────────────────────────────────────────────────────────┘

📊 STATISTICS:
  - Total Queries: 5
  - Rows Fetched: ~19 (1 customer + 2 subs + 5 line items + 5 prices + 3 meters + 3 features)
  - Scope: Single customer only (narrow scope from start)
  - Complexity: Low (straightforward queries)
  - Addons: Handled automatically (no extra code)
```

---

## Bottoms-Up Approach (Proposed - NOT Recommended ❌)

```
┌─────────────────────────────────────────────────────────────────┐
│ Event Arrives                                                   │
│ { customer: "cust_123", event: "api_call", properties: {...} } │
└─────────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────────┐
│ Query 1: Find Meters by Event Name                             │
│ SELECT * FROM meters                                            │
│ WHERE event_name = 'api_call'                                   │
│                                                                  │
│ ⚠️  Result: 50 meters (from ALL customers/tenants using this    │
│     event name - OVERBROAD!)                                    │
└─────────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────────┐
│ Query 2: Get Features for Meters                               │
│ SELECT * FROM features                                          │
│ WHERE meter_id IN (all 50 meter IDs)                           │
│                                                                  │
│ Result: 50 features                                             │
└─────────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────────┐
│ Query 3: Get Prices by Meter IDs                               │
│ SELECT * FROM prices                                            │
│ WHERE meter_id IN (all 50 meter IDs)                           │
│                                                                  │
│ ⚠️  Result: 500 prices (from ALL customers/tenants - HUGE!)     │
└─────────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────────┐
│ Query 4: Get Line Items by Price IDs                           │
│ SELECT sli.*, s.customer_id, s.status                          │
│ FROM subscription_line_items sli                               │
│ JOIN subscriptions s ON sli.subscription_id = s.id             │
│ WHERE sli.price_id IN (all 500 price IDs)  ← Need to filter!  │
│                                                                  │
│ ⚠️  Result: 5000 line items (from ALL subscriptions)            │
│     ⚠️  Must now filter in application or add WHERE clause       │
└─────────────────────────────────────────────────────────────────┘
                            ↓
              ┌─────────────┴─────────────┐
              │ ⚠️  PROBLEM: Need customer │
              │ context but don't have it │
              │ Must either:              │
              │ 1. Fetch all & filter, OR │
              │ 2. Add complex WHERE      │
              └─────────────┬─────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────────┐
│ Query 4b: Filter by Customer (additional complexity)           │
│ ... AND s.customer_id = (                                       │
│     SELECT id FROM customers WHERE external_id = 'cust_123'    │
│ )                                                                │
│ AND s.status IN ('active', 'trialing')                         │
│                                                                  │
│ Result: 5 line items (after filtering 4,995 irrelevant ones)   │
└─────────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────────┐
│ Query 5: Get Subscriptions (if not already joined)             │
│ SELECT * FROM subscriptions                                     │
│ WHERE id IN (subscription IDs from line items)                 │
│                                                                  │
│ Result: 2 subscriptions                                         │
└─────────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────────┐
│ Query 6: Get Customer Details                                   │
│ SELECT * FROM customers WHERE id = 'customer_id'               │
│                                                                  │
│ Result: 1 customer                                              │
└─────────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────────┐
│ ⚠️  Additional Processing Required:                              │
│ - Filter meters by event properties (done late)                │
│ - Validate subscription dates                                   │
│ - Handle addon associations (need extra joins)                 │
│ - Check line item active dates                                 │
│ - Throw away 99% of fetched data                               │
└─────────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────────┐
│ Create Feature Usage Records                                    │
│ - Same 2 records as top-down approach                          │
└─────────────────────────────────────────────────────────────────┘

📊 STATISTICS:
  - Total Queries: 6+
  - Rows Fetched: ~5,606 (50 meters + 50 features + 500 prices + 5000 line items + ...)
  - Scope: Initially SYSTEM-WIDE, filtered down later
  - Complexity: High (complex joins and filtering)
  - Addons: ❌ Need extra logic to handle addon associations
  - Wasted Data: 99%+ of fetched data discarded after filtering
```

---

## Key Differences

| Aspect | Top-Down ✅ | Bottoms-Up ❌ |
|--------|------------|--------------|
| **Starting Point** | Customer (narrow) | All meters (broad) |
| **Query Count** | 5 | 6+ |
| **Rows Fetched** | ~19 (only relevant data) | ~5,606 (99% discarded) |
| **Customer Filtering** | Early (query 1) | Late (query 4-5) |
| **Addon Handling** | Automatic | Complex joins needed |
| **Memory Usage** | Low | High |
| **Database Load** | Low | High |
| **Scalability** | Excellent | Poor |
| **Code Complexity** | Simple | Complex |

---

## Real Example: High-Volume Tenant

### Scenario
- Tenant with 1,000 customers
- 10,000 active subscriptions
- Event name "api_call" used by 80% of subscriptions

### Top-Down Approach
```
Query 1: Get 1 customer            → 1 row
Query 2: Get 2 subscriptions       → 2 rows + 5 line items
Query 3: Get 5 prices              → 5 rows
Query 4: Get 3 meters              → 3 rows  
Query 5: Get 3 features            → 3 rows
─────────────────────────────────────────────
TOTAL: ~19 rows fetched
```

### Bottoms-Up Approach
```
Query 1: Get meters for "api_call" → 200 rows (80% × 250 avg meters)
Query 2: Get features              → 200 rows
Query 3: Get prices                → 2,000 rows (10 prices per meter avg)
Query 4: Get line items            → 80,000 rows (8 line items per sub avg)
         Must filter by customer   → 5 rows (discard 79,995 rows)
Query 5: Get subscriptions         → 2 rows
Query 6: Get customer              → 1 row
─────────────────────────────────────────────
TOTAL: ~82,408 rows fetched, 99.98% discarded
```

---

## Addon Handling Comparison

### Scenario: Subscription with Base Plan + 2 Addons

```
Subscription: sub_123
├── Plan A (base)
│   ├── price_1 → meter_api_calls → feature_api
│   └── price_2 → meter_storage → feature_storage
├── Addon X
│   └── price_3 → meter_api_calls (different tier) → feature_api
└── Addon Y
    └── price_4 → meter_webhooks → feature_webhooks
```

### Top-Down Approach ✅
```go
// Query 2 returns ALL line items automatically
lineItems = [
    {id: "li_1", price_id: "price_1", addon_id: null},      // Plan
    {id: "li_2", price_id: "price_2", addon_id: null},      // Plan
    {id: "li_3", price_id: "price_3", addon_id: "addon_x"}, // Addon X
    {id: "li_4", price_id: "price_4", addon_id: "addon_y"}  // Addon Y
]

// Process all line items the same way
for item in lineItems:
    if item.IsUsage() && item.IsActive():
        process(item)  // ✅ Works for all, no special logic
```

### Bottoms-Up Approach ❌
```sql
-- Query 4 needs complex joins
SELECT sli.*, s.*, aa.*
FROM subscription_line_items sli
JOIN subscriptions s ON sli.subscription_id = s.id
LEFT JOIN addon_associations aa ON sli.addon_id = aa.id  -- Need extra join!
WHERE sli.price_id IN (price_ids_from_meters)
  AND s.customer_id = 'cust_123'
  AND s.status IN ('active', 'trialing')
  AND (
    sli.addon_id IS NULL  -- Plan line items
    OR (
      aa.id IS NOT NULL 
      AND aa.status = 'active'         -- Check addon status
      AND aa.start_date <= NOW()       -- Check addon dates
      AND (aa.end_date IS NULL OR aa.end_date > NOW())
    )
  )
```
❌ Much more complex, requires understanding of addon_associations table
❌ Extra join increases query cost
❌ More places for bugs

---

## Conclusion

### Top-Down (Current) ✅
- **Efficient:** Fetches only relevant data
- **Simple:** Straightforward query flow
- **Scalable:** Performance doesn't degrade with tenant size
- **Maintainable:** Clear separation of concerns
- **Addon-friendly:** Handles addons automatically

### Bottoms-Up (Proposed) ❌
- **Inefficient:** Fetches 100-1000x more data
- **Complex:** Requires complex joins and filtering
- **Unscalable:** Performance degrades linearly with tenant size
- **Error-prone:** More complex logic = more bugs
- **Addon-hostile:** Requires special handling for addons

**🎯 Verdict: Keep the Top-Down approach!**

