package service

import "errors"

var (
	ErrProfileNotFound      = errors.New("profile not found")
	ErrProfileAlreadyExists = errors.New("profile already exists")
	ErrInvalidUsername      = errors.New("invalid username")
	ErrDatabaseError        = errors.New("database error")
)
