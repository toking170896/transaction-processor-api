package service

import (
	"context"
	"fmt"
	"transaction-processor/internal/model"
	"transaction-processor/internal/repository"

	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog"
	"github.com/shopspring/decimal"
)

type CancellationServiceImpl struct {
	userRepo        repository.UserRepository
	transactionRepo repository.TransactionRepository
	dbManager       repository.DBManager
	logger          zerolog.Logger
}

func NewCancellationService(
	userRepo repository.UserRepository,
	transactionRepo repository.TransactionRepository,
	dbManager repository.DBManager,
	logger zerolog.Logger,
) CancellationService {
	return &CancellationServiceImpl{
		userRepo:        userRepo,
		transactionRepo: transactionRepo,
		dbManager:       dbManager,
		logger:          logger,
	}
}

// ProcessOddRecordCancellation cancels odd-numbered processed transactions and adjusts user balance
func (s *CancellationServiceImpl) ProcessOddRecordCancellation(ctx context.Context) error {
	var cancelledCount int

	// Fetch up to 10 latest odd transactions with 'processed' state
	transactions, err := s.transactionRepo.GetLatestOddProcessedTransactions(ctx, 10)
	if err != nil {
		return fmt.Errorf("get odd transactions: %w", err)
	}

	if len(transactions) == 0 {
		s.logger.Debug().Msg("no odd transactions with 'processed' state to cancel")
		return nil
	}

	// Process each transaction in its own transaction
	for _, trans := range transactions {
		// Stop quickly on shutdown
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		var cancelled bool
		err = s.dbManager.WithTransaction(ctx, func(tx pgx.Tx) error {
			// Lock transaction row to avoid duplicate work under concurrency
			locked, err := s.transactionRepo.LockTransactionForCancellation(ctx, trans.ID, tx)
			if err != nil {
				return fmt.Errorf("lock transaction for cancellation: %w", err)
			}
			if !locked {
				s.logger.Debug().Str("transaction_id", trans.TransactionID).Msg("transaction already claimed or cancelled")
				return nil
			}

			// Get user with lock
			user, err := s.userRepo.GetUserForUpdate(ctx, trans.UserID, tx)
			if err != nil {
				return fmt.Errorf("get user for update: %w", err)
			}

			// Reverse the transaction (+/-)
			// "win" originally adds to user balance, so cancellation subtracts it back
			newBalance := user.Balance
			switch trans.State {
			case model.StateWin:
				// Reverse win = subtract
				newBalance = newBalance.Sub(trans.Amount)
			case model.StateLost:
				// Reverse lost = add
				newBalance = newBalance.Add(trans.Amount)
			}

			// Check balance constraint
			if newBalance.LessThan(decimal.Zero) {
				s.logger.Warn().
					Str("transaction_id", trans.TransactionID).
					Int64("user_id", trans.UserID).
					Str("current_balance", user.Balance.StringFixed(2)).
					Str("would_be_balance", newBalance.StringFixed(2)).
					Msg("cannot cancel transaction: negative balance not allowed")
				return nil
			}

			err = s.userRepo.UpdateBalance(ctx, trans.UserID, newBalance, tx)
			if err != nil {
				return fmt.Errorf("update balance: %w", err)
			}

			// Update transaction status, if current status is 'processed'
			updated, err := s.transactionRepo.CancelTransactionIfProcessed(ctx, trans.ID, tx)
			if err != nil {
				return fmt.Errorf("update transaction status: %w", err)
			}

			if !updated {
				s.logger.Warn().Str("transaction_id", trans.TransactionID).Msg("transaction status not updated - may have been already cancelled")
				return nil
			}

			s.logger.Info().
				Str("transaction_id", trans.TransactionID).
				Int64("user_id", trans.UserID).
				Str("original_state", trans.State.String()).
				Str("amount", trans.Amount.StringFixed(2)).
				Str("old_balance", user.Balance.StringFixed(2)).
				Str("new_balance", newBalance.StringFixed(2)).
				Msg("transaction cancelled and balance adjusted")
			cancelled = true
			return nil
		})

		if err != nil {
			s.logger.Error().
				Err(err).
				Str("transaction_id", trans.TransactionID).
				Int64("user_id", trans.UserID).
				Msg("failed to cancel transaction")
		}
		if cancelled {
			cancelledCount++
		}
	}

	s.logger.Info().
		Int("requested", len(transactions)).
		Int("cancelled", cancelledCount).
		Msg("odd transactions cancellation completed")

	return nil
}
