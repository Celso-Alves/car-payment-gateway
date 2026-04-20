// Package entity defines the canonical domain model for vehicle debts.
// These types are the authoritative specification (SPEC-002) that all
// adapters must normalize their provider-specific formats into.
package entity

import "time"

// DebtType identifies the category of a vehicle debt.
// Adding a new category requires only a new constant here and a
// corresponding InterestStrategy — no changes to use-case logic.
type DebtType string

const (
	DebtTypeIPVA  DebtType = "IPVA"
	DebtTypeMULTA DebtType = "MULTA"
)

// Debt is the canonical representation of a single vehicle debt,
// independent of the provider format (JSON, XML, etc.).
type Debt struct {
	Type    DebtType
	Amount  float64
	DueDate time.Time
}

// UpdatedDebt holds the original debt along with the interest-adjusted amount.
type UpdatedDebt struct {
	Debt
	UpdatedAmount float64
	DaysOverdue   int
}

// PaymentSummary carries the aggregate totals across all debts.
type PaymentSummary struct {
	TotalOriginal float64 `json:"total_original"`
	TotalUpdated  float64 `json:"total_atualizado"`
}

// Installment represents a single credit card installment option.
type Installment struct {
	Quantity       int     `json:"quantidade"`
	InstallmentAmt float64 `json:"valor_parcela"`
}

// PixOption represents a PIX payment with the 8% discount applied.
type PixOption struct {
	TotalWithDiscount float64 `json:"total_com_desconto"`
}

// CardOption holds the available installment plans for credit card payment.
type CardOption struct {
	Installments []Installment `json:"parcelas"`
}

// PaymentOption represents a complete payment simulation for a given
// debt grouping (TOTAL, SOMENTE_IPVA, SOMENTE_MULTAS, etc.).
type PaymentOption struct {
	Type       string     `json:"tipo"`
	BaseAmount float64    `json:"valor_base"`
	Pix        PixOption  `json:"pix"`
	Card       CardOption `json:"cartao_credito"`
}

// ConsultResult is the full API response payload (SPEC-006).
type ConsultResult struct {
	Plate   string         `json:"placa"`
	Summary PaymentSummary `json:"resumo"`
	Payment struct {
		Options []PaymentOption `json:"opcoes"`
	} `json:"pagamentos"`
}
