package utils

import (
	"errors"

	"github.com/google/uuid"
)

const (
	ErrInvalidUUID = "invalid UUID format"
	ErrEmptyUUID   = "UUID cannot be empty"
)

func ParseUUID(s string) (uuid.UUID, error) {
	if s == "" {
		return uuid.Nil, errors.New(ErrEmptyUUID)
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil, errors.New(ErrInvalidUUID)
	}
	return id, nil
}
