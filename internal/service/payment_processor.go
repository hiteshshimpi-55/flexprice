package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/flexprice/flexprice/internal/api/dto"
	"github.com/flexprice/flexprice/internal/domain/payment"
	"github.com/flexprice/flexprice/internal/domain/wallet"
	ierr "github.com/flexprice/flexprice/internal/errors"
	"github.com/flexprice/flexprice/internal/types"
	webhookDto "github.com/flexprice/flexprice/internal/webhook/dto"
	"github.com/samber/lo"
	"github.com/shopspring/decimal"
)

type PaymentProcessorService interface {
	ProcessPayment(ctx context.Context, id string) (*payment.Payment, error)
}

type paymentProcessor struct {
	ServiceParams
}

func NewPaymentProcessorService(params ServiceParams) PaymentProcessorService {
	return &paymentProcessor{
		ServiceParams: params,
	}
}

func (p *paymentProcessor) ProcessPayment(ctx context.Context, id string) (*payment.Payment, error) {
	paymentObj, err := p.PaymentRepo.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	// For payment links, accept both INITIATED and PENDING statuses
	// For other payment methods, only accept PENDING status
	if paymentObj.PaymentMethodType == types.PaymentMethodTypePaymentLink {
		if paymentObj.PaymentStatus != types.PaymentStatusInitiated && paymentObj.PaymentStatus != types.PaymentStatusPending {
			return paymentObj, ierr.NewError("payment is not in initiated or pending state").
				WithHint("Invalid payment status for processing payment link").
				WithReportableDetails(map[string]interface{}{
					"payment_id": paymentObj.ID,
					"status":     paymentObj.PaymentStatus,
				}).
				Mark(ierr.ErrInvalidOperation)
		}
	} else {
		// If payment is not in pending state, return error
		if paymentObj.PaymentStatus != types.PaymentStatusPending {
			return paymentObj, ierr.NewError("payment is not in pending state").
				WithHint("Invalid payment status for processing payment").
				WithReportableDetails(map[string]interface{}{
					"payment_id": paymentObj.ID,
					"status":     paymentObj.PaymentStatus,
				}).
				Mark(ierr.ErrInvalidOperation)
		}
	}

	// Create a new payment attempt if tracking is enabled
	var attempt *payment.PaymentAttempt
	if paymentObj.TrackAttempts {
		attempt, err = p.createNewAttempt(ctx, paymentObj)
		if err != nil {
			return paymentObj, err
		}
	}

	// For payment links, if status is INITIATED, we need to create the payment link
	// If status is already PENDING, we don't need to do anything more
	if paymentObj.PaymentMethodType == types.PaymentMethodTypePaymentLink && paymentObj.PaymentStatus == types.PaymentStatusInitiated {
		// Update payment status to processing temporarily
		paymentObj.PaymentStatus = types.PaymentStatusProcessing
		paymentObj.UpdatedAt = time.Now().UTC()
		if err := p.PaymentRepo.Update(ctx, paymentObj); err != nil {
			return paymentObj, err
		}
		p.publishWebhookEvent(ctx, types.WebhookEventPaymentPending, paymentObj.ID)
	} else if paymentObj.PaymentMethodType != types.PaymentMethodTypePaymentLink {
		// Update payment status to processing and fire pending event
		// TODO: take a lock on the payment object here to avoid race conditions
		paymentObj.PaymentStatus = types.PaymentStatusProcessing
		paymentObj.UpdatedAt = time.Now().UTC()
		if err := p.PaymentRepo.Update(ctx, paymentObj); err != nil {
			return paymentObj, err
		}
		p.publishWebhookEvent(ctx, types.WebhookEventPaymentPending, paymentObj.ID)
	}

	// Process payment based on payment method type
	var processErr error
	switch paymentObj.PaymentMethodType {
	case types.PaymentMethodTypeOffline:
		// For offline payments, we just mark them as succeeded immediately
		processErr = nil
	case types.PaymentMethodTypeCredits:
		processErr = p.handleCreditsPayment(ctx, paymentObj)
	case types.PaymentMethodTypePaymentLink:
		// For payment links, create the actual payment link
		// If status is already PENDING, skip creation
		if paymentObj.PaymentStatus == types.PaymentStatusPending {
			processErr = nil // Already processed
		} else {
			processErr = p.handlePaymentLinkCreation(ctx, paymentObj)
		}
	case types.PaymentMethodTypeCard:
		// TODO: Implement card payment processing
		processErr = ierr.NewError("card payment processing not implemented").
			WithHint("Card payment processing is not implemented").
			WithReportableDetails(map[string]interface{}{
				"payment_id": paymentObj.ID,
			}).
			Mark(ierr.ErrInvalidOperation)
	case types.PaymentMethodTypeACH:
		// TODO: Implement ACH payment processing
		processErr = ierr.NewError("ACH payment processing not implemented").
			WithHint("ACH payment processing is not supported yet").
			WithReportableDetails(map[string]interface{}{
				"payment_id": paymentObj.ID,
			}).
			Mark(ierr.ErrInvalidOperation)
	default:
		processErr = ierr.NewError(fmt.Sprintf("unsupported payment method type: %s", paymentObj.PaymentMethodType)).
			WithHint("Unsupported payment method type").
			WithReportableDetails(map[string]interface{}{
				"payment_id": paymentObj.ID,
			}).
			Mark(ierr.ErrInvalidOperation)
	}

	// Update attempt status if tracking is enabled
	if paymentObj.TrackAttempts && attempt != nil {
		if processErr != nil {
			// For payment links, keep attempt as pending even on error
			if paymentObj.PaymentMethodType == types.PaymentMethodTypePaymentLink {
				attempt.PaymentStatus = types.PaymentStatusPending
				attempt.ErrorMessage = lo.ToPtr(processErr.Error())
			} else {
				attempt.PaymentStatus = types.PaymentStatusFailed
				attempt.ErrorMessage = lo.ToPtr(processErr.Error())
			}
		} else {
			// For payment links, keep attempt as pending
			if paymentObj.PaymentMethodType == types.PaymentMethodTypePaymentLink {
				attempt.PaymentStatus = types.PaymentStatusPending
			} else {
				attempt.PaymentStatus = types.PaymentStatusSucceeded
			}
		}
		attempt.UpdatedAt = time.Now().UTC()
		if err := p.PaymentRepo.UpdateAttempt(ctx, attempt); err != nil {
			p.Logger.Errorw("failed to update payment attempt", "error", err)
		}
	}

	// Update payment status based on processing result
	if processErr != nil {
		// For payment links, if the error occurred during payment link creation,
		// keep the status as INITIATED instead of marking as FAILED
		if paymentObj.PaymentMethodType == types.PaymentMethodTypePaymentLink {
			// Keep status as INITIATED when Stripe SDK fails for payment links
			p.Logger.Infow("keeping payment link as initiated due to Stripe SDK failure",
				"payment_id", paymentObj.ID,
				"status", paymentObj.PaymentStatus,
				"error", processErr.Error())
			// Reset status back to INITIATED if it was changed to PROCESSING
			if paymentObj.PaymentStatus == types.PaymentStatusProcessing {
				paymentObj.PaymentStatus = types.PaymentStatusInitiated
			}
			// Don't set failed_at or error_message for payment links
		} else {
			// For other cases, mark as failed
			paymentObj.PaymentStatus = types.PaymentStatusFailed
			errMsg := processErr.Error()
			paymentObj.ErrorMessage = lo.ToPtr(errMsg)
			failedAt := time.Now().UTC()
			paymentObj.FailedAt = &failedAt
			p.publishWebhookEvent(ctx, types.WebhookEventPaymentFailed, paymentObj.ID)
		}
	} else {
		// For payment links, keep as pending until external payment completion
		if paymentObj.PaymentMethodType == types.PaymentMethodTypePaymentLink {
			paymentObj.PaymentStatus = types.PaymentStatusPending
			paymentObj.SucceededAt = nil // Keep succeeded_at as nil
			p.Logger.Infow("keeping payment link as pending", "payment_id", paymentObj.ID, "status", paymentObj.PaymentStatus)
			p.publishWebhookEvent(ctx, types.WebhookEventPaymentPending, paymentObj.ID)
		} else {
			paymentObj.PaymentStatus = types.PaymentStatusSucceeded
			succeededAt := time.Now().UTC()
			paymentObj.SucceededAt = &succeededAt
			p.Logger.Infow("marking payment as succeeded", "payment_id", paymentObj.ID, "status", paymentObj.PaymentStatus)
			p.publishWebhookEvent(ctx, types.WebhookEventPaymentSuccess, paymentObj.ID)
		}
	}

	paymentObj.UpdatedAt = time.Now().UTC()
	if err := p.PaymentRepo.Update(ctx, paymentObj); err != nil {
		return paymentObj, err
	}

	// If payment succeeded, handle post-processing
	if paymentObj.PaymentStatus == types.PaymentStatusSucceeded {
		if err := p.handlePostProcessing(ctx, paymentObj); err != nil {
			p.Logger.Errorw("failed to handle post-processing", "error", err, "payment_id", paymentObj.ID)
			// Note: We don't return this error as the payment itself was successful
		}
	}

	if processErr != nil {
		return paymentObj, processErr
	}

	return paymentObj, nil
}

