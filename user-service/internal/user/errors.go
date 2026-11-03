package user

import "errors"

// Domain errors returned by the user service. The REST handler maps them to an HTTP status.

// ErrInvalidInput marks errors caused by bad caller input (handler maps it to 400)
var ErrInvalidInput = errors.New("invalid input")
var ErrNotFound = errors.New("user not found")
var ErrEmailTaken = errors.New("email already taken") // another user has this email (REST 409)
