package errdefs

import "errors"

var (
	ErrAlreadyExists    = errors.New("user already exists")
	ErrValidation       = errors.New("validation error")
	ErrAuthentication   = errors.New("authentication error")
	ErrNotFound         = errors.New("user not found")
	ErrPermissionDenied = errors.New("permission denied")
)