func (p *paymentProcessor) handlePaymentLinkCreation(ctx context.Context, paymentObj *payment.Payment) error {
	// Get the invoice to get customer information
	invoice, err := p.InvoiceRepo.Get(ctx, paymentObj.DestinationID)
	if err != nil {
		return ierr.WithError(err).
			WithHint("Failed to get invoice for payment link creation").
			WithReportableDetails(map[string]interface{}{
				"payment_id": paymentObj.ID,
				"invoice_id": paymentObj.DestinationID,
			}).
			Mark(ierr.ErrValidation)
	}

	// Create payment link using the specified gateway
	gatewayService := NewPaymentGatewayService(p.ServiceParams)

	// Extract success and cancel URLs from metadata
	successURL := ""
	cancelURL := ""
	if paymentObj.Metadata != nil {
		if url, exists := paymentObj.Metadata["success_url"]; exists {
			successURL = url
		}
		if url, exists := paymentObj.Metadata["cancel_url"]; exists {
			cancelURL = url
		}
	}

	// Convert to payment link request
	paymentLinkReq := &dto.CreatePaymentLinkRequest{
		InvoiceID:  paymentObj.DestinationID,
		CustomerID: invoice.CustomerID,
		Amount:     paymentObj.Amount,
		Currency:   paymentObj.Currency,
		SuccessURL: successURL,
		CancelURL:  cancelURL,
		Gateway: func() *types.PaymentGatewayType {
			if paymentObj.PaymentGateway != nil {
				gatewayType := types.PaymentGatewayType(*paymentObj.PaymentGateway)
				return &gatewayType
			}
			return nil
		}(),
		Metadata: paymentObj.Metadata,
	}

	paymentLinkResp, err := gatewayService.CreatePaymentLink(ctx, paymentLinkReq)
	if err != nil {
		// If Stripe SDK fails, keep payment status as INITIATED and return error
		p.Logger.Errorw("failed to create payment link via Stripe SDK",
			"error", err,
			"payment_id", paymentObj.ID,
			"invoice_id", paymentObj.DestinationID)
		return err
	}

	// If Stripe SDK succeeds, update payment status to PENDING
	paymentObj.PaymentStatus = types.PaymentStatusPending

	// Update payment with gateway information
	paymentObj.GatewayTrackingID = &paymentLinkResp.ID // Store session_id in gateway_tracking_id
	paymentObj.GatewayPaymentID = &paymentLinkResp.PaymentIntentID
	if paymentObj.GatewayMetadata == nil {
		paymentObj.GatewayMetadata = types.Metadata{}
	}
	paymentObj.GatewayMetadata["payment_url"] = paymentLinkResp.PaymentURL
	paymentObj.GatewayMetadata["gateway"] = paymentLinkResp.Gateway
	paymentObj.GatewayMetadata["session_id"] = paymentLinkResp.ID

	// Update the payment record
	if err := p.PaymentRepo.Update(ctx, paymentObj); err != nil {
		return ierr.WithError(err).
			WithHint("Failed to update payment with payment link information").
			WithReportableDetails(map[string]interface{}{
				"payment_id": paymentObj.ID,
			}).
			Mark(ierr.ErrDatabase)
	}

	p.Logger.Infow("successfully created payment link and updated status to pending",
		"payment_id", paymentObj.ID,
		"session_id", paymentLinkResp.ID,
		"payment_url", paymentLinkResp.PaymentURL)

	return nil
}

