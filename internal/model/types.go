package model

type State string

const (
	StateWin  State = "win"
	StateLost State = "lost"
)

type SourceType string

const (
	SourceGame    SourceType = "game"
	SourceServer  SourceType = "server"
	SourcePayment SourceType = "payment"
)

type TransactionStatus string

const (
	StatusProcessed TransactionStatus = "processed"
	StatusCancelled TransactionStatus = "cancelled"
)

func ParseSourceType(s string) (SourceType, error) {
	switch s {
	case string(SourceGame):
		return SourceGame, nil
	case string(SourceServer):
		return SourceServer, nil
	case string(SourcePayment):
		return SourcePayment, nil
	default:
		return "", ErrInvalidSourceType
	}
}

func (s SourceType) String() string {
	return string(s)
}

func ParseState(s string) (State, error) {
	switch s {
	case string(StateWin):
		return StateWin, nil
	case string(StateLost):
		return StateLost, nil
	default:
		return "", ErrInvalidState
	}
}

func (s State) String() string {
	return string(s)
}
