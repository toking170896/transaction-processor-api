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
)

func TestCancellationService_ProcessOddRecordCancellation_Success(t *testing.T) {
	ctx := context.Background()
	logger := zerolog.Nop()

	mockUserRepo := mocks.NewUserRepository(t)
	mockTransRepo := mocks.NewTransactionRepository(t)
	mockDBManager := mocks.NewDBManager(t)

	transactions := []*model.Transaction{
		{
			ID:     1,
			UserID: 1,
			State:  "win",
			Amount: decimal.NewFromInt(100),
			Status: "processed",
		},
	}

	mockTransRepo.On("GetLatestOddProcessedTransactions", ctx, 10).Return(transactions, nil)
	mockDBManager.On("WithTransaction", ctx, mock.Anything).Return(func(ctx context.Context, fn func(pgx.Tx) error) error {
		return fn(nil)
	})
	mockTransRepo.On("LockTransactionForCancellation", ctx, int64(1), mock.Anything).Return(true, nil)
	mockUserRepo.On("GetUserForUpdate", ctx, int64(1), mock.Anything).Return(&model.User{
		ID:      1,
		Balance: decimal.NewFromInt(200),
		Version: 1,
	}, nil)
	mockUserRepo.On("UpdateBalance", ctx, int64(1), decimal.NewFromInt(100), mock.Anything).Return(nil)
	mockTransRepo.On("CancelTransactionIfProcessed", ctx, int64(1), mock.Anything).Return(true, nil)

	service := NewCancellationService(mockUserRepo, mockTransRepo, mockDBManager, logger)
	err := service.ProcessOddRecordCancellation(ctx)

	assert.NoError(t, err)
}

func TestCancellationService_ProcessOddRecordCancellation_NoTransactionsToCancel(t *testing.T) {
	ctx := context.Background()
	logger := zerolog.Nop()

	mockUserRepo := mocks.NewUserRepository(t)
	mockTransRepo := mocks.NewTransactionRepository(t)
	mockDBManager := mocks.NewDBManager(t)

	mockTransRepo.On("GetLatestOddProcessedTransactions", ctx, 10).Return([]*model.Transaction{}, nil)

	service := NewCancellationService(mockUserRepo, mockTransRepo, mockDBManager, logger)
	err := service.ProcessOddRecordCancellation(ctx)

	assert.NoError(t, err)

	mockUserRepo.AssertNotCalled(t, "GetUserForUpdate")
	mockUserRepo.AssertNotCalled(t, "UpdateBalance")
	mockTransRepo.AssertNotCalled(t, "CancelTransactionIfProcessed")
	mockDBManager.AssertNotCalled(t, "WithTransaction")
}
