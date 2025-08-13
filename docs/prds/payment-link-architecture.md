# Payment Link Feature - Simple Architecture & Flow

## What is the Payment Link Feature?

The Payment Link feature allows customers to pay invoices through secure payment links. When a customer clicks the link, they're taken to Stripe's secure payment page to complete their payment.

## How It Works - Step by Step

### 1. Creating a Payment Link

**What happens:**
1. Client requests a payment link for an invoice
2. System creates a payment record with status `INITIATED`
3. System calls Stripe to create a checkout session
4. If Stripe succeeds: status becomes `PENDING` and returns the payment URL
5. If Stripe fails: status stays `INITIATED` and returns an error

**Status Flow:**
```
INITIATED → PENDING (if Stripe succeeds)
INITIATED → INITIATED (if Stripe fails)
```

### 2. Customer Pays

**What happens:**
1. Customer clicks the payment link
2. Customer enters payment details on Stripe's secure page
3. Stripe processes the payment
4. Stripe sends webhooks to our system with the result

### 3. Webhook Processing

**What happens when Stripe sends a webhook:**
1. System verifies the webhook is from Stripe (security check)
2. System finds the payment record using the session ID
3. System checks if payment is already successful (protection)
4. If payment is already successful: skip update
5. If payment is not successful: call Stripe API to get latest status
6. Update payment status in database
7. If payment succeeded: reconcile with invoice

**Protection Check:**
- If payment status is already `SUCCEEDED`, no webhook can change it
- This prevents data corruption from duplicate webhooks

## Webhook Types We Handle

| Webhook Type | When It Happens | What We Do |
|--------------|-----------------|------------|
| `checkout.session.completed` | Customer completes payment | Mark payment as SUCCEEDED |
| `checkout.session.async_payment_succeeded` | Async payment succeeds | Mark payment as SUCCEEDED |
| `checkout.session.async_payment_failed` | Async payment fails | Mark payment as FAILED |
| `checkout.session.expired` | Payment link expires | Mark payment as FAILED |
| `payment_intent.payment_failed` | Payment fails immediately | Mark payment as FAILED |
| `payment_intent.succeeded` | Payment succeeds | Mark payment as SUCCEEDED |

## Payment Status Flow

```
INITIATED → PENDING → PROCESSING → SUCCEEDED (Final)
INITIATED → PENDING → PROCESSING → FAILED (Final)
```

**Important:** Once a payment reaches `SUCCEEDED`, it cannot be changed by any webhook.

## Database Fields We Use

| Field | What It Stores | Example |
|-------|----------------|---------|
| `payment_status` | Current status | `INITIATED`, `PENDING`, `SUCCEEDED`, `FAILED` |
| `gateway_tracking_id` | Stripe session ID | `cs_test_1234567890` |
| `gateway_payment_id` | Stripe payment intent ID | `pi_1234567890` |
| `payment_method_id` | Stripe payment method ID | `pm_1234567890` |
| `succeeded_at` | When payment succeeded | `2024-01-15T10:30:00Z` |
| `failed_at` | When payment failed | `2024-01-15T10:30:00Z` |
| `error_message` | Why payment failed | `"Card declined"` |

## Security Features

### 1. Webhook Verification
- Every webhook is verified using Stripe's signature
- Only webhooks from Stripe are processed

### 2. Payment Protection
- Successful payments cannot be modified by webhooks
- Prevents data corruption from duplicate webhooks

### 3. Environment Isolation
- Each environment (dev, staging, prod) has separate configurations
- Each tenant has separate webhook endpoints

## Error Handling

### Stripe SDK Fails
- Keep payment status as `INITIATED`
- Return error to client
- Allow retry

### Webhook for Unknown Payment
- Log warning
- Return success (don't fail webhook)
- Allow manual investigation

### Duplicate Webhooks
- Protection check prevents updates to successful payments
- Multiple webhooks won't corrupt data

## API Endpoints

### Create Payment Link
```
POST /api/v1/payments
{
    "payment_method_type": "PAYMENT_LINK",
    "destination_id": "invoice-uuid",
    "amount": "1000",
    "currency": "USD"
}
```

### Process Payment
```
POST /api/v1/payments/{payment-id}/process
```

### Webhook Endpoint
```
POST /api/v1/webhooks/stripe/{tenant-id}/{environment-id}
```

## Testing

### Happy Path
1. Create payment link → Status: `INITIATED`
2. Process payment → Status: `PENDING`
3. Customer pays → Webhook received → Status: `SUCCEEDED`

### Error Path
1. Create payment link → Status: `INITIATED`
2. Stripe fails → Status: `INITIATED` (stays same)
3. Retry → Status: `PENDING`

### Card Decline
1. Customer uses declined card → Webhook: `payment_intent.payment_failed`
2. Status: `FAILED`

## Common Issues & Solutions≠

### Payment Link Not Generated
**Problem:** Status stuck at `INITIATED`
**Solution:** Check Stripe API keys and configuration

### Webhook Not Received
**Problem:** Payment status not updating after customer payment
**Solution:** Check webhook endpoint configuration in Stripe dashboard

### Payment Status Wrong
**Problem:** Database status doesn't match Stripe
**Solution:** Check webhook processing logs and database connectivity

## Key Benefits

1. **Simple**: Easy to understand and implement
2. **Secure**: Proper webhook verification and data protection
3. **Reliable**: Handles errors gracefully
4. **Auditable**: Full logging for debugging
5. **Scalable**: Can handle high volume

## Summary

The Payment Link feature is a simple, secure way to process payments through Stripe. It creates payment links, handles webhooks from Stripe, and maintains data integrity through protection mechanisms. The system is production-ready and handles real-world payment scenarios reliably. 