func (p *paymentProcessor) handleCreditsPayment(ctx context.Context, paymentObj *payment.Payment) error {
	// In case of credits, we need to find the wallet by the set payment method id
	selectedWallet, err := p.WalletRepo.GetWalletByID(ctx, paymentObj.PaymentMethodID)
	if err != nil {
		return err
	}

	// Validate wallet status
	if selectedWallet.WalletStatus != types.WalletStatusActive {
		return ierr.NewError("wallet is not active").
			WithHint("Wallet is not active").
			WithReportableDetails(map[string]interface{}{
				"wallet_id": selectedWallet.ID,
			}).
			Mark(ierr.ErrInvalidOperation)
	}

	// Validate currency match
	if !types.IsMatchingCurrency(selectedWallet.Currency, paymentObj.Currency) {
		return ierr.NewError("wallet currency does not match payment currency").
			WithHint("Wallet currency does not match payment currency").
			WithReportableDetails(map[string]interface{}{
				"wallet_id": selectedWallet.ID,
				"currency":  paymentObj.Currency,
			}).
			Mark(ierr.ErrInvalidOperation)
	}

	// Validate sufficient balance
	if selectedWallet.Balance.LessThan(paymentObj.Amount) {
		return ierr.NewError("wallet balance is less than payment amount").
			WithHint("Wallet balance is less than payment amount").
			WithReportableDetails(map[string]interface{}{
				"wallet_id": selectedWallet.ID,
				"amount":    paymentObj.Amount,
			}).
			Mark(ierr.ErrInvalidOperation)
	}

	// Create wallet operation
	operation := &wallet.WalletOperation{
		WalletID:          selectedWallet.ID,
		Type:              types.TransactionTypeDebit,
		Amount:            paymentObj.Amount,
		ReferenceType:     types.WalletTxReferenceTypePayment,
		ReferenceID:       paymentObj.ID,
		Description:       fmt.Sprintf("Payment for invoice %s", paymentObj.DestinationID),
		TransactionReason: types.TransactionReasonInvoicePayment,
		Metadata: types.Metadata{
			"payment_id":     paymentObj.ID,
			"invoice_id":     paymentObj.DestinationID,
			"payment_method": string(paymentObj.PaymentMethodType),
			"wallet_type":    string(selectedWallet.WalletType),
		},
	}

	// Create wallet service
	walletService := NewWalletService(p.ServiceParams)

	// Transactional workflow begins here
	err = p.DB.WithTx(ctx, func(ctx context.Context) error {
		// Debit wallet
		if err := walletService.DebitWallet(ctx, operation); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return err
	}

	return nil
}

func (p *paymentProcessor) handlePostProcessing(ctx context.Context, paymentObj *payment.Payment) error {
	switch paymentObj.DestinationType {
	case types.PaymentDestinationTypeInvoice:
		return p.handleInvoicePostProcessing(ctx, paymentObj)
	default:
		return ierr.NewError("unsupported payment destination type").
			WithHint("Unsupported payment destination type").
			WithReportableDetails(map[string]interface{}{
				"payment_id":       paymentObj.ID,
				"destination_type": paymentObj.DestinationType,
			}).
			Mark(ierr.ErrInvalidOperation)
	}
}

func (p *paymentProcessor) handleInvoicePostProcessing(ctx context.Context, paymentObj *payment.Payment) error {
	// Get the invoice
	invoice, err := p.InvoiceRepo.Get(ctx, paymentObj.DestinationID)
	if err != nil {
		return err
	}

	// Get all successful payments for this invoice
	filter := &types.PaymentFilter{
		DestinationType: lo.ToPtr(string(types.PaymentDestinationTypeInvoice)),
		DestinationID:   lo.ToPtr(invoice.ID),
		PaymentStatus:   lo.ToPtr(string(types.PaymentStatusSucceeded)),
	}
	payments, err := p.PaymentRepo.List(ctx, filter)
	if err != nil {
		return err
	}

	// Calculate total paid amount
	totalPaid := decimal.Zero
	for _, p := range payments {
		if p.Currency != invoice.Currency {
			return ierr.NewError("payment currency does not match invoice currency").
				WithHint("Payment currency does not match invoice currency").
				WithReportableDetails(map[string]interface{}{
					"payment_id":       paymentObj.ID,
					"invoice_id":       invoice.ID,
					"payment_currency": p.Currency,
					"invoice_currency": invoice.Currency,
				}).
				Mark(ierr.ErrInvalidOperation)
		}
		totalPaid = totalPaid.Add(p.Amount)
	}

	// Update invoice payment status and amounts
	invoice.AmountPaid = totalPaid
	invoice.AmountRemaining = invoice.AmountDue.Sub(totalPaid)
	if invoice.AmountRemaining.IsZero() {
		paidAt := time.Now().UTC()
		invoice.PaidAt = &paidAt
	}

	// Ensure amount remaining is not less than zero even in case of excess payments.
	// TODO: think of a better way to handle Partial payments
	if invoice.AmountRemaining.LessThan(decimal.Zero) {
		invoice.AmountRemaining = decimal.Zero
	}

	if invoice.AmountRemaining.IsZero() {
		invoice.PaymentStatus = types.PaymentStatusSucceeded
	} else if invoice.AmountRemaining.LessThan(invoice.AmountDue) {
		invoice.PaymentStatus = types.PaymentStatusPending // Partial payment still keeps it pending
	}

	// Update the invoice
	if err := p.InvoiceRepo.Update(ctx, invoice); err != nil {
		return err
	}

	return nil
}

func (p *paymentProcessor) createNewAttempt(ctx context.Context, paymentObj *payment.Payment) (*payment.PaymentAttempt, error) {
	// Get latest attempt to determine attempt number
	latestAttempt, err := p.PaymentRepo.GetLatestAttempt(ctx, paymentObj.ID)
	if err != nil && !ierr.IsNotFound(err) {
		return nil, err
	}

	attemptNumber := 1
	if latestAttempt != nil {
		attemptNumber = latestAttempt.AttemptNumber + 1
	}

	attempt := &payment.PaymentAttempt{
		ID:            types.GenerateUUIDWithPrefix(types.UUID_PREFIX_PAYMENT_ATTEMPT),
		PaymentID:     paymentObj.ID,
		AttemptNumber: attemptNumber,
		PaymentStatus: types.PaymentStatusProcessing,
		Metadata:      types.Metadata{},
		EnvironmentID: types.GetEnvironmentID(ctx),
		BaseModel:     types.GetDefaultBaseModel(ctx),
	}

	if err := p.PaymentRepo.CreateAttempt(ctx, attempt); err != nil {
		return nil, err
	}

	return attempt, nil
}

func (p *paymentProcessor) publishWebhookEvent(ctx context.Context, eventName string, paymentID string) {
	webhookPayload, err := json.Marshal(webhookDto.InternalPaymentEvent{
		PaymentID: paymentID,
		TenantID:  types.GetTenantID(ctx),
	})

	if err != nil {
		p.Logger.Errorw("failed to marshal webhook payload", "error", err)
		return
	}

	webhookEvent := &types.WebhookEvent{
		ID:            types.GenerateUUIDWithPrefix(types.UUID_PREFIX_WEBHOOK_EVENT),
		EventName:     eventName,
		TenantID:      types.GetTenantID(ctx),
		EnvironmentID: types.GetEnvironmentID(ctx),
		UserID:        types.GetUserID(ctx),
		Timestamp:     time.Now().UTC(),
		Payload:       json.RawMessage(webhookPayload),
	}
	if err := p.WebhookPublisher.PublishWebhook(ctx, webhookEvent); err != nil {
		p.Logger.Errorf("failed to publish %s event: %v", webhookEvent.EventName, err)
	}
}
