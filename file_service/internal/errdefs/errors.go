package errdefs

import "errors"

var (
	ErrAlreadyExists    = errors.New("already exists")
	ErrValidation       = errors.New("validation error")
	ErrAuthentication   = errors.New("authentication error")
	ErrNotFound         = errors.New("not found")
	ErrPermissionDenied = errors.New("permission denied")
)
