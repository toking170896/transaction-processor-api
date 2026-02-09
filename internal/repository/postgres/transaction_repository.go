package postgres

import (
	"context"
	"errors"
	"fmt"
	"transaction-processor/internal/model"
	"transaction-processor/internal/repository"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Ensure implementation satisfies interface at compile time
var _ repository.TransactionRepository = (*TransactionRepositoryImpl)(nil)

// TransactionRepositoryImpl is the PostgreSQL implementation of TransactionRepository
type TransactionRepositoryImpl struct {
	*TransactionManager
}

func NewTransactionRepository(pool *pgxpool.Pool) repository.TransactionRepository {
	return &TransactionRepositoryImpl{
		TransactionManager: NewTransactionManager(pool),
	}
}

// InsertTransaction creates a new transaction record
func (r *TransactionRepositoryImpl) InsertTransaction(ctx context.Context, trans *model.Transaction, tx pgx.Tx) error {
	query := `
        INSERT INTO transactions (transaction_id, user_id, source_type, state, amount, status)
        VALUES ($1, $2, $3, $4, $5, $6)
        RETURNING id, created_at, updated_at`

	err := tx.QueryRow(ctx, query, trans.TransactionID, trans.UserID, trans.SourceType, trans.State, trans.Amount, trans.Status).
		Scan(&trans.ID, &trans.CreatedAt, &trans.UpdatedAt)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return model.ErrDuplicateTransaction
		}
		return fmt.Errorf("failed to insert transaction: %w", err)
	}
	return nil
}

// GetTransaction retrieves a transaction by its transaction ID
func (r *TransactionRepositoryImpl) GetTransaction(ctx context.Context, transactionID string, tx ...pgx.Tx) (*model.Transaction, error) {
	query := `
        SELECT id, transaction_id, user_id, source_type, state, amount, status, cancelled_at, created_at, updated_at
        FROM transactions WHERE transaction_id = $1`

	trans := &model.Transaction{}
	executor := r.getExecutor(tx...)
	err := executor.QueryRow(ctx, query, transactionID).Scan(&trans.ID, &trans.TransactionID, &trans.UserID, &trans.SourceType, &trans.State, &trans.Amount, &trans.Status, &trans.CancelledAt, &trans.CreatedAt, &trans.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, model.ErrTransactionNotFound
		}
		return nil, fmt.Errorf("failed to get transaction: %w", err)
	}
	return trans, nil
}

// GetTransactionsByUser retrieves paginated transactions for a user
func (r *TransactionRepositoryImpl) GetTransactionsByUser(ctx context.Context, userID int64, limit, offset int) ([]*model.Transaction, error) {
	query := `
        SELECT id, transaction_id, user_id, source_type, state, amount, status, cancelled_at, created_at, updated_at
        FROM transactions WHERE user_id = $1
        ORDER BY created_at DESC
        LIMIT $2 OFFSET $3`

	rows, err := r.pool.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query transactions: %w", err)
	}
	defer rows.Close()

	var transactions []*model.Transaction
	for rows.Next() {
		trans := &model.Transaction{}
		if err := rows.Scan(&trans.ID, &trans.TransactionID, &trans.UserID, &trans.SourceType, &trans.State, &trans.Amount, &trans.Status, &trans.CancelledAt, &trans.CreatedAt, &trans.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan transaction: %w", err)
		}
		transactions = append(transactions, trans)
	}
	return transactions, nil
}

// GetLatestOddProcessedTransactions retrieves latest odd-numbered processed transactions
func (r *TransactionRepositoryImpl) GetLatestOddProcessedTransactions(ctx context.Context, limit int) ([]*model.Transaction, error) {
	query := `
        SELECT id, transaction_id, user_id, source_type, state, amount, status, cancelled_at, created_at, updated_at
        FROM transactions
        WHERE id % 2 = 1 AND status = 'processed'
        ORDER BY id DESC
        LIMIT $1`

	rows, err := r.pool.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query latest odd transactions: %w", err)
	}
	defer rows.Close()

	var transactions []*model.Transaction
	for rows.Next() {
		trans := &model.Transaction{}
		if err := rows.Scan(&trans.ID, &trans.TransactionID, &trans.UserID, &trans.SourceType, &trans.State, &trans.Amount, &trans.Status, &trans.CancelledAt, &trans.CreatedAt, &trans.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan transaction: %w", err)
		}
		transactions = append(transactions, trans)
	}
	return transactions, nil
}

// CancelTransactionIfProcessed cancels a transaction if status is processed
func (r *TransactionRepositoryImpl) CancelTransactionIfProcessed(ctx context.Context, id int64, tx pgx.Tx) (bool, error) {
	query := `
		UPDATE transactions
		SET status = $1,
		    cancelled_at = NOW(),
		    updated_at = NOW()
		WHERE id = $2
		  AND status = $3`

	result, err := tx.Exec(ctx, query, string(model.StatusCancelled), id, string(model.StatusProcessed))
	if err != nil {
		return false, fmt.Errorf("failed to cancel transaction: %w", err)
	}
	return result.RowsAffected() == 1, nil
}

// LockTransactionForCancellation locks a transaction row for cancellation if it's still processed
func (r *TransactionRepositoryImpl) LockTransactionForCancellation(ctx context.Context, id int64, tx pgx.Tx) (bool, error) {
	query := `SELECT id FROM transactions WHERE id = $1 AND status = 'processed' FOR UPDATE SKIP LOCKED`

	var lockedID int64
	err := tx.QueryRow(ctx, query, id).Scan(&lockedID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("failed to lock transaction for cancellation: %w", err)
	}
	return true, nil
}
