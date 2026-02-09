package service

import (
	"context"
	"testing"
	"transaction-processor/internal/model"
	"transaction-processor/mocks/repository"

	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestProcessTransaction_Win_HappyPath(t *testing.T) {
	ctx := context.Background()
	logger := zerolog.Nop()

	mockUserRepo := mocks.NewUserRepository(t)
	mockTransRepo := mocks.NewTransactionRepository(t)
	mockDBManager := mocks.NewDBManager(t)

	mockDBManager.On("WithTransaction", ctx, mock.Anything).Return(func(ctx context.Context, fn func(pgx.Tx) error) error { return fn(nil) })
	mockTransRepo.On("GetTransaction", ctx, "550e8400-e29b-41d4-a716-446655440000", mock.Anything).Return(nil, model.ErrTransactionNotFound)
	mockUserRepo.On("GetUserForUpdate", ctx, int64(1), mock.Anything).Return(&model.User{
		ID:      1,
		Balance: decimal.NewFromInt(100),
		Version: 1,
	}, nil)
	mockUserRepo.On("UpdateBalance", ctx, int64(1), decimal.RequireFromString("110.50"), mock.Anything).Return(nil)
	mockTransRepo.On("InsertTransaction", ctx, mock.MatchedBy(func(trans *model.Transaction) bool {
		return trans.TransactionID == "550e8400-e29b-41d4-a716-446655440000" &&
			trans.UserID == 1 &&
			trans.Amount.Equal(decimal.RequireFromString("10.50")) &&
			trans.State == "win"
	}), mock.Anything).Return(nil)

	service := NewTransactionService(mockUserRepo, mockTransRepo, mockDBManager, logger)

	req := &model.TransactionRequest{
		State:         "win",
		Amount:        "10.50",
		TransactionID: "550e8400-e29b-41d4-a716-446655440000",
	}

	resp, err := service.ProcessTransaction(ctx, req, "game", 1)

	require.NoError(t, err)
	assert.Equal(t, "success", resp.Status)
	assert.Equal(t, "110.50", resp.Balance)
	assert.Equal(t, "Transaction processed successfully", resp.Message)
}

func TestProcessTransaction_Lost_HappyPath(t *testing.T) {
	ctx := context.Background()
	logger := zerolog.Nop()

	mockUserRepo := mocks.NewUserRepository(t)
	mockTransRepo := mocks.NewTransactionRepository(t)
	mockDBManager := mocks.NewDBManager(t)

	mockDBManager.On("WithTransaction", ctx, mock.Anything).Return(func(ctx context.Context, fn func(pgx.Tx) error) error {
		return fn(nil)
	})
	mockTransRepo.On("GetTransaction", ctx, "550e8400-e29b-41d4-a716-446655440001", mock.Anything).Return(nil, model.ErrTransactionNotFound)
	mockUserRepo.On("GetUserForUpdate", ctx, int64(1), mock.Anything).Return(&model.User{
		ID:      1,
		Balance: decimal.NewFromInt(100),
		Version: 1,
	}, nil)
	mockUserRepo.On("UpdateBalance", ctx, int64(1), decimal.RequireFromString("89.50"), mock.Anything).Return(nil)
	mockTransRepo.On("InsertTransaction", ctx, mock.MatchedBy(func(trans *model.Transaction) bool {
		return trans.TransactionID == "550e8400-e29b-41d4-a716-446655440001" &&
			trans.UserID == 1 &&
			trans.Amount.Equal(decimal.RequireFromString("10.50")) &&
			trans.State == "lost"
	}), mock.Anything).Return(nil)

	service := NewTransactionService(mockUserRepo, mockTransRepo, mockDBManager, logger)

	req := &model.TransactionRequest{
		State:         "lost",
		Amount:        "10.50",
		TransactionID: "550e8400-e29b-41d4-a716-446655440001",
	}

	resp, err := service.ProcessTransaction(ctx, req, "game", 1)

	require.NoError(t, err)
	assert.Equal(t, "success", resp.Status)
	assert.Equal(t, "89.50", resp.Balance)
}

func TestProcessTransaction_DuplicateTransaction_SameUser(t *testing.T) {
	ctx := context.Background()
	logger := zerolog.Nop()

	mockUserRepo := mocks.NewUserRepository(t)
	mockTransRepo := mocks.NewTransactionRepository(t)
	mockDBManager := mocks.NewDBManager(t)

	mockDBManager.On("WithTransaction", ctx, mock.Anything).Return(func(ctx context.Context, fn func(pgx.Tx) error) error {
		return fn(nil)
	})
	mockTransRepo.On("GetTransaction", ctx, "550e8400-e29b-41d4-a716-446655440002", mock.Anything).Return(&model.Transaction{
		ID:            1,
		TransactionID: "550e8400-e29b-41d4-a716-446655440002",
		UserID:        1,
		State:         "win",
		Amount:        decimal.NewFromFloat(10.50),
		Status:        "processed",
	}, nil)
	mockUserRepo.On("GetBalance", ctx, int64(1), mock.Anything).Return(decimal.NewFromInt(150), nil)

	service := NewTransactionService(mockUserRepo, mockTransRepo, mockDBManager, logger)

	req := &model.TransactionRequest{
		State:         "win",
		Amount:        "10.50",
		TransactionID: "550e8400-e29b-41d4-a716-446655440002",
	}

	resp, err := service.ProcessTransaction(ctx, req, "game", 1)

	require.NoError(t, err)
	assert.Equal(t, "already_processed", resp.Status)
	assert.Equal(t, "150.00", resp.Balance)
}

func TestProcessTransaction_DuplicateTransaction_DifferentUser(t *testing.T) {
	ctx := context.Background()
	logger := zerolog.Nop()

	mockUserRepo := mocks.NewUserRepository(t)
	mockTransRepo := mocks.NewTransactionRepository(t)
	mockDBManager := mocks.NewDBManager(t)

	mockDBManager.On("WithTransaction", ctx, mock.Anything).Return(func(ctx context.Context, fn func(pgx.Tx) error) error {
		return fn(nil)
	})
	mockTransRepo.On("GetTransaction", ctx, "550e8400-e29b-41d4-a716-446655440003", mock.Anything).Return(&model.Transaction{
		ID:            1,
		TransactionID: "550e8400-e29b-41d4-a716-446655440003",
		UserID:        999,
		State:         "win",
		Amount:        decimal.NewFromFloat(10.50),
	}, nil)

	service := NewTransactionService(mockUserRepo, mockTransRepo, mockDBManager, logger)

	req := &model.TransactionRequest{
		State:         "win",
		Amount:        "10.50",
		TransactionID: "550e8400-e29b-41d4-a716-446655440003",
	}

	resp, err := service.ProcessTransaction(ctx, req, "game", 1) // User 1

	require.Error(t, err)
	assert.Nil(t, resp)
	assert.ErrorIs(t, err, model.ErrDuplicateTransaction)
	assert.Contains(t, err.Error(), "already exists for user 999")
}

func TestProcessTransaction_InsufficientBalance(t *testing.T) {
	ctx := context.Background()
	logger := zerolog.Nop()

	mockUserRepo := mocks.NewUserRepository(t)
	mockTransRepo := mocks.NewTransactionRepository(t)
	mockDBManager := mocks.NewDBManager(t)

	mockDBManager.On("WithTransaction", ctx, mock.Anything).Return(func(ctx context.Context, fn func(pgx.Tx) error) error { return fn(nil) })
	mockTransRepo.On("GetTransaction", ctx, "550e8400-e29b-41d4-a716-446655440004", mock.Anything).Return(nil, model.ErrTransactionNotFound)
	mockUserRepo.On("GetUserForUpdate", ctx, int64(1), mock.Anything).Return(&model.User{
		ID:      1,
		Balance: decimal.NewFromInt(5),
		Version: 1,
	}, nil)

	service := NewTransactionService(mockUserRepo, mockTransRepo, mockDBManager, logger)

	req := &model.TransactionRequest{
		State:         "lost",
		Amount:        "10.50",
		TransactionID: "550e8400-e29b-41d4-a716-446655440004",
	}

	resp, err := service.ProcessTransaction(ctx, req, "game", 1)

	require.Error(t, err)
	assert.Nil(t, resp)
	assert.ErrorIs(t, err, model.ErrInsufficientBalance)
}

func TestProcessTransaction_InvalidAmount_Zero(t *testing.T) {
	ctx := context.Background()
	logger := zerolog.Nop()

	mockUserRepo := mocks.NewUserRepository(t)
	mockTransRepo := mocks.NewTransactionRepository(t)
	mockDBManager := mocks.NewDBManager(t)

	service := NewTransactionService(mockUserRepo, mockTransRepo, mockDBManager, logger)

	req := &model.TransactionRequest{
		State:         "win",
		Amount:        "0",
		TransactionID: "550e8400-e29b-41d4-a716-446655440005",
	}

	resp, err := service.ProcessTransaction(ctx, req, "game", 1)

	require.Error(t, err)
	assert.Nil(t, resp)
	assert.ErrorIs(t, err, model.ErrInvalidAmount)
	assert.Contains(t, err.Error(), "amount must be positive")
}

func TestProcessTransaction_InvalidAmount_Negative(t *testing.T) {
	ctx := context.Background()
	logger := zerolog.Nop()

	mockUserRepo := mocks.NewUserRepository(t)
	mockTransRepo := mocks.NewTransactionRepository(t)
	mockDBManager := mocks.NewDBManager(t)

	service := NewTransactionService(mockUserRepo, mockTransRepo, mockDBManager, logger)

	req := &model.TransactionRequest{
		State:         "win",
		Amount:        "-10.50",
		TransactionID: "550e8400-e29b-41d4-a716-446655440006",
	}

	resp, err := service.ProcessTransaction(ctx, req, "game", 1)

	require.Error(t, err)
	assert.Nil(t, resp)
	assert.ErrorIs(t, err, model.ErrInvalidAmount)
}

func TestProcessTransaction_UserNotFound(t *testing.T) {
	ctx := context.Background()
	logger := zerolog.Nop()

	mockUserRepo := mocks.NewUserRepository(t)
	mockTransRepo := mocks.NewTransactionRepository(t)
	mockDBManager := mocks.NewDBManager(t)

	mockDBManager.On("WithTransaction", ctx, mock.Anything).Return(func(ctx context.Context, fn func(pgx.Tx) error) error { return fn(nil) })
	mockTransRepo.On("GetTransaction", ctx, "550e8400-e29b-41d4-a716-446655440008", mock.Anything).Return(nil, model.ErrTransactionNotFound)
	mockUserRepo.On("GetUserForUpdate", ctx, int64(999), mock.Anything).Return(nil, model.ErrUserNotFound)

	service := NewTransactionService(mockUserRepo, mockTransRepo, mockDBManager, logger)

	req := &model.TransactionRequest{
		State:         "win",
		Amount:        "10.50",
		TransactionID: "550e8400-e29b-41d4-a716-446655440008",
	}

	resp, err := service.ProcessTransaction(ctx, req, "game", 999)

	require.Error(t, err)
	assert.Nil(t, resp)
	assert.ErrorIs(t, err, model.ErrUserNotFound)
}
