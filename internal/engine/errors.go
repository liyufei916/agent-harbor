package engine

import "github.com/liyufei916/agent-harbor/internal/contracts"

func errEngine(message string) *contracts.AppError {
	return &contracts.AppError{
		Code:    contracts.ErrIsolationUnavailable,
		Message: message,
	}
}
