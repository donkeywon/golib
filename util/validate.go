package util

import "github.com/go-playground/validator/v10"

var V = validator.New(validator.WithRequiredStructEnabled())
