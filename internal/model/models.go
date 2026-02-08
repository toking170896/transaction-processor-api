package model

import (
	"github.com/shopspring/decimal"
	"time"
)

type User struct {
	ID        int64           `json:"id"`
	Balance   decimal.Decimal `json:"balance"`
	Version   int             `json:"version"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

type Transaction struct {
	ID            int64             `json:"id"`
	TransactionID string            `json:"transaction_id"`
	UserID        int64             `json:"user_id"`
	SourceType    SourceType        `json:"source_type"`
	State         State             `json:"state"`
	Amount        decimal.Decimal   `json:"amount"`
	Status        TransactionStatus `json:"status"`
	CancelledAt   *time.Time        `json:"cancelled_at,omitempty"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
}

type TransactionRequest struct {
	State         string `json:"state" binding:"required,oneof=win lost" example:"win" enums:"win,lost"`
	Amount        string `json:"amount" binding:"required" example:"10.15"`
	TransactionID string `json:"transaction_id" binding:"required,uuid" example:"550e8400-e29b-41d4-a716-446655440000"`
}

type TransactionResponse struct {
	Status  string `json:"status" example:"success"`
	Balance string `json:"balance" example:"110.15"`
	Message string `json:"message,omitempty" example:"Transaction processed successfully"`
}

type ErrorResponse struct {
	Error   string `json:"error" example:"insufficient balance"`
	Code    string `json:"code,omitempty" example:"INSUFFICIENT_BALANCE"`
	Details string `json:"details,omitempty"`
}

type BalanceResponse struct {
	UserID  int64  `json:"user_id" example:"1"`
	Balance string `json:"balance" example:"100.50"`
}

type TransactionListResponse struct {
	Transactions []*Transaction `json:"transactions"`
	Total        int            `json:"total"`
	Limit        int            `json:"limit"`
	Offset       int            `json:"offset"`
}
