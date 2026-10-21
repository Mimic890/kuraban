package middleware

import (
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"
)

// structValidator adapts the go-playground validator to fiber's
// StructValidator hook so that the `validate:"required"` tags on request
// models are enforced automatically whenever a handler binds with
// c.Bind().JSON() (and friends). Without it, fiber skips validation and the
// tags are inert, leaving required-field checks to manual handler code.
type structValidator struct {
	validate *validator.Validate
}

// Validate is the fiber.StructValidator entry point. Returning a non-nil
// error makes the bind chain report the failure, which handlers surface as a
// 400 Bad Request.
func (v *structValidator) Validate(out any) error {
	return v.validate.Struct(out)
}

// NewStructValidator builds a fiber.StructValidator backed by the
// go-playground validator. Register it on fiber.Config.StructValidator to
// turn on automatic request-model validation.
func NewStructValidator() fiber.StructValidator {
	return &structValidator{validate: validator.New()}
}