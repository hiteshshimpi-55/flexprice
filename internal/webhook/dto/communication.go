package webhookDto

import "github.com/flexprice/flexprice/internal/api/dto"

type InternalCommunicationEvent struct {
	InvoiceID string `json:"invoice_id"`
	TenantID  string `json:"tenant_id"`
}

type CommunicationWebhookPayload struct {
	Invoice *dto.InvoiceResponse `json:"invoice"`
}

func NewCommunicationWebhookPayload(invoice *dto.InvoiceResponse) *CommunicationWebhookPayload {
	return &CommunicationWebhookPayload{Invoice: invoice}
}
