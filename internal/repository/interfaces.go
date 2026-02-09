package repository

import (
	"context"
	"transaction-processor/internal/model"

	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
)

// DBManager provides database transaction management
type DBManager interface {
	// WithTransaction executes a function within a database transaction
	WithTransaction(ctx context.Context, fn func(pgx.Tx) error) error
}

// UserRepository defines operations for user/balance management
type UserRepository interface {
	// GetUserForUpdate retrieves a user with row-level lock (must be in transaction)
	GetUserForUpdate(ctx context.Context, userID int64, tx pgx.Tx) (*model.User, error)

	// GetBalance retrieves the current balance for a user (read-only)
	GetBalance(ctx context.Context, userID int64, tx ...pgx.Tx) (decimal.Decimal, error)

	// UpdateBalance updates user balance
	UpdateBalance(ctx context.Context, userID int64, balance decimal.Decimal, tx pgx.Tx) error
}

// TransactionRepository defines operations for transaction management
type TransactionRepository interface {
	// InsertTransaction creates a new transaction record
	InsertTransaction(ctx context.Context, trans *model.Transaction, tx pgx.Tx) error

	// GetTransaction retrieves a transaction by its transaction ID
	GetTransaction(ctx context.Context, transactionID string, tx ...pgx.Tx) (*model.Transaction, error)

	// GetTransactionsByUser retrieves paginated transactions for a user
	GetTransactionsByUser(ctx context.Context, userID int64, limit, offset int) ([]*model.Transaction, error)

	// GetLatestOddProcessedTransactions retrieves latest odd-numbered processed transactions
	GetLatestOddProcessedTransactions(ctx context.Context, limit int) ([]*model.Transaction, error)

	// CancelTransactionIfProcessed cancels a transaction if status is processed
	CancelTransactionIfProcessed(ctx context.Context, id int64, tx pgx.Tx) (bool, error)

	// LockTransactionForCancellation locks a transaction row for cancellation if it's still processed
	LockTransactionForCancellation(ctx context.Context, id int64, tx pgx.Tx) (bool, error)
}
