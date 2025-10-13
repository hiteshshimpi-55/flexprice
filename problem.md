### Current Flow (Top-Down) - ✅ RECOMMENDED

Event → Customer → Subscriptions → LineItems → Prices → Meters → Features
  │         │            │             │          │        │         │
  │         │            │             │          │        │         └─> FeatureID
  │         │            │             │          │        └─> MeterID (aggregation type)
  │         │            │             │          └─> PriceID
  │         │            │             └─> SubLineItemID
  │         │            └─> SubscriptionID, PeriodID
  │         └─> CustomerID
  └─> EventID, Timestamp, Properties, Quantity

**Database Calls:** 5 optimized bulk queries
- Customer lookup (by external_customer_id)
- Subscriptions with line items (filtered by customer)
- Prices (bulk fetch by price IDs)
- Meters (bulk fetch by meter IDs)
- Features (bulk fetch by meter IDs)

**Addons:** ✅ Automatically handled through subscription line items


### Bottoms-up approach - ❌ NOT RECOMMENDED

Event -> Meters -> Features -> Prices -> Subline items -> Subscription_id -> Customer
            |                              |
            |                              \-> Plan_id
            \-> Filters

**Database Calls:** 6+ queries with complex filtering
- Meters (by event_name - returns ALL meters across system)
- Features (by meter IDs)
- Prices (by meter IDs - returns ALL prices across system)
- Line items (by price IDs - need customer filter)
- Subscriptions (need to verify customer & status)
- Customer (finally validate)

**Problems:**
❌ Loses customer context early - fetches data for ALL customers
❌ No reduction in DB calls (actually more)
❌ Complex addon handling (need extra joins/queries)
❌ Poor scalability with high-volume tenants
❌ More memory overhead (fetch more data, filter later)

---

## Analysis Result

**See detailed analysis:** `bottoms-up-analysis.md`

**Verdict:** Keep the current top-down approach. The bottoms-up approach:
- Does NOT reduce database calls
- Introduces architectural problems
- Makes addon handling complex
- Scales worse with tenant size
- Provides no measurable benefits