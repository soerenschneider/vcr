package config

import "github.com/go-playground/validator/v10"

var validate *validator.Validate = validator.New()

func Validate(s any) error {
	return validate.Struct(s)
}
