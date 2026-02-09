package service

import (
	"context"
	"transaction-processor/internal/model"
)

// TransactionService defines the business logic for processing transactions
type TransactionService interface {
	ProcessTransaction(ctx context.Context, req *model.TransactionRequest, sourceType model.SourceType, userID int64) (*model.TransactionResponse, error)
	GetBalance(ctx context.Context, userID int64) (*model.BalanceResponse, error)
	GetTransactionsByUser(ctx context.Context, userID int64, limit, offset int) ([]*model.Transaction, error)
}

// CancellationService defines the business logic for cancelling transactions
type CancellationService interface {
	// ProcessOddRecordCancellation cancels odd-numbered processed transactions and adjusts user balances
	ProcessOddRecordCancellation(ctx context.Context) error
}
