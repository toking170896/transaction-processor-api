package model

import "errors"

var (
	ErrInsufficientBalance  = errors.New("insufficient balance")
	ErrDuplicateTransaction = errors.New("duplicate transaction")
	ErrInvalidState         = errors.New("invalid state")
	ErrInvalidAmount        = errors.New("invalid amount")
	ErrInvalidSourceType    = errors.New("invalid source type")
	ErrUserNotFound         = errors.New("user not found")
	ErrTransactionNotFound  = errors.New("transaction not found")
)
