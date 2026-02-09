package postgres

import (
	"context"
	"errors"
	"fmt"
	"transaction-processor/internal/model"
	"transaction-processor/internal/repository"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// Ensure implementation satisfies interface at compile time
var _ repository.UserRepository = (*UserRepositoryImpl)(nil)

// UserRepositoryImpl is the PostgreSQL implementation of UserRepository
type UserRepositoryImpl struct {
	*TransactionManager
}

func NewUserRepository(pool *pgxpool.Pool) repository.UserRepository {
	return &UserRepositoryImpl{
		TransactionManager: NewTransactionManager(pool),
	}
}

// GetUserForUpdate retrieves a user with row-level lock
func (r *UserRepositoryImpl) GetUserForUpdate(ctx context.Context, userID int64, tx pgx.Tx) (*model.User, error) {
	query := `SELECT id, balance, version, created_at, updated_at FROM users WHERE id = $1 FOR UPDATE`

	user := &model.User{}
	err := tx.QueryRow(ctx, query, userID).Scan(&user.ID, &user.Balance, &user.Version, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, model.ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user for update: %w", err)
	}
	return user, nil
}

// GetBalance get the current balance for a user
func (r *UserRepositoryImpl) GetBalance(ctx context.Context, userID int64, tx ...pgx.Tx) (decimal.Decimal, error) {
	query := `SELECT balance FROM users WHERE id = $1`
	var balance decimal.Decimal
	executor := r.getExecutor(tx...)
	err := executor.QueryRow(ctx, query, userID).Scan(&balance)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return decimal.Zero, model.ErrUserNotFound
		}
		return decimal.Zero, fmt.Errorf("failed to get balance: %w", err)
	}
	return balance, nil
}

// UpdateBalance update user balance
func (r *UserRepositoryImpl) UpdateBalance(ctx context.Context, userID int64, balance decimal.Decimal, tx pgx.Tx) error {
	query := `
        UPDATE users 
        SET balance = $1, version = version + 1, updated_at = NOW()
        WHERE id = $2`

	commandTag, err := tx.Exec(ctx, query, balance, userID)
	if err != nil {
		var pgErr *pgconn.PgError
		// check if error is constraint violation, CONSTRAINT balance_non_negative CHECK (balance >= 0)
		if errors.As(err, &pgErr) && pgErr.Code == "23514" {
			return model.ErrInsufficientBalance
		}
		return fmt.Errorf("failed to update balance: %w", err)
	}

	if commandTag.RowsAffected() == 0 {
		return model.ErrUserNotFound
	}
	return nil
}
