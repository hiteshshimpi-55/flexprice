### PROBLEM
Currently we only support commitment only for subscription period. But we want to support commitment for different period. 
eg. Monthly subscription period and Annual commitment duration for commitment amount of 1000$.
THis will only be applied on subscription level commitment. not line item level commitment.

### SOLUTION

So suppose we have subscription started on 1st of jan
configured subscription level commitment of 1000$ with duration annual and overage factor to be 2

now 
1st month (Jan - Feb) amount was 200$
2nd month (Feb - Mar) amount was 600$
Current month usage is going like 300$ which is 100$ overage

So the overage will be 100$*2 = 200$ so the total for this month is 200$ + 200$ = 400$

Now the question is what will be the committed amount for next month?
- How to look back for overage ? 
  - use previous months invoices with subtotal
- Should we conside the overage charges for next month lookup / base charge. As the previous month was already overage so it wont matter right ?
    This is on you to answer


### Implementation

You will need to change in 2 method in @billing.go service.
1. CalculateUsageCharges : used for invoice calculation
2. CalculateFeatureUsageCharges: used for preview.
