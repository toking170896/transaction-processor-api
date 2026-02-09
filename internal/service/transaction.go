package service

import (
	"context"
	"errors"
	"fmt"
	"transaction-processor/internal/model"
	"transaction-processor/internal/repository"

	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog"
	"github.com/shopspring/decimal"
)

// rollback and check for duplicate outside tx
var errDuplicateInsertRace = errors.New("duplicate transaction insert race")

type TransactionServiceImpl struct {
	userRepo        repository.UserRepository
	transactionRepo repository.TransactionRepository
	dbManager       repository.DBManager
	logger          zerolog.Logger
}

func NewTransactionService(
	userRepo repository.UserRepository,
	transactionRepo repository.TransactionRepository,
	dbManager repository.DBManager,
	logger zerolog.Logger,
) TransactionService {
	return &TransactionServiceImpl{
		userRepo:        userRepo,
		transactionRepo: transactionRepo,
		dbManager:       dbManager,
		logger:          logger,
	}
}

func (s *TransactionServiceImpl) ProcessTransaction(ctx context.Context, req *model.TransactionRequest, sourceType model.SourceType, userID int64) (*model.TransactionResponse, error) {
	var result *model.TransactionResponse

	// Validate inputs early, before transaction and locks
	amount, err := decimal.NewFromString(req.Amount)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", model.ErrInvalidAmount, err.Error())
	}

	if amount.LessThanOrEqual(decimal.Zero) {
		return nil, fmt.Errorf("%w: amount must be positive", model.ErrInvalidAmount)
	}

	state, err := model.ParseState(req.State)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", model.ErrInvalidState, err)
	}

	// Service manages transaction to keep operations to multiple repos atomic
	err = s.dbManager.WithTransaction(ctx, func(tx pgx.Tx) error {
		// Get transaction if exists and validate user_id
		existingTrans, err := s.transactionRepo.GetTransaction(ctx, req.TransactionID, tx)
		if err != nil && !errors.Is(err, model.ErrTransactionNotFound) {
			return fmt.Errorf("get transaction: %w", err)
		}

		// Transaction exists
		if existingTrans != nil {
			if existingTrans.UserID != userID {
				// Same transaction_id but different user - return error
				return fmt.Errorf("%w: transaction %s already exists for user %d, requested for user %d",
					model.ErrDuplicateTransaction, req.TransactionID, existingTrans.UserID, userID)
			}

			// Same transaction_id and same user - return existing result
			balance, err := s.userRepo.GetBalance(ctx, userID, tx)
			if err != nil {
				return fmt.Errorf("get balance: %w", err)
			}

			s.logger.Info().Str("transaction_id", req.TransactionID).Int64("user_id", userID).Msg("transaction already processed")
			result = &model.TransactionResponse{
				Status:  "already_processed",
				Balance: balance.StringFixed(2),
				Message: "Transaction already processed",
			}
			return nil
		}

		// Get user with lock
		user, err := s.userRepo.GetUserForUpdate(ctx, userID, tx)
		if err != nil {
			return fmt.Errorf("get user for update: %w", err)
		}

		newBalance := user.Balance
		switch state {
		case model.StateWin:
			newBalance = newBalance.Add(amount)
		case model.StateLost:
			newBalance = newBalance.Sub(amount)
		}

		// Negative balance is not allowed
		if newBalance.LessThan(decimal.Zero) {
			return model.ErrInsufficientBalance
		}

		err = s.userRepo.UpdateBalance(ctx, userID, newBalance, tx)
		if err != nil {
			return fmt.Errorf("update balance: %w", err)
		}

		// Insert transaction
		transaction := &model.Transaction{
			TransactionID: req.TransactionID,
			UserID:        userID,
			SourceType:    sourceType,
			State:         state,
			Amount:        amount,
			Status:        model.StatusProcessed,
		}

		err = s.transactionRepo.InsertTransaction(ctx, transaction, tx)
		if err != nil {
			if errors.Is(err, model.ErrDuplicateTransaction) {
				// Another request inserted the same transaction_id, rollback tx
				return errDuplicateInsertRace
			}
			return fmt.Errorf("insert transaction: %w", err)
		}

		s.logger.Info().Str("transaction_id", req.TransactionID).Int64("user_id", userID).Str("state", state.String()).
			Str("amount", amount.String()).
			Str("new_balance", newBalance.StringFixed(2)).
			Msg("transaction processed successfully")

		result = &model.TransactionResponse{
			Status:  "success",
			Balance: newBalance.StringFixed(2),
			Message: "Transaction processed successfully",
		}

		return nil
	})

	// Handle duplicate transaction, check if created for same user or not
	if errors.Is(err, errDuplicateInsertRace) {
		existing, getErr := s.transactionRepo.GetTransaction(ctx, req.TransactionID)
		if getErr != nil {
			return nil, fmt.Errorf("get transaction after duplicate: %w", getErr)
		}

		if existing.UserID != userID {
			return nil, fmt.Errorf("%w: transaction %s already exists for user %d, requested for user %d",
				model.ErrDuplicateTransaction, req.TransactionID, existing.UserID, userID)
		}

		balance, balErr := s.userRepo.GetBalance(ctx, userID)
		if balErr != nil {
			return nil, fmt.Errorf("get balance after duplicate: %w", balErr)
		}

		s.logger.Info().
			Str("transaction_id", req.TransactionID).
			Int64("user_id", userID).
			Msg("transaction already processed (detected after rollback)")

		return &model.TransactionResponse{
			Status:  "already_processed",
			Balance: balance.StringFixed(2),
			Message: "Transaction already processed",
		}, nil
	}

	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *TransactionServiceImpl) GetBalance(ctx context.Context, userID int64) (*model.BalanceResponse, error) {
	balance, err := s.userRepo.GetBalance(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get balance: %w", err)
	}

	return &model.BalanceResponse{
		UserID:  userID,
		Balance: balance.StringFixed(2),
	}, nil
}

func (s *TransactionServiceImpl) GetTransactionsByUser(ctx context.Context, userID int64, limit, offset int) ([]*model.Transaction, error) {
	transactions, err := s.transactionRepo.GetTransactionsByUser(ctx, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("get user transactions: %w", err)
	}

	return transactions, nil
}
