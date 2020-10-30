package errors

import (
	"errors"
)

type Type error

var (
	ErrInvalidInput       = Type(errors.New("invalid input"))
	ErrNotAllowed         = Type(errors.New("not allowed"))
	ErrNotFound           = Type(errors.New("not found"))
	ErrTemporyUnavailable = Type(errors.New("temporary unavailable"))
	ErrInternalError      = Type(errors.New("internal error"))
)

type TypedError struct {
	type_ Type
	cause error
}

func NewTyped(typ Type, cause error) TypedError {
	return TypedError{
		type_: typ,
		cause: cause,
	}
}

func (e TypedError) Error() string {
	return e.cause.Error()
}

func (e TypedError) Unwrap() error {
	return error(e.type_)
}
